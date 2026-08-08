package commands

import (
	"context"
	"log/slog"
	"strings"

	"tg_bot/internal/database"
	"tg_bot/internal/telegram"
)

// Delete handles the !delete command.
// It deletes the message that the command replies to, then deletes the command message itself.
func Delete(ctx context.Context, client *telegram.Client, st database.Store, msg *telegram.Message, log *slog.Logger) {
	if msg == nil || msg.Chat == nil {
		log.Warn("delete called with nil message or chat")
		return
	}

	log.Info("delete command received",
		"chat_id", msg.Chat.ID,
		"chat_type", msg.Chat.Type,
		"message_id", msg.MessageID,
		"text", msg.Text,
		"is_reply", msg.ReplyToMessage != nil,
	)

	// Only allow !delete in groups/supergroups (channel comments are discussion groups).
	chatType := msg.Chat.Type
	if chatType != "group" && chatType != "supergroup" {
		log.Info("skipping !delete outside of a group",
			"chat_id", msg.Chat.ID,
			"chat_type", chatType,
		)
		return
	}

	text := strings.TrimSpace(msg.Text)
	if !strings.HasPrefix(strings.ToLower(text), "!delete") {
		log.Info("not a !delete command",
			"chat_id", msg.Chat.ID,
			"message_id", msg.MessageID,
			"text", text,
		)
		return
	}

	chatID := msg.Chat.ID
	log.Info("processing !delete command",
		"chat_id", chatID,
		"message_id", msg.MessageID,
		"reply_to_message_id", func() int64 {
			if msg.ReplyToMessage != nil {
				return msg.ReplyToMessage.MessageID
			}
			return 0
		}(),
	)

	// If the command is a reply, delete the replied-to message first.
	if msg.ReplyToMessage != nil {
		if err := client.DeleteMessage(ctx, chatID, msg.ReplyToMessage.MessageID); err != nil {
			log.Error("failed to delete replied message",
				"chat_id", chatID,
				"message_id", msg.ReplyToMessage.MessageID,
				"error", err,
			)
		} else {
			if err := recordAction(ctx, st, msg, "!delete", msg.ReplyToMessage); err != nil {
				log.Error("failed to record delete action", "error", err, "chat_id", chatID, "message_id", msg.ReplyToMessage.MessageID)
			}
			log.Info("deleted replied message",
				"chat_id", chatID,
				"message_id", msg.ReplyToMessage.MessageID,
			)
		}
	}

	// Delete the command message itself.
	if err := client.DeleteMessage(ctx, chatID, msg.MessageID); err != nil {
		log.Error("failed to delete command message",
			"chat_id", chatID,
			"message_id", msg.MessageID,
			"error", err,
		)
	} else {
		log.Info("deleted command message",
			"chat_id", chatID,
			"message_id", msg.MessageID,
		)
	}
}
