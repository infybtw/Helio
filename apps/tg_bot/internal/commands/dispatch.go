package commands

import (
	"context"
	"log/slog"
	"strings"
	"time"

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
		reply := msg.ReplyToMessage
		action := database.ActionRecord{
			ChatID:    msg.Chat.ID,
			MessageID: msg.MessageID,
			Action:    custom.Name,
			EventType: "custom",
		}
		if reply != nil {
			action.TargetMessageID = reply.MessageID
			if reply.From != nil {
				action.TargetUserID = reply.From.ID
			}
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

		for _, action := range custom.Actions {
			switch action.Type {
			case "send_message":
				if err := client.SendMessage(ctx, msg.Chat.ID, action.Payload, 0); err != nil {
					log.Warn("failed to execute custom command action", "error", err, "chat_id", msg.Chat.ID, "command", command, "action_type", action.Type)
				}
			case "reply_message":
				if err := client.SendMessage(ctx, msg.Chat.ID, action.Payload, msg.MessageID); err != nil {
					log.Warn("failed to execute custom reply action", "error", err, "chat_id", msg.Chat.ID, "command", command, "target_message_id", msg.MessageID)
				}
			case "mute":
				if reply == nil || reply.From == nil {
					log.Warn("cannot mute without a replied-to user", "chat_id", msg.Chat.ID, "command", command)
					continue
				}
				duration := defaultMuteDuration
				if action.Payload != "" {
					parsed, err := parseDuration(action.Payload)
					if err != nil {
						log.Warn("invalid custom mute duration", "error", err, "chat_id", msg.Chat.ID, "command", command, "duration", action.Payload)
						continue
					}
					duration = parsed
				}
				target := reply.From
				member, err := client.GetChatMember(ctx, msg.Chat.ID, target.ID)
				if err != nil {
					log.Warn("cannot verify custom mute target", "error", err, "chat_id", msg.Chat.ID, "command", command, "target_user_id", target.ID)
					continue
				}
				if member.Status == "creator" {
					log.Warn("skipping custom mute for chat owner", "chat_id", msg.Chat.ID, "command", command, "target_user_id", target.ID)
					continue
				}
				if err := client.RestrictChatMember(ctx, msg.Chat.ID, target.ID, telegram.MutePermissions, time.Now().Add(duration).Unix()); err != nil {
					log.Warn("failed to execute custom mute action", "error", err, "chat_id", msg.Chat.ID, "command", command, "target_user_id", target.ID)
				} else {
					log.Info("executed custom mute action", "chat_id", msg.Chat.ID, "command", command, "target_user_id", target.ID)
				}
			case "delete_message":
				if reply == nil {
					log.Warn("cannot delete without a replied-to message", "chat_id", msg.Chat.ID, "command", command)
					continue
				}
				if err := client.DeleteMessage(ctx, msg.Chat.ID, reply.MessageID); err != nil {
					log.Warn("failed to execute custom delete action", "error", err, "chat_id", msg.Chat.ID, "command", command, "target_message_id", reply.MessageID)
				} else {
					log.Info("executed custom delete action", "chat_id", msg.Chat.ID, "command", command, "target_message_id", reply.MessageID)
				}
			}
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
