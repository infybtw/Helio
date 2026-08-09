package commands

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"tg_bot/internal/database"
	"tg_bot/internal/telegram"
)

type handler func(ctx context.Context, client *telegram.Client, st database.Store, msg *telegram.Message, log *slog.Logger)

// BuiltInCommand describes a command configurable by a chat owner.
type BuiltInCommand struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Permission  string `json:"permission"`
}

var builtInCommands = []BuiltInCommand{
	{Name: "!delete", Description: "Delete the replied-to message.", Permission: "moderator"},
	{Name: "!mute", Description: "Mute the replied-to user and delete their message.", Permission: "moderator"},
	{Name: "!ban", Description: "Ban the replied-to user and delete their message.", Permission: "moderator"},
	{Name: "!grant", Description: "Grant moderation command rights to the replied-to user.", Permission: "owner"},
	{Name: "!revoke", Description: "Revoke moderation command rights from the replied-to user.", Permission: "owner"},
	{Name: "!help", Description: "Show the available bot commands.", Permission: "user"},
}

// BuiltInCommands returns the commands owners can enable or disable per chat.
func BuiltInCommands() []BuiltInCommand {
	return append([]BuiltInCommand(nil), builtInCommands...)
}

var builtInHandlers = map[string]handler{
	"!delete": Delete,
	"!mute":   Mute,
	"!ban":    Ban,
	"!grant":  Grant,
	"!revoke": Revoke,
	"!help":   Help,
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
	if builtIn, ok := findBuiltInCommand(command); ok {
		if msg.Chat == nil {
			return
		}
		setting, configured, err := st.GetBuiltInCommandSetting(ctx, msg.Chat.ID, command)
		if err != nil {
			log.Warn("failed to load built-in command setting", "error", err, "chat_id", msg.Chat.ID, "command", command)
			return
		}
		if configured && !setting.Enabled {
			return
		}
		permission := builtIn.Permission
		if configured && setting.Permission != "" {
			permission = setting.Permission
		}
		if msg.From == nil || !hasBuiltInCommandPermission(ctx, client, st, msg, permission, log) {
			logUnauthorized(log, command, msg)
			return
		}
		if command != "!help" && configured && setting.ReplyMessage != "" {
			if err := client.SendMessage(ctx, msg.Chat.ID, interpolateVariables(setting.ReplyMessage, msg), msg.MessageID); err != nil {
				log.Warn("failed to send built-in command reply", "error", err, "chat_id", msg.Chat.ID, "command", command)
			}
		}
		builtInHandlers[command](ctx, client, st, msg, log)
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
		if msg.From == nil || !hasCustomCommandPermission(ctx, client, st, msg, custom.Permission, log) {
			return
		}
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
				if err := client.SendMessage(ctx, msg.Chat.ID, interpolateVariables(action.Payload, msg), 0); err != nil {
					log.Warn("failed to execute custom command action", "error", err, "chat_id", msg.Chat.ID, "command", command, "action_type", action.Type)
				}
			case "reply_message":
				if err := client.SendMessage(ctx, msg.Chat.ID, interpolateVariables(action.Payload, msg), msg.MessageID); err != nil {
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

func isBuiltInCommand(command string) bool {
	_, ok := findBuiltInCommand(command)
	return ok
}

func findBuiltInCommand(command string) (BuiltInCommand, bool) {
	for _, builtIn := range builtInCommands {
		if builtIn.Name == command {
			return builtIn, true
		}
	}
	return BuiltInCommand{}, false
}

func hasBuiltInCommandPermission(ctx context.Context, client *telegram.Client, st database.Store, msg *telegram.Message, permission string, log *slog.Logger) bool {
	switch permission {
	case "user":
		return true
	case "owner":
		return isGroupOwner(ctx, client, msg, log)
	case "moderator":
		return isGroupOwner(ctx, client, msg, log) || isChatAdmin(ctx, st, msg, log)
	default:
		log.Warn("ignoring built-in command with invalid permission", "chat_id", msg.Chat.ID, "permission", permission)
		return false
	}
}

func hasCustomCommandPermission(ctx context.Context, client *telegram.Client, st database.Store, msg *telegram.Message, permission string, log *slog.Logger) bool {
	switch permission {
	case "user":
		return true
	case "owner":
		return isGroupOwner(ctx, client, msg, log)
	case "moderator":
		if isGroupOwner(ctx, client, msg, log) {
			return true
		}
		return isChatAdmin(ctx, st, msg, log)
	default:
		log.Warn("ignoring custom command with invalid permission", "chat_id", msg.Chat.ID, "permission", permission)
		return false
	}
}

func interpolateVariables(text string, msg *telegram.Message) string {
	username, firstName, userID := "", "", ""
	replyUsername, replyFirstName := "", ""
	if msg != nil && msg.From != nil {
		username = msg.From.Username
		firstName = msg.From.FirstName
		userID = strconv.FormatInt(msg.From.ID, 10)
	}
	if msg != nil && msg.ReplyToMessage != nil && msg.ReplyToMessage.From != nil {
		replyUsername = msg.ReplyToMessage.From.Username
		replyFirstName = msg.ReplyToMessage.From.FirstName
	}
	return strings.NewReplacer(
		"{{username}}", username,
		"{{firstname}}", firstName,
		"{{user_id}}", userID,
		"{{reply_username}}", replyUsername,
		"{{reply_firstname}}", replyFirstName,
	).Replace(text)
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
