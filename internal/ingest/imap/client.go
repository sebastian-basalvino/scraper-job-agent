package imap

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/mail"

	"scraper/internal/model"
)

// ClientConfig configures the IMAP client.
type ClientConfig struct {
	Host           string
	Port           int
	User           string
	Password       string
	Inbox          string
	Processed       string
	LinkedInSender  string
	WorkanaSender   string
	LinkedInEnabled bool
	WorkanaEnabled  bool
}

// Client fetches and processes emails from an IMAP mailbox.
type Client struct {
	cfg    ClientConfig
	logger *slog.Logger
}

// NewClient creates a new IMAP client.
func NewClient(cfg ClientConfig, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{cfg: cfg, logger: logger}
}

// FetchOffers fetches unprocessed emails from the inbox, parses them, and moves processed messages.
func (c *Client) FetchOffers(ctx context.Context) ([]model.Offer, error) {
	addr := fmt.Sprintf("%s:%d", c.cfg.Host, c.cfg.Port)
	client, err := imapclient.DialTLS(addr, nil)
	if err != nil {
		return nil, fmt.Errorf("dial imap: %w", err)
	}
	defer client.Close()

	if err := client.Login(c.cfg.User, c.cfg.Password).Wait(); err != nil {
		return nil, fmt.Errorf("imap login: %w", err)
	}

	if err := c.ensureProcessedFolder(client); err != nil {
		return nil, err
	}

	mbox, err := client.Select(c.cfg.Inbox, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("select inbox: %w", err)
	}

	if mbox.NumMessages == 0 {
		return nil, nil
	}

	seqSet := imap.SeqSetNum(1, mbox.NumMessages)
	fetchOptions := &imap.FetchOptions{
		Envelope: true,
		UID:      true,
		BodySection: []*imap.FetchItemBodySection{
			{},
		},
	}

	fetchCmd := client.Fetch(seqSet, fetchOptions)

	var offers []model.Offer
	var processedUIDs []imap.UID

	for {
		msg := fetchCmd.Next()
		if msg == nil {
			break
		}

		data, err := msg.Collect()
		if err != nil {
			c.logger.Error("collect message", "error", err)
			continue
		}

		if data.Envelope == nil || len(data.Envelope.From) == 0 {
			continue
		}

		sender := strings.ToLower(data.Envelope.From[0].Addr())
		body, err := readMessageBody(data)
		if err != nil {
			c.logger.Error("read message body", "uid", data.UID, "error", err)
			continue
		}

		receivedAt := data.InternalDate
		if receivedAt.IsZero() && data.Envelope != nil {
			receivedAt = data.Envelope.Date
		}

		var parsed []model.Offer
		switch {
		case c.cfg.LinkedInEnabled && strings.Contains(sender, strings.ToLower(c.cfg.LinkedInSender)):
			parsed, err = ParseLinkedIn(body, receivedAt)
			if err != nil {
				c.logger.Error("parse linkedin mail", "uid", data.UID, "error", err, "body", truncate(body, 2000))
				continue
			}
		case c.cfg.WorkanaEnabled && strings.Contains(sender, strings.ToLower(c.cfg.WorkanaSender)):
			parsed, err = ParseWorkana(body, receivedAt)
			if err != nil {
				c.logger.Error("parse workana mail", "uid", data.UID, "error", err)
				continue
			}
		default:
			continue
		}

		if len(parsed) > 0 {
			offers = append(offers, parsed...)
			processedUIDs = append(processedUIDs, data.UID)
		}
	}

	if err := fetchCmd.Close(); err != nil {
		return offers, fmt.Errorf("close fetch: %w", err)
	}

	if len(processedUIDs) > 0 {
		if err := c.moveToProcessed(client, processedUIDs); err != nil {
			return offers, fmt.Errorf("move processed mails: %w", err)
		}
	}

	return offers, nil
}

func (c *Client) ensureProcessedFolder(client *imapclient.Client) error {
	listCmd := client.List("", "*", nil)

	exists := false
	for {
		data := listCmd.Next()
		if data == nil {
			break
		}
		if strings.EqualFold(data.Mailbox, c.cfg.Processed) {
			exists = true
			break
		}
	}
	if err := listCmd.Close(); err != nil {
		return fmt.Errorf("list mailboxes: %w", err)
	}
	if exists {
		return nil
	}

	if err := client.Create(c.cfg.Processed, nil).Wait(); err != nil {
		return fmt.Errorf("create processed folder: %w", err)
	}
	return nil
}

func (c *Client) moveToProcessed(client *imapclient.Client, uids []imap.UID) error {
	var uidSet imap.UIDSet
	for _, uid := range uids {
		uidSet.AddNum(uid)
	}

	if _, err := client.Copy(uidSet, c.cfg.Processed).Wait(); err != nil {
		return fmt.Errorf("copy to processed: %w", err)
	}

	storeFlags := &imap.StoreFlags{
		Op:     imap.StoreFlagsAdd,
		Flags:  []imap.Flag{imap.FlagDeleted},
		Silent: true,
	}
	storeCmd := client.Store(uidSet, storeFlags, nil)
	if err := storeCmd.Close(); err != nil {
		return fmt.Errorf("mark deleted: %w", err)
	}

	if err := client.UIDExpunge(uidSet).Close(); err != nil {
		return fmt.Errorf("expunge: %w", err)
	}

	return nil
}

func readMessageBody(data *imapclient.FetchMessageBuffer) (string, error) {
	for _, section := range data.BodySection {
		if len(section.Bytes) == 0 {
			continue
		}

		entity, err := mail.CreateReader(strings.NewReader(string(section.Bytes)))
		if err != nil {
			return string(section.Bytes), nil
		}

		var parts []string
		for {
			part, err := entity.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return string(section.Bytes), nil
			}

			body, err := io.ReadAll(part.Body)
			if err != nil {
				continue
			}
			parts = append(parts, string(body))
		}

		if len(parts) == 0 {
			return string(section.Bytes), nil
		}
		return strings.Join(parts, "\n"), nil
	}
	return "", fmt.Errorf("no body section found")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
