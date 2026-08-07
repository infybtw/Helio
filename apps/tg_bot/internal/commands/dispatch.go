package commands

import (
	"context"
	"log/slog"
	"strings"

	"tg_bot/internal/telegram"
)

type handler func(ctx context.Context, client *telegram.Client, msg *telegram.Message, log *slog.Logger)

// adminCommands can be used only by the group owner.
var adminCommands = map[string]handler{
	"!delete": Delete,
	"!mute":   Mute,
	"!ban":    Ban,
}

// userCommands can be used by any group member.
var userCommands = map[string]handler{}

// Dispatch routes a message to the matching command handler based on the
// first word of its text.
func Dispatch(ctx context.Context, client *telegram.Client, msg *telegram.Message, log *slog.Logger) {
	if msg == nil {
		return
	}

	fields := strings.Fields(msg.Text)
	if len(fields) == 0 {
		return
	}

	command := strings.ToLower(fields[0])

	if h, ok := userCommands[command]; ok {
		h(ctx, client, msg, log)
		return
	}

	if h, ok := adminCommands[command]; ok {
		if msg.From == nil || msg.Chat == nil {
			return
		}
		if !isGroupOwner(ctx, client, msg, log) {
			log.Warn("ignoring admin command from non-owner",
				"command", command,
				"chat_id", msg.Chat.ID,
				"user_id", msg.From.ID,
				"username", msg.From.Username,
			)
			return
		}
		h(ctx, client, msg, log)
	}
}

// isGroupOwner reports whether the message sender is the owner of the chat.
func isGroupOwner(ctx context.Context, client *telegram.Client, msg *telegram.Message, log *slog.Logger) bool {
	if msg.From == nil || msg.Chat == nil {
		return false
	}

	member, err := client.GetChatMember(ctx, msg.Chat.ID, msg.From.ID)
	if err != nil {
		log.Warn("failed to get chat member",
			"error", err,
			"chat_id", msg.Chat.ID,
			"user_id", msg.From.ID,
		)
		return false
	}

	return member.Status == "creator"
}
