package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const telegramAPIBase = "https://api.telegram.org/bot"

// Client is a thin HTTP client for the Telegram Bot API.
type Client struct {
	token   string
	baseURL string
	http    *http.Client
}

// NewClient creates a Telegram Bot API client for the provided token.
func NewClient(token string) *Client {
	return &Client{
		token:   token,
		baseURL: telegramAPIBase + token + "/",
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// call invokes a Telegram Bot API method and decodes the response into dst.
func (c *Client) call(ctx context.Context, method string, payload any, dst any) error {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+method, body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if !apiResp.OK {
		return fmt.Errorf("telegram API error %d: %s", apiResp.ErrorCode, apiResp.Description)
	}

	if dst != nil && len(apiResp.Result) > 0 {
		if err := json.Unmarshal(apiResp.Result, dst); err != nil {
			return fmt.Errorf("decode result: %w", err)
		}
	}

	return nil
}

// SetWebhookParams contains parameters for the setWebhook method.
type SetWebhookParams struct {
	URL                string   `json:"url"`
	SecretToken        string   `json:"secret_token,omitempty"`
	AllowedUpdates     []string `json:"allowed_updates,omitempty"`
	DropPendingUpdates bool     `json:"drop_pending_updates,omitempty"`
}

// SetWebhook configures the webhook endpoint with Telegram.
func (c *Client) SetWebhook(ctx context.Context, params SetWebhookParams) error {
	return c.call(ctx, "setWebhook", params, nil)
}

// DeleteWebhookParams contains parameters for the deleteWebhook method.
type DeleteWebhookParams struct {
	DropPendingUpdates bool `json:"drop_pending_updates,omitempty"`
}

// DeleteWebhook removes the currently configured webhook.
func (c *Client) DeleteWebhook(ctx context.Context, params DeleteWebhookParams) error {
	return c.call(ctx, "deleteWebhook", params, nil)
}

// DeleteMessageParams contains parameters for the deleteMessage method.
type DeleteMessageParams struct {
	ChatID    int64 `json:"chat_id"`
	MessageID int64 `json:"message_id"`
}

// DeleteMessage deletes a message in a chat or channel.
func (c *Client) DeleteMessage(ctx context.Context, chatID, messageID int64) error {
	return c.call(ctx, "deleteMessage", DeleteMessageParams{ChatID: chatID, MessageID: messageID}, nil)
}

// GetMe returns information about the bot itself.
func (c *Client) GetMe(ctx context.Context) (*User, error) {
	var user User
	if err := c.call(ctx, "getMe", nil, &user); err != nil {
		return nil, err
	}
	return &user, nil
}
