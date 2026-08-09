// Package database defines the abstract storage layer used by the bot.
// Callers depend on the Store interface, never on SQL.
package database

import "context"

// MessageRecord is a Telegram message retained for moderation history.
type MessageRecord struct {
	ChatID         int64
	MessageID      int64
	ChatType       string
	ChatTitle      string
	ChatUsername   string
	SenderID       int64
	SenderUsername string
	Text           string
	SentAt         int64
}

// ActionRecord describes a successful moderation or access-control action.
type ActionRecord struct {
	ChatID          int64
	MessageID       int64
	ActorID         int64
	ActorFirstName  string
	Action          string
	EventType       string
	TargetMessageID int64
	TargetUserID    int64
	TargetUsername  string
}

type DashboardChat struct {
	ChatID       int64  `json:"chat_id"`
	Name         string `json:"name"`
	Handle       string `json:"handle"`
	Members      int64  `json:"members"`
	Participants int64  `json:"participants"`
	Actions      int64  `json:"actions"`
	Status       string `json:"status"`
	Initials     string `json:"initials"`
}

type DashboardActivity struct {
	Action          string `json:"action"`
	EventType       string `json:"event_type"`
	Actor           string `json:"actor"`
	Chat            string `json:"chat"`
	TargetMessageID int64  `json:"target_message_id"`
	CreatedAt       string `json:"created_at"`
}

type DashboardData struct {
	ProtectedChats  int64               `json:"protected_chats"`
	ActionsThisWeek int64               `json:"actions_this_week"`
	MessagesCleaned int64               `json:"messages_cleaned"`
	Chats           []DashboardChat     `json:"chats"`
	Activity        []DashboardActivity `json:"activity"`
}

type ActivityPage struct {
	Items []DashboardActivity `json:"items"`
	Total int64               `json:"total"`
}

type CustomCommand struct {
	ID         int64                 `json:"id"`
	ChatID     int64                 `json:"chat_id"`
	Name       string                `json:"name"`
	Aliases    []string              `json:"aliases"`
	Actions    []CustomCommandAction `json:"actions"`
	Enabled    bool                  `json:"enabled"`
	Permission string                `json:"permission"`
	CreatedAt  string                `json:"created_at"`
}

type CustomCommandAction struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
}

// BuiltInCommandSetting is an owner-configured override for a built-in command.
// Commands without a row remain enabled by default.
type BuiltInCommandSetting struct {
	ChatID       int64  `json:"chat_id"`
	Command      string `json:"command"`
	Enabled      bool   `json:"enabled"`
	Permission   string `json:"permission"`
	MuteDuration string `json:"mute_duration"`
	ReplyMessage string `json:"reply_message"`
}

// VoiceRecognitionSettings controls transcription for one group.
type VoiceRecognitionSettings struct {
	ChatID             int64  `json:"chat_id"`
	Enabled            bool   `json:"enabled"`
	Permission         string `json:"permission"`
	MaxDurationSeconds int    `json:"max_duration_seconds"`
}

// Store is the abstract database interface. Implementations hide all SQL.
type Store interface {
	// TrackChat records a chat where the bot received a trusted Telegram update.
	TrackChat(ctx context.Context, chatID int64, chatType, title, username string) error
	// RecordMessage stores an incoming message for later moderation history.
	RecordMessage(ctx context.Context, message MessageRecord) error
	// RecordAction stores a successful action performed by the bot.
	RecordAction(ctx context.Context, action ActionRecord) error
	// ListTrackedChatIDs returns chats in which the bot has received updates.
	ListTrackedChatIDs(ctx context.Context) ([]int64, error)
	// IsChatAdmin reports whether the user has been granted command rights in the chat.
	IsChatAdmin(ctx context.Context, chatID, userID int64) (bool, error)
	// GrantChatAdmin grants command rights to the user in the chat.
	GrantChatAdmin(ctx context.Context, chatID, userID, grantedBy int64) error
	// RevokeChatAdmin revokes command rights from the user in the chat.
	// It reports whether a row was removed.
	RevokeChatAdmin(ctx context.Context, chatID, userID int64) (bool, error)
	// DashboardData returns moderation metrics for the supplied chats.
	DashboardData(ctx context.Context, chatIDs []int64) (DashboardData, error)
	ListActivity(ctx context.Context, chatIDs []int64, eventType string, limit, offset int) (ActivityPage, error)
	ListCustomCommands(ctx context.Context, chatIDs []int64) ([]CustomCommand, error)
	CreateCustomCommand(ctx context.Context, chatID, createdBy int64, name, permission string, aliases []string, actions []CustomCommandAction) (CustomCommand, error)
	UpdateCustomCommand(ctx context.Context, id, chatID int64, chatIDs []int64, name, permission string, aliases []string, actions []CustomCommandAction) (CustomCommand, bool, error)
	DeleteCustomCommand(ctx context.Context, id int64, chatIDs []int64) (CustomCommand, bool, error)
	SetCustomCommandEnabled(ctx context.Context, id int64, enabled bool, chatIDs []int64) (bool, error)
	FindCustomCommand(ctx context.Context, chatID int64, name string) (CustomCommand, bool, error)
	ListBuiltInCommandSettings(ctx context.Context, chatID int64) ([]BuiltInCommandSetting, error)
	SetBuiltInCommandEnabled(ctx context.Context, chatID int64, command string, enabled bool) error
	UpdateBuiltInCommandSetting(ctx context.Context, setting BuiltInCommandSetting) error
	ResetBuiltInCommandSettings(ctx context.Context, chatID int64) error
	GetBuiltInCommandSetting(ctx context.Context, chatID int64, command string) (BuiltInCommandSetting, bool, error)
	IsBuiltInCommandEnabled(ctx context.Context, chatID int64, command string) (bool, error)
	GetVoiceRecognitionSettings(ctx context.Context, chatID int64) (VoiceRecognitionSettings, error)
	UpdateVoiceRecognitionSettings(ctx context.Context, settings VoiceRecognitionSettings) error
	// Close releases database resources.
	Close()
}
