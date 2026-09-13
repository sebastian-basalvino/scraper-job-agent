.PHONY: help build build-linux build-linux-amd64 test \
        provision-ec2 wait-ssh bootstrap bootstrap-run destroy-ec2 \
        deploy deploy-binary deploy-env deploy-init deploy-run deploy-logs deploy-cron \
        check-deploy check-provision check-aws

GO            ?= go
GOOS_LINUX    ?= linux
GOARCH_LINUX  ?= arm64
BINARY_LOCAL  ?= bin/scraper
BINARY_LINUX  ?= bin/scraper-linux
REMOTE_DIR    ?= /opt/scraper
ENV_FILE      ?= .env
CRON_SCHEDULE ?= 0 * * * *

# AWS / EC2 provisioning — override on the command line or in deploy.env
AWS_REGION          ?= us-east-1
AWS_KEY_NAME        ?=
EC2_INSTANCE_NAME   ?= freelance-job-agent
EC2_INSTANCE_TYPE   ?= t4g.micro
EC2_SSH_USER        ?= ec2-user
EC2_SG_NAME         ?= $(EC2_INSTANCE_NAME)-sg
MY_IP               ?=
DEPLOY_STATE_FILE   ?= deploy.state

# Deploy connection — override on the command line or in deploy.env / deploy.state
EC2_HOST      ?=
SSH_KEY       ?=
SSH_OPTS      ?= -o StrictHostKeyChecking=accept-new -o ConnectTimeout=5

-include deploy.env
-include deploy.state

ifeq ($(SSH_KEY),)
SSH := ssh $(SSH_OPTS)
SCP := scp $(SSH_OPTS)
else
SSH := ssh $(SSH_OPTS) -i $(SSH_KEY)
SCP := scp $(SSH_OPTS) -i $(SSH_KEY)
endif

help: ## Show available targets
	@echo "Usage: make <target> [vars from deploy.env]"
	@echo ""
	@echo "Build:"
	@echo "  build              Build native binary ($(BINARY_LOCAL))"
	@echo "  build-linux        Cross-compile for Linux ($(GOARCH_LINUX)) -> $(BINARY_LINUX)"
	@echo "  build-linux-amd64  Cross-compile for Linux amd64 (t3.micro)"
	@echo "  test               Run tests"
	@echo ""
	@echo "Infrastructure (requires AWS CLI + deploy.env):"
	@echo "  provision-ec2      Create or reuse EC2 instance, write deploy.state"
	@echo "  wait-ssh           Wait until SSH is reachable"
	@echo "  bootstrap          Full first-time setup: provision + deploy + cron"
	@echo "  bootstrap-run      bootstrap + run scraper once"
	@echo "  destroy-ec2        Terminate the provisioned instance"
	@echo ""
	@echo "Deploy (requires EC2_HOST and SSH_KEY):"
	@echo "  deploy             Build + init dirs + upload binary + upload .env"
	@echo "  deploy-binary      Upload binary only (no build)"
	@echo "  deploy-env         Upload .env only"
	@echo "  deploy-init        Create remote directories"
	@echo "  deploy-run         Run scraper once on the instance"
	@echo "  deploy-logs        Tail remote logs"
	@echo "  deploy-cron        Install hourly cron job on the instance"
	@echo ""
	@echo "Quick start:"
	@echo "  cp deploy.env.example deploy.env   # set AWS_KEY_NAME + SSH_KEY"
	@echo "  make bootstrap-run                 # provision EC2 + deploy + cron + test"

# --- Build ---

build: ## Build native binary
	$(GO) build -o $(BINARY_LOCAL) ./cmd/scraper

build-linux: ## Cross-compile for Linux (default: arm64 / t4g.micro)
	GOOS=$(GOOS_LINUX) GOARCH=$(GOARCH_LINUX) CGO_ENABLED=0 $(GO) build -o $(BINARY_LINUX) ./cmd/scraper

build-linux-amd64: ## Cross-compile for Linux amd64 (t3.micro)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) build -o $(BINARY_LINUX) ./cmd/scraper

test: ## Run tests
	$(GO) test ./...

# --- AWS / EC2 ---

check-aws:
	@command -v aws >/dev/null 2>&1 || (echo "error: aws CLI not found — install and run 'aws configure'" && exit 1)
	@aws sts get-caller-identity >/dev/null || (echo "error: AWS credentials not configured — run 'aws configure'" && exit 1)

check-provision: check-aws
ifndef AWS_KEY_NAME
	$(error AWS_KEY_NAME is required. Set it in deploy.env — the key pair name in AWS EC2)
endif
ifndef SSH_KEY
	$(error SSH_KEY is required. Set it in deploy.env — path to your .pem file)
endif

# Read deploy.state at recipe time (after provision-ec2 writes it)
read_state = $(shell grep '^$(1)=' $(DEPLOY_STATE_FILE) 2>/dev/null | cut -d= -f2-)

