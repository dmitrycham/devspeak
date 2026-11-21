// Command bot starts the DevSpeak Telegram bot (long polling).
package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/dmitrycham/devspeak/go-api/internal/ai"
	"github.com/dmitrycham/devspeak/go-api/internal/bot"
	"github.com/dmitrycham/devspeak/go-api/internal/cards"
	"github.com/dmitrycham/devspeak/go-api/internal/config"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("bot stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	// The .env file is in the repo root, and we start from go-api/.
	if err := godotenv.Load("../.env"); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("load .env: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	botCfg, err := config.LoadBot()
	if err != nil {
		return fmt.Errorf("load bot config: %w", err)
	}

	// ctx is cancelled on Ctrl+C (SIGINT) or SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("create db pool: %w", err)
	}
	defer pool.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}

	// Same layers as the API: repository → service. The bot replaces the HTTP handler.
	// The bot has no enrich command yet, but a real client is safer than nil.
	aiClient := ai.NewClient(cfg.AIServiceURL, cfg.AITimeout)
	svc := cards.NewService(cards.NewPostgresRepository(pool), aiClient)
	handler := bot.NewHandler(svc, botCfg.AllowedUserID, logger)

	return handler.Run(ctx, botCfg.Token)
}
