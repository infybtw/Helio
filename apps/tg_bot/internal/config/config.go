package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds runtime configuration loaded from environment variables.
type Config struct {
	BotToken         string
	WebhookURL       string
	WebhookSecret    string
	WebhookPath      string
	Port             string
	DatabaseURL      string
	AllowedUpdates   []string
	SessionSecret    string
	DashboardOrigin  string
	DashboardURL     string
	SessionMaxAge    int
	CookieSecure     bool
	CookieSameSite   string
	CookieDomain     string
	OIDCClientID     string
	OIDCClientSecret string
	OIDCRedirectURI  string
	OIDCScopes       []string
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
		allowed = []string{"message", "channel_post", "edited_message", "edited_channel_post", "my_chat_member"}
	} else if !contains(allowed, "my_chat_member") {
		allowed = append(allowed, "my_chat_member")
	}

	sessionMaxAge := 7 * 24 * 60 * 60 // 7 days in seconds
	if v := os.Getenv("SESSION_MAX_AGE"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			sessionMaxAge = parsed
		}
	}

	scopes := strings.Split(os.Getenv("OIDC_SCOPES"), " ")
	if len(scopes) == 1 && scopes[0] == "" {
		scopes = []string{"openid", "profile"}
	}

	cfg := &Config{
		BotToken:         os.Getenv("TELEGRAM_BOT_TOKEN"),
		WebhookURL:       os.Getenv("WEBHOOK_URL"),
		WebhookSecret:    os.Getenv("WEBHOOK_SECRET"),
		WebhookPath:      webhookPath,
		Port:             port,
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		AllowedUpdates:   allowed,
		SessionSecret:    os.Getenv("SESSION_SECRET"),
		DashboardOrigin:  os.Getenv("DASHBOARD_ORIGIN"),
		DashboardURL:     os.Getenv("DASHBOARD_URL"),
		SessionMaxAge:    sessionMaxAge,
		CookieSecure:     strings.EqualFold(os.Getenv("COOKIE_SECURE"), "true"),
		CookieSameSite:   os.Getenv("COOKIE_SAME_SITE"),
		CookieDomain:     os.Getenv("COOKIE_DOMAIN"),
		OIDCClientID:     os.Getenv("OIDC_CLIENT_ID"),
		OIDCClientSecret: os.Getenv("OIDC_CLIENT_SECRET"),
		OIDCRedirectURI:  os.Getenv("OIDC_REDIRECT_URI"),
		OIDCScopes:       scopes,
	}

	if cfg.BotToken == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}
	if cfg.WebhookURL == "" {
		return nil, fmt.Errorf("WEBHOOK_URL is required")
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.WebhookSecret == "" {
		return nil, fmt.Errorf("WEBHOOK_SECRET is required")
	}
	if cfg.SessionSecret == "" {
		return nil, fmt.Errorf("SESSION_SECRET is required")
	}
	if cfg.DashboardOrigin == "" {
		return nil, fmt.Errorf("DASHBOARD_ORIGIN is required")
	}
	if cfg.OIDCClientID == "" {
		return nil, fmt.Errorf("OIDC_CLIENT_ID is required")
	}
	if cfg.OIDCClientSecret == "" {
		return nil, fmt.Errorf("OIDC_CLIENT_SECRET is required")
	}
	if cfg.OIDCRedirectURI == "" {
		return nil, fmt.Errorf("OIDC_REDIRECT_URI is required")
	}

	return cfg, nil
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == wanted {
			return true
		}
	}
	return false
}