provision-ec2: check-provision ## Create or reuse EC2 instance
	@chmod +x scripts/provision-ec2.sh
	@AWS_REGION=$(AWS_REGION) \
	 AWS_KEY_NAME=$(AWS_KEY_NAME) \
	 SSH_KEY=$(SSH_KEY) \
	 EC2_INSTANCE_NAME=$(EC2_INSTANCE_NAME) \
	 EC2_INSTANCE_TYPE=$(EC2_INSTANCE_TYPE) \
	 EC2_SSH_USER=$(EC2_SSH_USER) \
	 EC2_SG_NAME=$(EC2_SG_NAME) \
	 MY_IP=$(MY_IP) \
	 DEPLOY_STATE_FILE=$(DEPLOY_STATE_FILE) \
	 ./scripts/provision-ec2.sh
	@$(MAKE) wait-ssh \
		EC2_HOST="$(call read_state,EC2_HOST)" \
		SSH_KEY="$(SSH_KEY)" \
		SSH_OPTS="$(SSH_OPTS)"

wait-ssh: check-deploy ## Wait until SSH is reachable
	@echo "Waiting for SSH on $(EC2_HOST)..."
	@for i in $$(seq 1 30); do \
		$(SSH) $(EC2_HOST) "echo connected" >/dev/null 2>&1 && \
		{ echo "SSH ready"; exit 0; }; \
		echo "  attempt $$i/30..."; \
		sleep 10; \
	done; \
	echo "error: SSH timeout on $(EC2_HOST)"; exit 1

destroy-ec2: check-aws ## Terminate the provisioned EC2 instance
ifndef EC2_INSTANCE_ID
	$(error EC2_INSTANCE_ID not set — run provision-ec2 first or set it in deploy.state)
endif
	@echo "Terminating instance $(EC2_INSTANCE_ID)..."
	@aws ec2 terminate-instances --instance-ids $(EC2_INSTANCE_ID) --region $(AWS_REGION)
	@rm -f $(DEPLOY_STATE_FILE)
	@echo "Instance terminated. Removed $(DEPLOY_STATE_FILE)."

bootstrap: provision-ec2 ## Full setup: EC2 + deploy + cron
	@$(MAKE) deploy deploy-cron \
		EC2_HOST="$(call read_state,EC2_HOST)" \
		GOARCH_LINUX="$(call read_state,GOARCH_LINUX)" \
		SSH_KEY="$(SSH_KEY)" \
		SSH_OPTS="$(SSH_OPTS)"
	@echo ""
	@echo "Bootstrap complete."
	@echo "  Instance: $(call read_state,EC2_HOST)"
	@echo "  Cron:     $(CRON_SCHEDULE)"
	@echo "  Logs:     make deploy-logs"

bootstrap-run: bootstrap ## Full setup + test run
	@$(MAKE) deploy-run \
		EC2_HOST="$(call read_state,EC2_HOST)" \
		SSH_KEY="$(SSH_KEY)" \
		SSH_OPTS="$(SSH_OPTS)"

# --- Deploy checks ---

check-deploy:
ifndef EC2_HOST
	$(error EC2_HOST is required. Run 'make provision-ec2' or set EC2_HOST in deploy.env)
endif
ifndef SSH_KEY
	$(error SSH_KEY is required. Set SSH_KEY in deploy.env)
endif

# --- Deploy ---

deploy-init: check-deploy ## Create remote directory layout
	$(SSH) $(EC2_HOST) "mkdir -p $(REMOTE_DIR)/{bin,data,logs}"

deploy-binary: check-deploy ## Upload binary to the instance
	@test -f $(BINARY_LINUX) || (echo "error: $(BINARY_LINUX) not found — run 'make build-linux' first" && exit 1)
	$(SCP) $(BINARY_LINUX) $(EC2_HOST):$(REMOTE_DIR)/bin/scraper
	$(SSH) $(EC2_HOST) "chmod +x $(REMOTE_DIR)/bin/scraper"

deploy-env: check-deploy ## Upload .env to the instance
	@test -f $(ENV_FILE) || (echo "error: $(ENV_FILE) not found" && exit 1)
	$(SCP) $(ENV_FILE) $(EC2_HOST):$(REMOTE_DIR)/.env
	$(SSH) $(EC2_HOST) "chmod 600 $(REMOTE_DIR)/.env"

deploy: check-deploy build-linux deploy-init deploy-binary deploy-env ## Full deploy: build + upload binary and .env
	@echo "Deploy complete -> $(EC2_HOST):$(REMOTE_DIR)"

deploy-run: check-deploy ## Run scraper once on the instance (manual test)
	$(SSH) $(EC2_HOST) "cd $(REMOTE_DIR) && ./bin/scraper"

deploy-logs: check-deploy ## Tail remote scraper logs
	$(SSH) $(EC2_HOST) "tail -f $(REMOTE_DIR)/logs/scraper.log"

deploy-cron: check-deploy ## Install hourly cron job on the instance
	$(SSH) $(EC2_HOST) '\
		(crontab -l 2>/dev/null | grep -v "$(REMOTE_DIR)/bin/scraper"; \
		 echo "$(CRON_SCHEDULE) cd $(REMOTE_DIR) && $(REMOTE_DIR)/bin/scraper >> $(REMOTE_DIR)/logs/scraper.log 2>&1") \
		| crontab -'
	@echo "Cron installed:"
	@$(SSH) $(EC2_HOST) "crontab -l"
