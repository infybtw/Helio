package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"tg_bot/internal/config"
	"tg_bot/internal/handlers"
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

	// Build the Gin server.
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	webhook := handlers.NewWebhook(client, cfg.WebhookSecret, logger)
	router.POST(cfg.WebhookPath, webhook.Handle)

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	go func() {
		logger.Info("starting http server", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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
