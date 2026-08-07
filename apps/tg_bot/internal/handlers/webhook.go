package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"tg_bot/internal/commands"
	"tg_bot/internal/database"
	"tg_bot/internal/telegram"
)

const secretTokenHeader = "X-Telegram-Bot-Api-Secret-Token"

// Webhook processes incoming Telegram webhook updates.
type Webhook struct {
	client *telegram.Client
	db     database.Store
	secret string
	log    *slog.Logger
}

// NewWebhook creates a new webhook handler.
func NewWebhook(client *telegram.Client, st database.Store, secret string, log *slog.Logger) *Webhook {
	return &Webhook{
		client: client,
		db:     st,
		secret: secret,
		log:    log,
	}
}

// Handle is the Gin handler for Telegram webhook requests.
func (w *Webhook) Handle(c *gin.Context) {
	if w.secret != "" && c.GetHeader(secretTokenHeader) != w.secret {
		w.log.Warn("webhook request with missing or invalid secret token")
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	var update telegram.Update
	if err := c.ShouldBindJSON(&update); err != nil {
		w.log.Warn("failed to decode update", "error", err)
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	w.log.Info("received update",
		"update_id", update.UpdateID,
		"has_message", update.Message != nil,
		"has_edited_message", update.EditedMessage != nil,
		"has_channel_post", update.ChannelPost != nil,
		"has_edited_channel_post", update.EditedChannelPost != nil,
	)
	w.dispatch(c.Request.Context(), &update)

	c.Status(http.StatusOK)
}

// dispatch routes the update to the appropriate command handler.
func (w *Webhook) dispatch(ctx context.Context, update *telegram.Update) {
	switch {
	case update.Message != nil:
		w.log.Info("dispatching to message handler",
			"update_id", update.UpdateID,
			"message_id", update.Message.MessageID,
			"chat_id", update.Message.Chat.ID,
			"chat_type", update.Message.Chat.Type,
			"text", update.Message.Text,
		)
		commands.Dispatch(ctx, w.client, w.db, update.Message, w.log)
	case update.EditedMessage != nil:
		w.log.Info("dispatching to edited message handler",
			"update_id", update.UpdateID,
			"message_id", update.EditedMessage.MessageID,
			"chat_id", update.EditedMessage.Chat.ID,
			"chat_type", update.EditedMessage.Chat.Type,
			"text", update.EditedMessage.Text,
		)
		commands.Dispatch(ctx, w.client, w.db, update.EditedMessage, w.log)
	}
}
