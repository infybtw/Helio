package commands

import (
	"context"
	"log/slog"

	"tg_bot/internal/database"
	"tg_bot/internal/telegram"
)

// Grant handles the !grant command.
// It grants command rights in the chat to the author of the replied-to message,
// then deletes the command message itself.
func Grant(ctx context.Context, client *telegram.Client, st database.Store, msg *telegram.Message, log *slog.Logger) {
	if msg == nil || msg.Chat == nil || msg.From == nil {
		log.Warn("grant called with nil message, chat or sender")
		return
	}

	chatType := msg.Chat.Type
	if chatType != "group" && chatType != "supergroup" {
		log.Info("skipping !grant outside of a group",
			"chat_id", msg.Chat.ID,
			"chat_type", chatType,
		)
		return
	}

	if msg.ReplyToMessage == nil || msg.ReplyToMessage.From == nil {
		log.Info("skipping !grant without a replied-to user",
			"chat_id", msg.Chat.ID,
			"message_id", msg.MessageID,
		)
		return
	}

	chatID := msg.Chat.ID
	target := msg.ReplyToMessage.From

	if err := st.GrantChatAdmin(ctx, chatID, target.ID, msg.From.ID); err != nil {
		log.Error("failed to grant command rights",
			"chat_id", chatID,
			"user_id", target.ID,
			"error", err,
		)
		return
	}

	log.Info("granted command rights",
		"chat_id", chatID,
		"user_id", target.ID,
		"username", target.Username,
		"granted_by", msg.From.ID,
	)
	if err := recordAction(ctx, st, msg, "!grant", msg.ReplyToMessage); err != nil {
		log.Error("failed to record grant action", "error", err, "chat_id", chatID)
	}

	if err := client.DeleteMessage(ctx, chatID, msg.MessageID); err != nil {
		log.Error("failed to delete command message",
			"chat_id", chatID,
			"message_id", msg.MessageID,
			"error", err,
		)
	}
}

// Revoke handles the !revoke command.
// It revokes command rights in the chat from the author of the replied-to
// message, then deletes the command message itself.
func Revoke(ctx context.Context, client *telegram.Client, st database.Store, msg *telegram.Message, log *slog.Logger) {
	if msg == nil || msg.Chat == nil || msg.From == nil {
		log.Warn("revoke called with nil message, chat or sender")
		return
	}

	chatType := msg.Chat.Type
	if chatType != "group" && chatType != "supergroup" {
		log.Info("skipping !revoke outside of a group",
			"chat_id", msg.Chat.ID,
			"chat_type", chatType,
		)
		return
	}

	if msg.ReplyToMessage == nil || msg.ReplyToMessage.From == nil {
		log.Info("skipping !revoke without a replied-to user",
			"chat_id", msg.Chat.ID,
			"message_id", msg.MessageID,
		)
		return
	}

	chatID := msg.Chat.ID
	target := msg.ReplyToMessage.From

	removed, err := st.RevokeChatAdmin(ctx, chatID, target.ID)
	if err != nil {
		log.Error("failed to revoke command rights",
			"chat_id", chatID,
			"user_id", target.ID,
			"error", err,
		)
		return
	}

	log.Info("revoked command rights",
		"chat_id", chatID,
		"user_id", target.ID,
		"username", target.Username,
		"revoked_by", msg.From.ID,
		"had_rights", removed,
	)
	if err := recordAction(ctx, st, msg, "!revoke", msg.ReplyToMessage); err != nil {
		log.Error("failed to record revoke action", "error", err, "chat_id", chatID)
	}

	if err := client.DeleteMessage(ctx, chatID, msg.MessageID); err != nil {
		log.Error("failed to delete command message",
			"chat_id", chatID,
			"message_id", msg.MessageID,
			"error", err,
		)
	}
}
