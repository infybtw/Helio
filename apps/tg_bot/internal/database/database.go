// Package database defines the abstract storage layer used by the bot.
// Callers depend on the Store interface, never on SQL.
package database

import "context"

// Store is the abstract database interface. Implementations hide all SQL.
type Store interface {
	// TrackChat records a chat where the bot received a trusted Telegram update.
	TrackChat(ctx context.Context, chatID int64, chatType, title, username string) error
	// ListTrackedChatIDs returns chats in which the bot has received updates.
	ListTrackedChatIDs(ctx context.Context) ([]int64, error)
	// IsChatAdmin reports whether the user has been granted command rights in the chat.
	IsChatAdmin(ctx context.Context, chatID, userID int64) (bool, error)
	// GrantChatAdmin grants command rights to the user in the chat.
	GrantChatAdmin(ctx context.Context, chatID, userID, grantedBy int64) error
	// RevokeChatAdmin revokes command rights from the user in the chat.
	// It reports whether a row was removed.
	RevokeChatAdmin(ctx context.Context, chatID, userID int64) (bool, error)
	// Close releases database resources.
	Close()
}
