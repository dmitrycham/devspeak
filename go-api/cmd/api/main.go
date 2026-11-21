// Command api starts the DevSpeak cards and reviews REST API.
package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/dmitrycham/devspeak/go-api/internal/ai"
	"github.com/dmitrycham/devspeak/go-api/internal/cards"
	"github.com/dmitrycham/devspeak/go-api/internal/config"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

// run holds all startup logic, so errors are returned instead of
// calling os.Exit in many places.
func run(logger *slog.Logger) error {
	// The .env file is in the repo root. In Docker it may not exist:
	// the variables come from docker compose instead.
	if err := godotenv.Load("../.env"); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("load .env: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// ctx is cancelled on Ctrl+C (SIGINT) or SIGTERM (docker stop).
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

	// Wire the layers: repository → service → handler.
	repo := cards.NewPostgresRepository(pool)
	aiClient := ai.NewClient(cfg.AIServiceURL, cfg.AITimeout)
	svc := cards.NewService(repo, aiClient)
	handler := cards.NewHandler(svc, logger)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		// Must be longer than the AI timeout: otherwise the server closes the
		// connection before POST /cards/{id}/enrich can send its answer.
		WriteTimeout: cfg.AITimeout + 10*time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start the server in a goroutine, so we can wait for a signal below.
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("api started", "addr", cfg.HTTPAddr)
		serverErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		logger.Info("shutting down")
	}

	// Graceful shutdown: stop accepting new requests and wait up to
	// 10 seconds for running requests to finish.
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}
