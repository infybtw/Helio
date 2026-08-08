package telegram

import "encoding/json"

// Update represents an incoming update from Telegram.
type Update struct {
	UpdateID          int64              `json:"update_id"`
	Message           *Message           `json:"message,omitempty"`
	EditedMessage     *Message           `json:"edited_message,omitempty"`
	ChannelPost       *Message           `json:"channel_post,omitempty"`
	EditedChannelPost *Message           `json:"edited_channel_post,omitempty"`
	MyChatMember      *ChatMemberUpdated `json:"my_chat_member,omitempty"`
	CallbackQuery     *CallbackQuery     `json:"callback_query,omitempty"`
}

// Message represents a message.
type Message struct {
	MessageID       int64           `json:"message_id"`
	MessageThreadID int64           `json:"message_thread_id,omitempty"`
	From            *User           `json:"from,omitempty"`
	SenderChat      *Chat           `json:"sender_chat,omitempty"`
	Date            int             `json:"date"`
	Chat            *Chat           `json:"chat"`
	Text            string          `json:"text,omitempty"`
	ReplyToMessage  *Message        `json:"reply_to_message,omitempty"`
	Entities        []MessageEntity `json:"entities,omitempty"`
}

// MessageEntity represents one special entity in a text message.
type MessageEntity struct {
	Type   string `json:"type"`
	Offset int    `json:"offset"`
	Length int    `json:"length"`
	URL    string `json:"url,omitempty"`
	User   *User  `json:"user,omitempty"`
}

// Chat represents a chat.
type Chat struct {
	ID       int64  `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title,omitempty"`
	Username string `json:"username,omitempty"`
}

// User represents a Telegram user or bot.
type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	Username  string `json:"username,omitempty"`
}

// CallbackQuery represents an incoming callback query from an inline keyboard.
type CallbackQuery struct {
	ID      string   `json:"id"`
	From    *User    `json:"from"`
	Message *Message `json:"message,omitempty"`
	Data    string   `json:"data,omitempty"`
}

// ChatMember represents a chat member.
type ChatMember struct {
	Status string `json:"status"`
	User   *User  `json:"user"`
}

// ChatMemberUpdated represents a change in the bot's membership status.
type ChatMemberUpdated struct {
	Chat          *Chat       `json:"chat"`
	OldChatMember *ChatMember `json:"old_chat_member"`
	NewChatMember *ChatMember `json:"new_chat_member"`
}

// APIResponse is the generic response wrapper for the Telegram Bot API.
type APIResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result,omitempty"`
	ErrorCode   int             `json:"error_code,omitempty"`
	Description string          `json:"description,omitempty"`
}
