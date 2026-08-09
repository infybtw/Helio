package commands

import (
	"context"
	"log/slog"
	"strings"

	"tg_bot/internal/database"
	"tg_bot/internal/telegram"
)

const helpText = `Available commands:

!help - show this help message

Admin commands (owner and granted users, use by replying to a message):
!delete - delete the replied-to message
!mute [duration] - mute the user (default 30m, e.g. 10m, 2h, 1d)
!ban - ban the user from the chat

Owner commands (use by replying to a message):
!grant - grant admin command rights to the user
!revoke - revoke admin command rights from the user`

// HelpText returns the default !help output used when a chat has no custom text.
func HelpText() string {
	return helpText
}

// Help handles the !help command.
// It replies to the command message with a list of all available commands.
func Help(ctx context.Context, client *telegram.Client, st database.Store, msg *telegram.Message, log *slog.Logger) {
	if msg == nil || msg.Chat == nil {
		log.Warn("help called with nil message or chat")
		return
	}

	fields := strings.Fields(msg.Text)
	if len(fields) == 0 || !strings.EqualFold(fields[0], "!help") {
		return
	}

	output := helpText
	setting, configured, err := st.GetBuiltInCommandSetting(ctx, msg.Chat.ID, "!help")
	if err != nil {
		log.Warn("failed to load !help output setting", "error", err, "chat_id", msg.Chat.ID)
	} else if configured && setting.ReplyMessage != "" {
		output = interpolateVariables(setting.ReplyMessage, msg)
	}
	if err := client.SendMessage(ctx, msg.Chat.ID, output, msg.MessageID); err != nil {
		log.Error("failed to send help message",
			"chat_id", msg.Chat.ID,
			"message_id", msg.MessageID,
			"error", err,
		)
	}
}
