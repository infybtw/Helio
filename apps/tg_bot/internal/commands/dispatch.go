package commands

import (
	"context"
	"log/slog"
	"strings"

	"tg_bot/internal/database"
	"tg_bot/internal/telegram"
)

type handler func(ctx context.Context, client *telegram.Client, st database.Store, msg *telegram.Message, log *slog.Logger)

// adminCommands can be used by the group owner and users granted rights via !grant.
var adminCommands = map[string]handler{
	"!delete": Delete,
	"!mute":   Mute,
	"!ban":    Ban,
}

// ownerCommands can be used only by the group owner.
var ownerCommands = map[string]handler{
	"!grant":  Grant,
	"!revoke": Revoke,
}

// userCommands can be used by any group member.
var userCommands = map[string]handler{
	"!help": Help,
}

// Dispatch routes a message to the matching command handler based on the
// first word of its text.
func Dispatch(ctx context.Context, client *telegram.Client, st database.Store, msg *telegram.Message, log *slog.Logger) {
	if msg == nil {
		return
	}

	fields := strings.Fields(msg.Text)
	if len(fields) == 0 {
		return
	}

	command := strings.ToLower(fields[0])

	if h, ok := userCommands[command]; ok {
		h(ctx, client, st, msg, log)
		return
	}

	if h, ok := ownerCommands[command]; ok {
		if msg.From == nil || msg.Chat == nil {
			return
		}
		if !isGroupOwner(ctx, client, msg, log) {
			logUnauthorized(log, command, msg)
			return
		}
		h(ctx, client, st, msg, log)
		return
	}

	if h, ok := adminCommands[command]; ok {
		if msg.From == nil || msg.Chat == nil {
			return
		}
		if !isGroupOwner(ctx, client, msg, log) && !isChatAdmin(ctx, st, msg, log) {
			logUnauthorized(log, command, msg)
			return
		}
		h(ctx, client, st, msg, log)
		return
	}

	if msg.Chat == nil {
		return
	}
	custom, ok, err := st.FindCustomCommand(ctx, msg.Chat.ID, command)
	if err != nil {
		log.Warn("failed to find custom command", "error", err, "chat_id", msg.Chat.ID, "command", command)
		return
	}
	if ok {
		action := database.ActionRecord{
			ChatID:    msg.Chat.ID,
			MessageID: msg.MessageID,
			Action:    custom.Name,
			EventType: "custom",
		}
		if msg.From != nil {
			action.ActorID = msg.From.ID
			action.ActorFirstName = msg.From.FirstName
		}
		if err := st.RecordAction(ctx, action); err != nil {
			log.Warn("failed to record custom command activity", "error", err, "chat_id", msg.Chat.ID, "command", custom.Name)
		} else {
			log.Info("custom command activity recorded", "chat_id", msg.Chat.ID, "command", custom.Name, "user_id", action.ActorID)
		}

		if err := client.SendMessage(ctx, msg.Chat.ID, custom.Response, msg.MessageID); err != nil {
			log.Warn("failed to send custom command response", "error", err, "chat_id", msg.Chat.ID, "command", command)
		}
	}
}

func logUnauthorized(log *slog.Logger, command string, msg *telegram.Message) {
	log.Warn("ignoring command from unauthorized user",
		"command", command,
		"chat_id", msg.Chat.ID,
		"user_id", msg.From.ID,
		"username", msg.From.Username,
	)
}

// isGroupOwner reports whether the message sender is the owner of the chat.
func isGroupOwner(ctx context.Context, client *telegram.Client, msg *telegram.Message, log *slog.Logger) bool {
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

// isChatAdmin reports whether the message sender has been granted command
// rights in the chat.
func isChatAdmin(ctx context.Context, st database.Store, msg *telegram.Message, log *slog.Logger) bool {
	ok, err := st.IsChatAdmin(ctx, msg.Chat.ID, msg.From.ID)
	if err != nil {
		log.Warn("failed to check command rights",
			"error", err,
			"chat_id", msg.Chat.ID,
			"user_id", msg.From.ID,
		)
		return false
	}

	return ok
}
