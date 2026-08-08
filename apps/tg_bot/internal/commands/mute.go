package commands

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"tg_bot/internal/database"
	"tg_bot/internal/telegram"
)

const defaultMuteDuration = 30 * time.Minute

// Mute handles the !mute command.
// It mutes the author of the replied-to message for the given duration
// (default 30 minutes), deletes the replied-to message, then deletes the
// command message itself.
func Mute(ctx context.Context, client *telegram.Client, st database.Store, msg *telegram.Message, log *slog.Logger) {
	if msg == nil || msg.Chat == nil {
		log.Warn("mute called with nil message or chat")
		return
	}

	log.Info("mute command received",
		"chat_id", msg.Chat.ID,
		"chat_type", msg.Chat.Type,
		"message_id", msg.MessageID,
		"text", msg.Text,
		"is_reply", msg.ReplyToMessage != nil,
	)

	chatType := msg.Chat.Type
	if chatType != "group" && chatType != "supergroup" {
		log.Info("skipping !mute outside of a group",
			"chat_id", msg.Chat.ID,
			"chat_type", chatType,
		)
		return
	}

	fields := strings.Fields(strings.TrimSpace(msg.Text))
	if len(fields) == 0 || !strings.EqualFold(fields[0], "!mute") {
		return
	}

	if msg.ReplyToMessage == nil || msg.ReplyToMessage.From == nil {
		log.Info("skipping !mute without a replied-to user",
			"chat_id", msg.Chat.ID,
			"message_id", msg.MessageID,
		)
		return
	}

	duration := defaultMuteDuration
	if len(fields) > 1 {
		parsed, err := parseDuration(fields[1])
		if err != nil {
			log.Warn("invalid !mute duration",
				"chat_id", msg.Chat.ID,
				"message_id", msg.MessageID,
				"arg", fields[1],
				"error", err,
			)
			return
		}
		duration = parsed
	}

	chatID := msg.Chat.ID
	target := msg.ReplyToMessage.From
	untilDate := time.Now().Add(duration).Unix()

	if err := client.RestrictChatMember(ctx, chatID, target.ID, telegram.MutePermissions, untilDate); err != nil {
		log.Error("failed to mute user",
			"chat_id", chatID,
			"user_id", target.ID,
			"duration", duration.String(),
			"error", err,
		)
	} else {
		log.Info("muted user",
			"chat_id", chatID,
			"user_id", target.ID,
			"username", target.Username,
			"duration", duration.String(),
			"until_date", untilDate,
		)
	}

	if err := client.DeleteMessage(ctx, chatID, msg.ReplyToMessage.MessageID); err != nil {
		log.Error("failed to delete replied message",
			"chat_id", chatID,
			"message_id", msg.ReplyToMessage.MessageID,
			"error", err,
		)
	} else if err := recordAction(ctx, st, msg, "!mute", msg.ReplyToMessage); err != nil {
		log.Error("failed to record mute action", "error", err, "chat_id", chatID, "message_id", msg.ReplyToMessage.MessageID)
	}

	if err := client.DeleteMessage(ctx, chatID, msg.MessageID); err != nil {
		log.Error("failed to delete command message",
			"chat_id", chatID,
			"message_id", msg.MessageID,
			"error", err,
		)
	}
}

// parseDuration parses a duration string. It accepts a bare number of minutes
// (e.g. "30"), Go duration syntax (e.g. "30m", "1h30m") and additionally a "d"
// suffix for days (e.g. "1d", "2d12h").
func parseDuration(s string) (time.Duration, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}

	if minutes, err := strconv.Atoi(s); err == nil {
		if minutes <= 0 {
			return 0, fmt.Errorf("duration must be positive: %q", s)
		}
		return time.Duration(minutes) * time.Minute, nil
	}

	if idx := strings.Index(s, "d"); idx > 0 {
		days, err := strconv.Atoi(s[:idx])
		if err != nil {
			return 0, fmt.Errorf("invalid days value: %q", s[:idx])
		}
		d := time.Duration(days) * 24 * time.Hour
		if rest := s[idx+1:]; rest != "" {
			extra, err := time.ParseDuration(rest)
			if err != nil {
				return 0, fmt.Errorf("invalid duration: %w", err)
			}
			d += extra
		}
		return d, nil
	}

	return 0, fmt.Errorf("invalid duration: %q", s)
}
