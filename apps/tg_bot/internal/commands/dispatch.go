package commands

import (
	"context"
	"log/slog"
	"strings"

	"tg_bot/internal/telegram"
)

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

	switch strings.ToLower(fields[0]) {
	case "!delete":
		Delete(ctx, client, msg, log)
	case "!mute":
		Mute(ctx, client, msg, log)
	case "!ban":
		Ban(ctx, client, msg, log)
	}
}
