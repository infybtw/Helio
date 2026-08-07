package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds runtime configuration loaded from environment variables.
type Config struct {
	BotToken       string
	WebhookURL     string
	WebhookSecret  string
	WebhookPath    string
	Port           string
	AllowedUpdates []string
}

// Load reads configuration from the environment and validates required fields.
func Load() (*Config, error) {
	// Load .env from the current working directory; ignore error if missing.
	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	webhookPath := os.Getenv("WEBHOOK_PATH")
	if webhookPath == "" {
		webhookPath = "/webhook"
	}

	allowed := strings.Split(os.Getenv("ALLOWED_UPDATES"), ",")
	if len(allowed) == 1 && allowed[0] == "" {
		allowed = []string{"message", "channel_post", "edited_message", "edited_channel_post"}
	}

	cfg := &Config{
		BotToken:       os.Getenv("TELEGRAM_BOT_TOKEN"),
		WebhookURL:     os.Getenv("WEBHOOK_URL"),
		WebhookSecret:  os.Getenv("WEBHOOK_SECRET"),
		WebhookPath:    webhookPath,
		Port:           port,
		AllowedUpdates: allowed,
	}

	if cfg.BotToken == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}
	if cfg.WebhookURL == "" {
		return nil, fmt.Errorf("WEBHOOK_URL is required")
	}

	return cfg, nil
}
