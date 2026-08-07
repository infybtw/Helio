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

// RestrictChatMemberParams contains parameters for the restrictChatMember method.
type RestrictChatMemberParams struct {
	ChatID      int64            `json:"chat_id"`
	UserID      int64            `json:"user_id"`
	Permissions ChatPermissions  `json:"permissions"`
	UntilDate   int64            `json:"until_date,omitempty"`
}

// ChatPermissions describes actions that a non-administrator user is allowed to take in a chat.
type ChatPermissions struct {
	CanSendMessages       bool `json:"can_send_messages"`
	CanSendAudios         bool `json:"can_send_audios"`
	CanSendDocuments      bool `json:"can_send_documents"`
	CanSendPhotos         bool `json:"can_send_photos"`
	CanSendVideos         bool `json:"can_send_videos"`
	CanSendVideoNotes     bool `json:"can_send_video_notes"`
	CanSendVoiceNotes     bool `json:"can_send_voice_notes"`
	CanSendPolls          bool `json:"can_send_polls"`
	CanSendOtherMessages  bool `json:"can_send_other_messages"`
	CanAddWebPagePreviews bool `json:"can_add_web_page_previews"`
}

// MutePermissions denies all sending permissions (a full mute).
var MutePermissions = ChatPermissions{}

// RestrictChatMember restricts a user in a chat. untilDate is a unix timestamp;
// use 0 to restrict forever.
func (c *Client) RestrictChatMember(ctx context.Context, chatID, userID int64, permissions ChatPermissions, untilDate int64) error {
	return c.call(ctx, "restrictChatMember", RestrictChatMemberParams{
		ChatID:      chatID,
		UserID:      userID,
		Permissions: permissions,
		UntilDate:   untilDate,
	}, nil)
}

// BanChatMemberParams contains parameters for the banChatMember method.
type BanChatMemberParams struct {
	ChatID         int64 `json:"chat_id"`
	UserID         int64 `json:"user_id"`
	UntilDate      int64 `json:"until_date,omitempty"`
	RevokeMessages bool  `json:"revoke_messages,omitempty"`
}

// BanChatMember bans a user in a chat. untilDate is a unix timestamp; use 0 to
// ban forever. If revokeMessages is true, all messages from the user are deleted.
func (c *Client) BanChatMember(ctx context.Context, chatID, userID int64, untilDate int64, revokeMessages bool) error {
	return c.call(ctx, "banChatMember", BanChatMemberParams{
		ChatID:         chatID,
		UserID:         userID,
		UntilDate:      untilDate,
		RevokeMessages: revokeMessages,
	}, nil)
}

// GetChatMemberParams contains parameters for the getChatMember method.
type GetChatMemberParams struct {
	ChatID int64 `json:"chat_id"`
	UserID int64 `json:"user_id"`
}

// GetChatMember returns information about a member of a chat.
func (c *Client) GetChatMember(ctx context.Context, chatID, userID int64) (*ChatMember, error) {
	var member ChatMember
	if err := c.call(ctx, "getChatMember", GetChatMemberParams{ChatID: chatID, UserID: userID}, &member); err != nil {
		return nil, err
	}
	return &member, nil
}

// GetMe returns information about the bot itself.
func (c *Client) GetMe(ctx context.Context) (*User, error) {
	var user User
	if err := c.call(ctx, "getMe", nil, &user); err != nil {
		return nil, err
	}
	return &user, nil
}
