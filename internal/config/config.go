package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/sebastian-basalvino/scraper-job-agent/internal/model"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	IMAPHost     string
	IMAPPort     int
	IMAPUser     string
	IMAPPassword string
	IMAPInbox    string
	IMAPProcessed string

	LinkedInSender string
	WorkanaSender  string

	AnthropicAPIKey string
	ClaudeModel     string

	TelegramBotToken string
	TelegramChatID   string

	SQLitePath     string
	ScoreThreshold int

	EnabledSources map[model.Source]bool
}

// Load reads configuration from environment variables and validates required fields.
func Load() (Config, error) {
	cfg := Config{
		IMAPHost:       envOrDefault("IMAP_HOST", "imap.gmail.com"),
		IMAPPort:       envIntOrDefault("IMAP_PORT", 993),
		IMAPUser:       os.Getenv("IMAP_USER"),
		IMAPPassword:   os.Getenv("IMAP_PASSWORD"),
		IMAPInbox:      envOrDefault("IMAP_INBOX", "INBOX"),
		IMAPProcessed:  envOrDefault("IMAP_PROCESSED_FOLDER", "Processed"),
		LinkedInSender: envOrDefault("LINKEDIN_SENDER", "jobs-noreply@linkedin.com"),
		WorkanaSender:  envOrDefault("WORKANA_SENDER", "noreply@workana.com"),
		AnthropicAPIKey: os.Getenv("ANTHROPIC_API_KEY"),
		ClaudeModel:     envOrDefault("CLAUDE_MODEL", "claude-haiku-4-5"),
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:   os.Getenv("TELEGRAM_CHAT_ID"),
		SQLitePath:       envOrDefault("SQLITE_PATH", "./data/offers.db"),
		ScoreThreshold:   envIntOrDefault("SCORE_THRESHOLD", 65),
	}

	enabled, err := parseEnabledSources(envOrDefault("ENABLED_SOURCES", "linkedin"))
	if err != nil {
		return Config{}, err
	}
	cfg.EnabledSources = enabled

	var missing []string
	if cfg.IMAPUser == "" {
		missing = append(missing, "IMAP_USER")
	}
	if cfg.IMAPPassword == "" {
		missing = append(missing, "IMAP_PASSWORD")
	}
	if cfg.AnthropicAPIKey == "" {
		missing = append(missing, "ANTHROPIC_API_KEY")
	}
	if cfg.TelegramBotToken == "" {
		missing = append(missing, "TELEGRAM_BOT_TOKEN")
	}
	if cfg.TelegramChatID == "" {
		missing = append(missing, "TELEGRAM_CHAT_ID")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}

// SourceEnabled reports whether ingestion for the given source is active.
func (c Config) SourceEnabled(source model.Source) bool {
	return c.EnabledSources[source]
}

func parseEnabledSources(raw string) (map[model.Source]bool, error) {
	known := map[string]model.Source{
		"linkedin": model.SourceLinkedIn,
		"workana":  model.SourceWorkana,
		"remotive": model.SourceRemotive,
	}

	enabled := make(map[model.Source]bool)
	for part := range strings.SplitSeq(raw, ",") {
		name := strings.ToLower(strings.TrimSpace(part))
		if name == "" {
			continue
		}
		source, ok := known[name]
		if !ok {
			return nil, fmt.Errorf("unknown source in ENABLED_SOURCES: %s (valid: linkedin, workana, remotive)", name)
		}
		enabled[source] = true
	}

	if len(enabled) == 0 {
		return nil, fmt.Errorf("ENABLED_SOURCES must include at least one source")
	}

	return enabled, nil
}

func envOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
