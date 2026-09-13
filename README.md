# Freelance Job Agent

Automated system that monitors freelance/remote job offers from configurable sources — LinkedIn (IMAP), Workana (IMAP), and Remotive (API) — filters them, scores them with Claude Haiku, and notifies relevant matches via Telegram.

By default only **LinkedIn** is enabled. Enable or disable each connector with `ENABLED_SOURCES` (see [Connectors](#connectors)).

## Architecture

```
┌─────────────┐   ┌─────────────┐   ┌─────────────┐
│  LinkedIn   │   │   Workana   │   │  Remotive   │
│  (IMAP)     │   │  (IMAP)     │   │  (API REST) │
└──────┬──────┘   └──────┬──────┘   └──────┬──────┘
       └────────┬────────┴──────────────────┘
                ▼
       ingest → normalization → dedup (SQLite) → rule filter
                → scoring (Claude) → notification (Telegram)
```

**Execution:** cron every hour on an EC2 instance (free tier).

## Requirements

### Local development

- Go 1.25+
- Email account with IMAP enabled (App Password recommended)
- Anthropic API key
- Telegram bot (BotFather) and `chat_id`

### Infrastructure (production)

- AWS account with free tier available
- [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html) installed and configured
- SSH key pair (.pem) to access the EC2 instance

### Optional (operating from Cursor)

- `uv` / `uvx` for the [AWS MCP Server](https://docs.aws.amazon.com/agent-toolkit/latest/userguide/getting-started-aws-mcp-server.html)

---

## Local setup

1. Copy `.env.example` to `.env` and fill in the values (see [Environment variables](#environment-variables)).
2. Install dependencies and build:

```bash
go mod tidy
make build
# or: go build -o bin/scraper ./cmd/scraper
```

3. Run a manual execution:

```bash
mkdir -p data logs
./bin/scraper
```

4. Run the tests:

```bash
make test
# or: go test ./...
```

---

## AWS infrastructure

### Summary

| Resource | Details |
|----------|---------|
| Compute | EC2 `t4g.micro` (ARM64, Graviton) — alternative: `t3.micro` (x86) |
| OS | Amazon Linux 2023 |
| Storage | 8–10 GB EBS (included in free tier) |
| App persistence | SQLite at `/opt/scraper/data/offers.db` |
| Scheduler | User cron (`ec2-user`) |
| Secrets | File `/opt/scraper/.env` (do not commit) |
| Estimated cost | ~$0/month on free tier + ~$0.50–$2/month Claude API |

### Network connectivity

The app only needs **outbound HTTPS (port 443)** to:

| Destination | Purpose |
|-------------|---------|
| IMAP server (e.g. `imap.gmail.com:993`) | Read LinkedIn/Workana alerts (if enabled) |
| `api.anthropic.com` | Scoring with Claude |
| `api.telegram.org` | Notifications |
| `remotive.com` | Job listings API (only if `remotive` is in `ENABLED_SOURCES`) |

The default security group (all outbound traffic allowed) is sufficient. For SSH, open port 22 from your IP only.

### Instance layout

```
/opt/scraper/
├── bin/scraper          # binary compiled locally
├── .env                 # environment variables (secrets)
├── data/
│   └── offers.db        # SQLite (deduplication)
└── logs/
    └── scraper.log      # cron output
```

---

## Deployment guide (manual, local build)

Deploy by cross-compiling the binary on your machine and uploading it via `make deploy`. The binary is not built on the instance.

### Quick start — full automated setup

One command provisions EC2 (if it doesn't exist), deploys the app, and installs cron:

```bash
cp deploy.env.example deploy.env
# edit deploy.env → AWS_KEY_NAME, SSH_KEY

# in .env set: SQLITE_PATH=/opt/scraper/data/offers.db

make bootstrap-run
```

What `bootstrap-run` does:

1. Creates or reuses an EC2 instance tagged `freelance-job-agent`
2. Creates a security group with SSH (22) from your IP
3. Waits for SSH to be ready
4. Cross-compiles, uploads binary + `.env`
5. Installs hourly cron
6. Runs the scraper once as a smoke test

Connection details are saved to `deploy.state` (gitignored).

### Quick start — deploy to existing instance

If you already have an EC2 instance:

```bash
cp deploy.env.example deploy.env
# set EC2_HOST and SSH_KEY in deploy.env

make deploy
make deploy-run
make deploy-cron
```

### Makefile targets

| Target | Description |
|--------|-------------|
| `make help` | List all available targets |
| `make build` | Build native binary (`bin/scraper`) |
| `make build-linux` | Cross-compile for Linux ARM64 (`t4g.micro`, default) |
| `make build-linux-amd64` | Cross-compile for Linux x86 (`t3.micro`) |
| `make test` | Run tests |
| `make provision-ec2` | Create or reuse EC2 instance, write `deploy.state` |
| `make wait-ssh` | Wait until SSH is reachable |
| `make bootstrap` | Full setup: EC2 + deploy + cron |
| `make bootstrap-run` | `bootstrap` + smoke test run |
| `make destroy-ec2` | Terminate provisioned instance |
| `make deploy` | Full deploy: build + init dirs + upload binary + `.env` |
| `make deploy-binary` | Upload binary only (no build) |
| `make deploy-env` | Upload `.env` only |
| `make deploy-init` | Create remote directories |
| `make deploy-run` | Run scraper once on the instance |
| `make deploy-logs` | Tail remote logs |
| `make deploy-cron` | Install hourly cron job |

Override vars on the command line:

```bash
make bootstrap-run AWS_KEY_NAME=my-key SSH_KEY=~/.ssh/my-key.pem
make deploy EC2_HOST=ec2-user@1.2.3.4 SSH_KEY=~/.ssh/key.pem
make build-linux-amd64 bootstrap   # for t3.micro (x86)
```

Future code updates (instance already running):

```bash
make deploy
```

### Step 0 — Configure AWS CLI

```bash
aws configure
# Access Key ID, Secret Access Key, region (e.g. us-east-1), output: json
```

Verify that credentials work:

```bash
aws sts get-caller-identity
```

### Step 1 — Create the EC2 instance

#### Option A: Automated (recommended)

```bash
make provision-ec2
# or full flow: make bootstrap-run
```

Requires `AWS_KEY_NAME` and `SSH_KEY` in `deploy.env`. The script:
- Reuses an existing instance with tag `freelance-job-agent` (starts it if stopped)
- Creates a new `t4g.micro` instance if none exists
- Writes `EC2_HOST` and `GOARCH_LINUX` to `deploy.state`

#### Option B: AWS Console

1. **EC2 → Launch instance**
2. Name: `freelance-job-agent`
3. AMI: **Amazon Linux 2023**
4. Instance type: `t4g.micro` (ARM64, recommended) — alternative: `t3.micro` (x86)
5. Key pair: create or select a pair and download the `.pem`
6. Security group: allow SSH (22) from your IP
7. Storage: 8–10 GB
8. Launch

#### Option C: AWS CLI

Adjust `KEY_NAME`, `SUBNET_ID`, and `YOUR_IP` before running.

```bash
# Create security group
aws ec2 create-security-group \
  --group-name scraper-sg \
  --description "Scraper EC2 - SSH from my IP only"

# Allow SSH from your IP only
aws ec2 authorize-security-group-ingress \
  --group-name scraper-sg \
  --protocol tcp \
  --port 22 \
  --cidr YOUR_IP/32

# Launch instance (t4g.micro, ARM64)
aws ec2 run-instances \
  --image-id resolve:ssm:/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-arm64 \
  --instance-type t4g.micro \
  --key-name KEY_NAME \
  --security-groups scraper-sg \
  --tag-specifications 'ResourceType=instance,Tags=[{Key=Name,Value=freelance-job-agent}]'
```

For x86 (`t3.micro`), change the AMI and instance type:

```bash
# AMI: resolve:ssm:/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-x86_64
# --instance-type t3.micro
```

Get the public IP:

```bash
aws ec2 describe-instances \
  --filters "Name=tag:Name,Values=freelance-job-agent" "Name=instance-state-name,Values=running" \
  --query "Reservations[].Instances[].PublicIpAddress" \
  --output text
```

### Step 2 — Prepare SSH access

```bash
chmod 400 ~/.ssh/your-key.pem
cp deploy.env.example deploy.env
# set EC2_HOST=ec2-user@<PUBLIC-IP> and SSH_KEY=~/.ssh/your-key.pem
```

> On Amazon Linux 2023 the default user is `ec2-user`.

### Step 3 — Deploy

```bash
make deploy        # build-linux + init + upload binary + .env
make deploy-run    # test manually
make deploy-cron   # install hourly cron
```

For `t3.micro` (x86) instead of `t4g.micro` (ARM):

```bash
make build-linux-amd64 deploy
```

> `CGO_ENABLED=0` is safe: the project uses `modernc.org/sqlite` (pure Go, no CGO dependency).

### Step 4 — Redeploy (future updates)

```bash
make deploy          # code + env changes
make deploy-binary   # binary only (skip build: run build-linux first)
make deploy-env      # env only
make deploy-logs     # tail logs
```

No restart is required: cron picks up the new binary on the next run.

---

## Connectors

Each ingestion source can be turned on or off independently via `ENABLED_SOURCES` (comma-separated list).

| Source | Value | Transport | Notes |
|--------|-------|-----------|-------|
| LinkedIn | `linkedin` | IMAP | Parses job alert emails |
| Workana | `workana` | IMAP | Parses project alert emails |
| Remotive | `remotive` | HTTPS API | Fetches remote job listings |

**Default:** `linkedin` (Workana and Remotive are disabled).

Examples:

```bash
# LinkedIn only (default)
ENABLED_SOURCES=linkedin

# LinkedIn + Remotive
ENABLED_SOURCES=linkedin,remotive

# All sources
ENABLED_SOURCES=linkedin,workana,remotive
```

When a source is disabled:

- Its ingestion step is skipped entirely (no IMAP parsing for Workana, no API call for Remotive).
- Sender-specific env vars (`WORKANA_SENDER`, etc.) are ignored until that source is re-enabled.

---

## Environment variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ENABLED_SOURCES` | No | `linkedin` | Comma-separated list of active connectors: `linkedin`, `workana`, `remotive` |
| `IMAP_HOST` | No | `imap.gmail.com` | IMAP server |
| `IMAP_PORT` | No | `993` | IMAP port |
| `IMAP_USER` | **Yes** | — | Email username |
| `IMAP_PASSWORD` | **Yes** | — | IMAP App Password |
| `IMAP_INBOX` | No | `INBOX` | Inbox folder |
| `IMAP_PROCESSED_FOLDER` | No | `Processed` | Folder to move processed emails |
| `LINKEDIN_SENDER` | No | `jobs-noreply@linkedin.com` | LinkedIn alert sender |
| `WORKANA_SENDER` | No | `noreply@workana.com` | Workana alert sender |
| `ANTHROPIC_API_KEY` | **Yes** | — | Anthropic API key |
| `CLAUDE_MODEL` | No | `claude-haiku-4-5` | Model for scoring |
| `TELEGRAM_BOT_TOKEN` | **Yes** | — | Telegram bot token |
| `TELEGRAM_CHAT_ID` | **Yes** | — | Destination chat ID |
| `SQLITE_PATH` | No | `./data/offers.db` | SQLite database path |
| `SCORE_THRESHOLD` | No | `65` | Minimum score to notify |

Copy `.env.example` as a starting point:

```bash
cp .env.example .env
```

---

## Operations and troubleshooting

### View logs on the instance

```bash
ssh -i $KEY $EC2_HOST "tail -f /opt/scraper/logs/scraper.log"
```

### Run manually (debug)

```bash
ssh -i $KEY $EC2_HOST "cd /opt/scraper && ./bin/scraper"
```

### Check instance status

```bash
aws ec2 describe-instance-status \
  --filters "Name=tag:Name,Values=freelance-job-agent" \
  --query "InstanceStatuses[].{Instance:InstanceId,State:InstanceState.Name,System:SystemStatus.Status,InstanceCheck:InstanceStatus.Status}"
```

### Common issues

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| `exec format error` when running the binary | Wrong architecture (arm64 vs amd64) | On `t4g.micro`, build with `GOARCH=arm64` or `make build-linux` |
| `missing required environment variables` | Incomplete or missing `.env` | Verify `/opt/scraper/.env` exists and has all required variables |
| IMAP timeout | Security group blocks outbound traffic or wrong credentials | Verify App Password and that outbound port 993 is allowed |
| No Telegram notifications | Wrong `TELEGRAM_CHAT_ID` | Send a message to the bot and look up the chat ID via the Bot API |
| Cron not running | Misconfigured crontab or permissions | Run `crontab -l` and verify paths are absolute |
| Corrupt SQLite database | Disk full or abrupt shutdown | Check space with `df -h`; if needed, delete `offers.db` (dedup history will be lost) |

### Stop / start the instance

To save costs outside free tier, you can stop the instance (cron will not run while stopped):

```bash
# Stop
aws ec2 stop-instances --instance-ids <INSTANCE_ID>

# Start
aws ec2 start-instances --instance-ids <INSTANCE_ID>
```

> On restart, the public IP may change if you do not have an Elastic IP assigned. Update `EC2_HOST` accordingly.

---

## AWS MCP in Cursor (optional)

To operate infrastructure from the Cursor chat (query instances, security groups, etc.), configure the AWS MCP Server in `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "aws-mcp": {
      "command": "uvx",
      "args": [
        "mcp-proxy-for-aws==1.6.4",
        "https://aws-mcp.us-east-1.api.aws/mcp",
        "--metadata", "AWS_REGION=us-east-1"
      ]
    }
  }
}
```

Requirements:

1. AWS CLI configured (`aws configure`)
2. `uv` installed (`curl -LsSf https://astral.sh/uv/install.sh | sh`)
3. Restart Cursor and verify in **Settings → MCP** that the server is active

Documentation: [Setting up the AWS MCP Server](https://docs.aws.amazon.com/agent-toolkit/latest/userguide/getting-started-aws-mcp-server.html)

---

## Security

- **Never** commit `.env` or `.pem` files (they are already in `.gitignore`).
- Restrict SSH (port 22) to your IP only in the security group.
- Use App Passwords for IMAP, not your primary email password.
- In production, consider migrating secrets to [AWS Secrets Manager](https://docs.aws.amazon.com/secretsmanager/) or [SSM Parameter Store](https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-parameter-store.html) and loading them in cron via a wrapper script.
- Rotate `ANTHROPIC_API_KEY` and `TELEGRAM_BOT_TOKEN` periodically.

---

## Estimated costs

| Component | Monthly cost |
|-----------|--------------|
| EC2 `t4g.micro` (free tier, ARM64) | $0 |
| EBS 8 GB (free tier) | $0 |
| Claude Haiku (~50 offers/day) | ~$0.50–$2 |
| Telegram Bot API | $0 |
| **Total** | **~$0.50–$2/month** |
