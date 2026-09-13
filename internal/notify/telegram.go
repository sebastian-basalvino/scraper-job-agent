package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"scraper/internal/model"
)

// Notifier sends Telegram notifications for high-scoring offers.
type Notifier struct {
	botToken   string
	chatID     string
	httpClient *http.Client
}

// NewNotifier creates a Telegram notifier.
func NewNotifier(botToken, chatID string) *Notifier {
	return &Notifier{
		botToken: botToken,
		chatID:   chatID,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

type sendMessageRequest struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

// SendOffer sends a formatted notification for a scored offer.
func (n *Notifier) SendOffer(ctx context.Context, offer model.Offer, result model.ScoreResult) error {
	message := fmt.Sprintf(
		"*%s*\n\n%s\n\nPuntuación: %d\nFuente: %s\nLink: %s",
		escapeMarkdown(offer.Title),
		escapeMarkdown(result.Summary),
		result.Score,
		offer.Source,
		offer.Link,
	)

	payload, err := json.Marshal(sendMessageRequest{
		ChatID:    n.chatID,
		Text:      message,
		ParseMode: "Markdown",
	})
	if err != nil {
		return fmt.Errorf("marshal telegram request: %w", err)
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.botToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("telegram request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read telegram response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func escapeMarkdown(text string) string {
	replacer := strings.NewReplacer("_", "\\_", "*", "\\*", "[", "\\[", "`", "\\`")
	return replacer.Replace(text)
}
