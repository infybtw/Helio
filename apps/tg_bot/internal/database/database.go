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
	ActorUsername   string
	Action          string
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
	// Close releases database resources.
	Close()
}
