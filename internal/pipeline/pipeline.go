package pipeline

import (
	"context"
	"log/slog"

	"scraper/internal/config"
	"scraper/internal/filter"
	imapingest "scraper/internal/ingest/imap"
	"scraper/internal/ingest/remotive"
	"scraper/internal/model"
	"scraper/internal/notify"
	"scraper/internal/scoring"
	"scraper/internal/store"
)

// Pipeline orchestrates ingestion, filtering, scoring, and notification.
type Pipeline struct {
	cfg       config.Config
	store     *store.Store
	imap      *imapingest.Client
	remotive  *remotive.Client
	scorer    *scoring.Client
	notifier  *notify.Notifier
	logger    *slog.Logger
}

// New creates a new pipeline instance.
func New(cfg config.Config, st *store.Store, logger *slog.Logger) *Pipeline {
	if logger == nil {
		logger = slog.Default()
	}

	return &Pipeline{
		cfg: cfg,
		store: st,
		imap: imapingest.NewClient(imapingest.ClientConfig{
			Host:            cfg.IMAPHost,
			Port:            cfg.IMAPPort,
			User:            cfg.IMAPUser,
			Password:        cfg.IMAPPassword,
			Inbox:           cfg.IMAPInbox,
			Processed:       cfg.IMAPProcessed,
			LinkedInSender:  cfg.LinkedInSender,
			WorkanaSender:   cfg.WorkanaSender,
			LinkedInEnabled: cfg.SourceEnabled(model.SourceLinkedIn),
			WorkanaEnabled:  cfg.SourceEnabled(model.SourceWorkana),
		}, logger),
		remotive: remotive.NewClient(),
		scorer:   scoring.NewClient(cfg.AnthropicAPIKey, cfg.ClaudeModel),
		notifier: notify.NewNotifier(cfg.TelegramBotToken, cfg.TelegramChatID),
		logger:   logger,
	}
}

// Run executes a single pipeline pass.
func (p *Pipeline) Run(ctx context.Context) error {
	var allOffers []model.Offer

	if p.cfg.SourceEnabled(model.SourceLinkedIn) || p.cfg.SourceEnabled(model.SourceWorkana) {
		imapOffers, err := p.imap.FetchOffers(ctx)
		if err != nil {
			p.logger.Error("fetch imap offers", "error", err)
		} else {
			allOffers = append(allOffers, imapOffers...)
		}
	}

	if p.cfg.SourceEnabled(model.SourceRemotive) {
		remotiveOffers, err := p.remotive.FetchOffers(ctx)
		if err != nil {
			p.logger.Error("fetch remotive offers", "error", err)
		} else {
			allOffers = append(allOffers, remotiveOffers...)
		}
	}

	p.logger.Info("offers fetched", "count", len(allOffers))

	for _, offer := range allOffers {
		if err := p.processOffer(ctx, offer); err != nil {
			p.logger.Error("process offer", "unique_id", offer.UniqueID, "title", offer.Title, "error", err)
		}
	}

	return nil
}

func (p *Pipeline) processOffer(ctx context.Context, offer model.Offer) error {
	seen, err := p.store.IsSeen(offer.UniqueID)
	if err != nil {
		return err
	}
	if seen {
		p.logger.Debug("offer already processed", "unique_id", offer.UniqueID)
		return nil
	}

	if filter.ShouldDiscard(offer) {
		p.logger.Info("offer discarded by rules", "unique_id", offer.UniqueID, "title", offer.Title)
		return p.store.Save(offer, nil, false)
	}

	result, err := p.scorer.Score(ctx, offer)
	if err != nil {
		return err
	}

	notified := false
	if result.Score >= p.cfg.ScoreThreshold {
		if err := p.notifier.SendOffer(ctx, offer, result); err != nil {
			return err
		}
		notified = true
		p.logger.Info("offer notified", "unique_id", offer.UniqueID, "score", result.Score, "title", offer.Title)
	} else {
		p.logger.Info("offer below threshold", "unique_id", offer.UniqueID, "score", result.Score, "title", offer.Title)
	}

	score := result.Score
	return p.store.Save(offer, &score, notified)
}
