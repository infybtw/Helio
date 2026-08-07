package commands

import (
	"context"
	"log/slog"
	"strings"

	"tg_bot/internal/database"
	"tg_bot/internal/telegram"
)

// Ban handles the !ban command.
// It bans the author of the replied-to message from the chat, deletes the
// replied-to message, then deletes the command message itself.
func Ban(ctx context.Context, client *telegram.Client, _ database.Store, msg *telegram.Message, log *slog.Logger) {
	if msg == nil || msg.Chat == nil {
		log.Warn("ban called with nil message or chat")
		return
	}

	log.Info("ban command received",
		"chat_id", msg.Chat.ID,
		"chat_type", msg.Chat.Type,
		"message_id", msg.MessageID,
		"text", msg.Text,
		"is_reply", msg.ReplyToMessage != nil,
	)

	chatType := msg.Chat.Type
	if chatType != "group" && chatType != "supergroup" {
		log.Info("skipping !ban outside of a group",
			"chat_id", msg.Chat.ID,
			"chat_type", chatType,
		)
		return
	}

	fields := strings.Fields(msg.Text)
	if len(fields) == 0 || !strings.EqualFold(fields[0], "!ban") {
		return
	}

	if msg.ReplyToMessage == nil || msg.ReplyToMessage.From == nil {
		log.Info("skipping !ban without a replied-to user",
			"chat_id", msg.Chat.ID,
			"message_id", msg.MessageID,
		)
		return
	}

	chatID := msg.Chat.ID
	target := msg.ReplyToMessage.From

	if err := client.BanChatMember(ctx, chatID, target.ID, 0, false); err != nil {
		log.Error("failed to ban user",
			"chat_id", chatID,
			"user_id", target.ID,
			"error", err,
		)
	} else {
		log.Info("banned user",
			"chat_id", chatID,
			"user_id", target.ID,
			"username", target.Username,
		)
	}

	if err := client.DeleteMessage(ctx, chatID, msg.ReplyToMessage.MessageID); err != nil {
		log.Error("failed to delete replied message",
			"chat_id", chatID,
			"message_id", msg.ReplyToMessage.MessageID,
			"error", err,
		)
	}

	if err := client.DeleteMessage(ctx, chatID, msg.MessageID); err != nil {
		log.Error("failed to delete command message",
			"chat_id", chatID,
			"message_id", msg.MessageID,
			"error", err,
		)
	}
}
