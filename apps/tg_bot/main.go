package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"unicode/utf8"

	"tg_bot/internal/auth"
	"tg_bot/internal/config"
	"tg_bot/internal/database"
	"tg_bot/internal/database/postgres"
	"tg_bot/internal/handlers"
	"tg_bot/internal/httpserver"
	"tg_bot/internal/logger"
	"tg_bot/internal/telegram"
	"tg_bot/internal/voicequeue"
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
	voices, err := voicequeue.NewClient(cfg.NATSURL, logger)
	if err != nil {
		logger.Error("failed to connect to NATS", "error", err)
		os.Exit(1)
	}
	defer voices.Close()

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

	sessions := auth.NewSessionManager(cfg.SessionSecret, cfg.SessionMaxAge, cfg.CookieSecure, cfg.CookieSameSite, cfg.CookieDomain)
	// OIDC state cookie must be sent on the cross-site redirect from Telegram,
	// so it requires SameSite=None and Secure=true.
	stateMgr := auth.NewOIDCStateManager(cfg.SessionSecret, 600, true, "none", cfg.CookieDomain)
	oidcClient := auth.NewOIDCClient(cfg.OIDCClientID, cfg.OIDCClientSecret, cfg.OIDCRedirectURI, cfg.OIDCScopes)
	authHandler := handlers.NewAuth(oidcClient, stateMgr, sessions, db, client, cfg.DashboardOrigin, cfg.DashboardURL, logger)
	if _, err := voices.SubscribeResults(func(result voicequeue.Result) error {
		if err := db.RecordVoiceTranscription(ctx, database.VoiceTranscription{
			JobID: result.JobID, ChatID: result.ChatID, MessageID: result.MessageID, Transcript: result.Text,
			Language: result.Language, LanguageProbability: result.LanguageProbability,
			TranscriptionSeconds: result.TranscriptionSeconds, AudioDurationSeconds: result.AudioDurationSeconds,
		}); err != nil {
			logger.Error("failed to record voice transcription", "error", err, "job_id", result.JobID)
			return err
		}
		logger.Info(
			"recorded voice transcription",
			"job_id", result.JobID,
			"transcription_seconds", result.TranscriptionSeconds,
			"text_length", utf8.RuneCountInString(result.Text),
		)
		if result.Text == "" {
			return nil
		}
		reply := result.Text
		if utf8.RuneCountInString(reply) > 4096 {
			reply = "Расшифровка недоступна, слишком длинное сообщение"
		}
		if err := client.SendMessage(ctx, result.ChatID, reply, result.MessageID); err != nil {
			logger.Error("failed to send voice transcription", "error", err, "job_id", result.JobID)
			return err
		}
		logger.Info("sent voice transcription", "job_id", result.JobID, "language", result.Language)
		return nil
	}); err != nil {
		logger.Error("failed to subscribe to transcription results", "error", err)
		os.Exit(1)
	}
	webhook := handlers.NewWebhook(client, voices, db, cfg.WebhookSecret, logger)
	server := httpserver.New(
		":"+cfg.Port,
		cfg.WebhookPath,
		webhook.Handle,
		[]httpserver.RouteFunc{handlers.RegisterAuthRoutes(authHandler)},
		[]string{cfg.DashboardOrigin},
		logger,
	)

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
