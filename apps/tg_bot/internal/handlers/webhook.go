package handlers

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"tg_bot/internal/commands"
	"tg_bot/internal/database"
	"tg_bot/internal/telegram"
	"tg_bot/internal/voicequeue"
)

const secretTokenHeader = "X-Telegram-Bot-Api-Secret-Token"

// Webhook processes incoming Telegram webhook updates.
type Webhook struct {
	client *telegram.Client
	voices *voicequeue.Client
	db     database.Store
	secret string
	log    *slog.Logger
}

// NewWebhook creates a new webhook handler.
func NewWebhook(client *telegram.Client, voices *voicequeue.Client, st database.Store, secret string, log *slog.Logger) *Webhook {
	return &Webhook{
		client: client,
		voices: voices,
		db:     st,
		secret: secret,
		log:    log,
	}
}

// Handle is the Gin handler for Telegram webhook requests.
func (w *Webhook) Handle(c *gin.Context) {
	if w.secret == "" || subtle.ConstantTimeCompare([]byte(c.GetHeader(secretTokenHeader)), []byte(w.secret)) != 1 {
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
		"has_my_chat_member", update.MyChatMember != nil,
	)
	c.Status(http.StatusOK)
	c.Writer.WriteHeaderNow()
	if update.Message != nil {
		go w.transcribeMedia(update.Message)
	}
	w.dispatch(c.Request.Context(), &update)
}

// dispatch routes the update to the appropriate command handler.
func (w *Webhook) dispatch(ctx context.Context, update *telegram.Update) {
	switch {
	case update.Message != nil:
		if update.Message.Chat == nil {
			return
		}
		if update.Message.Chat != nil {
			if err := w.db.TrackChat(ctx, update.Message.Chat.ID, update.Message.Chat.Type, update.Message.Chat.Title, update.Message.Chat.Username); err != nil {
				w.log.Error("failed to track chat", "error", err, "chat_id", update.Message.Chat.ID)
			}
		}
		if err := w.db.RecordMessage(ctx, messageRecord(update.Message)); err != nil {
			w.log.Error("failed to record message", "error", err, "chat_id", update.Message.Chat.ID, "message_id", update.Message.MessageID)
		}
		w.log.Info("dispatching to message handler",
			"update_id", update.UpdateID,
			"message_id", update.Message.MessageID,
			"chat_id", update.Message.Chat.ID,
			"chat_type", update.Message.Chat.Type,
			"text", update.Message.Text,
		)
		commands.Dispatch(ctx, w.client, w.db, update.Message, w.log)
	case update.EditedMessage != nil:
		if update.EditedMessage.Chat == nil {
			return
		}
		if update.EditedMessage.Chat != nil {
			if err := w.db.TrackChat(ctx, update.EditedMessage.Chat.ID, update.EditedMessage.Chat.Type, update.EditedMessage.Chat.Title, update.EditedMessage.Chat.Username); err != nil {
				w.log.Error("failed to track chat", "error", err, "chat_id", update.EditedMessage.Chat.ID)
			}
		}
		if err := w.db.RecordMessage(ctx, messageRecord(update.EditedMessage)); err != nil {
			w.log.Error("failed to record edited message", "error", err, "chat_id", update.EditedMessage.Chat.ID, "message_id", update.EditedMessage.MessageID)
		}
		w.log.Info("dispatching to edited message handler",
			"update_id", update.UpdateID,
			"message_id", update.EditedMessage.MessageID,
			"chat_id", update.EditedMessage.Chat.ID,
			"chat_type", update.EditedMessage.Chat.Type,
			"text", update.EditedMessage.Text,
		)
		commands.Dispatch(ctx, w.client, w.db, update.EditedMessage, w.log)
	case update.MyChatMember != nil:
		w.trackBotChatMembership(ctx, update.MyChatMember)
	}
}

func (w *Webhook) transcribeMedia(msg *telegram.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	fileID, duration, fileSuffix, mediaType := transcriptionMedia(msg)
	if fileID == "" || (msg.Chat.Type != "group" && msg.Chat.Type != "supergroup") {
		return
	}
	settings, err := w.db.GetVoiceRecognitionSettings(ctx, msg.Chat.ID)
	if err != nil {
		w.log.Error("failed to load voice recognition settings", "error", err, "chat_id", msg.Chat.ID)
		return
	}
	if !settings.Enabled || duration > settings.MaxDurationSeconds || !w.canTranscribeVoice(ctx, msg, settings.Permission) {
		return
	}

	media, err := w.client.DownloadFile(ctx, fileID)
	if err != nil {
		w.log.Error("failed to download media message", "error", err, "media_type", mediaType, "chat_id", msg.Chat.ID, "message_id", msg.MessageID)
		return
	}
	defer media.Close()

	jobID, err := w.voices.Enqueue(ctx, media, msg.Chat.ID, msg.MessageID, fileSuffix)
	if err != nil {
		w.log.Error("failed to enqueue media message", "error", err, "media_type", mediaType, "chat_id", msg.Chat.ID, "message_id", msg.MessageID)
		return
	}
	w.log.Info("media message enqueued", "job_id", jobID, "media_type", mediaType, "chat_id", msg.Chat.ID, "message_id", msg.MessageID)
}

func transcriptionMedia(msg *telegram.Message) (fileID string, duration int, fileSuffix, mediaType string) {
	if msg.Voice != nil {
		return msg.Voice.FileID, msg.Voice.Duration, ".ogg", "voice"
	}
	if msg.VideoNote != nil {
		return msg.VideoNote.FileID, msg.VideoNote.Duration, ".mp4", "video_note"
	}
	return "", 0, "", ""
}

func (w *Webhook) canTranscribeVoice(ctx context.Context, msg *telegram.Message, permission string) bool {
	if permission == "user" {
		return true
	}
	if permission != "moderator" || msg.From == nil {
		return false
	}
	member, err := w.client.GetChatMember(ctx, msg.Chat.ID, msg.From.ID)
	if err == nil && member.Status == "creator" {
		return true
	}
	granted, err := w.db.IsChatAdmin(ctx, msg.Chat.ID, msg.From.ID)
	if err != nil {
		w.log.Warn("failed to check voice recognition access", "error", err, "chat_id", msg.Chat.ID, "user_id", msg.From.ID)
		return false
	}
	return granted
}

func messageRecord(msg *telegram.Message) database.MessageRecord {
	record := database.MessageRecord{
		ChatID:       msg.Chat.ID,
		MessageID:    msg.MessageID,
		ChatType:     msg.Chat.Type,
		ChatTitle:    msg.Chat.Title,
		ChatUsername: msg.Chat.Username,
		Text:         msg.Text,
		SentAt:       int64(msg.Date),
	}
	if msg.From != nil {
		record.SenderID = msg.From.ID
		record.SenderUsername = msg.From.Username
	}
	return record
}

func (w *Webhook) trackBotChatMembership(ctx context.Context, update *telegram.ChatMemberUpdated) {
	if update.Chat == nil || update.NewChatMember == nil {
		return
	}
	if update.Chat.Type != "group" && update.Chat.Type != "supergroup" {
		return
	}
	if update.NewChatMember.Status != "member" && update.NewChatMember.Status != "administrator" {
		return
	}

	if err := w.db.TrackChat(ctx, update.Chat.ID, update.Chat.Type, update.Chat.Title, update.Chat.Username); err != nil {
		w.log.Error("failed to track bot chat membership", "error", err, "chat_id", update.Chat.ID)
		return
	}
	w.log.Info("tracked bot chat membership", "chat_id", update.Chat.ID, "chat_type", update.Chat.Type)
}
