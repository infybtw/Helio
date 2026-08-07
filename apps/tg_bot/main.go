package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tg_bot/internal/config"
	"tg_bot/internal/database/postgres"
	"tg_bot/internal/handlers"
	"tg_bot/internal/httpserver"
	"tg_bot/internal/logger"
	"tg_bot/internal/telegram"
)

func main() {
	// Configure log level from environment (default: info)
	logLevel := slog.LevelInfo
	switch os.Getenv("LOG_LEVEL") {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	}
	logger := logger.New(os.Stdout, logLevel)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	client := telegram.NewClient(cfg.BotToken)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	db, err := postgres.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	logger.Info("connected to database")

	// Validate token and optionally log bot identity.
	me, err := client.GetMe(ctx)
	if err != nil {
		logger.Error("failed to get bot info", "error", err)
		os.Exit(1)
	}
	logger.Info("bot authenticated", "id", me.ID, "username", me.Username)

	// Register the webhook with Telegram.
	if err := client.SetWebhook(ctx, telegram.SetWebhookParams{
		URL:                cfg.WebhookURL,
		SecretToken:        cfg.WebhookSecret,
		AllowedUpdates:     cfg.AllowedUpdates,
		DropPendingUpdates: true,
	}); err != nil {
		logger.Error("failed to set webhook", "error", err)
		os.Exit(1)
	}
	logger.Info("webhook registered", "url", cfg.WebhookURL)

	webhook := handlers.NewWebhook(client, db, cfg.WebhookSecret, logger)
	server := httpserver.New(":"+cfg.Port, cfg.WebhookPath, webhook.Handle, logger)

	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server error", "error", err)
			cancel()
		}
	}()

	<-ctx.Done()

	logger.Info("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("http shutdown error", "error", err)
	}

	if err := client.DeleteWebhook(context.Background(), telegram.DeleteWebhookParams{DropPendingUpdates: true}); err != nil {
		logger.Error("failed to delete webhook", "error", err)
	}

	logger.Info("shutdown complete")
}
