package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/joho/godotenv"

	"scraper/internal/config"
	"scraper/internal/pipeline"
	"scraper/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	st, err := store.Open(cfg.SQLitePath)
	if err != nil {
		logger.Error("open store", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	p := pipeline.New(cfg, st, logger)
	if err := p.Run(ctx); err != nil {
		logger.Error("pipeline run failed", "error", err)
		os.Exit(1)
	}

	logger.Info("pipeline run completed")
}
