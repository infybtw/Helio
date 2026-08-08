// Package postgres implements database.Store on top of PostgreSQL.
package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tg_bot/internal/database"
)

// Postgres is a PostgreSQL-backed database.Store.
type Postgres struct {
	pool *pgxpool.Pool
}

// New connects to the database and verifies the connection.
func New(ctx context.Context, dsn string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &Postgres{pool: pool}, nil
}

// Close releases the connection pool.
func (p *Postgres) Close() {
	p.pool.Close()
}

// TrackChat records a chat where the bot received a trusted Telegram update.
func (p *Postgres) TrackChat(ctx context.Context, chatID int64, chatType, title, username string) error {
	_, err := p.pool.Exec(ctx,
		`INSERT INTO tracked_chats (chat_id, chat_type, title, username, last_seen_at)
		 VALUES ($1, $2, $3, $4, now())
		 ON CONFLICT (chat_id) DO UPDATE SET
		   chat_type = EXCLUDED.chat_type,
		   title = EXCLUDED.title,
		   username = EXCLUDED.username,
		   last_seen_at = now()`,
		chatID, chatType, title, username,
	)
	if err != nil {
		return fmt.Errorf("track chat: %w", err)
	}
	return nil
}

// RecordMessage stores an incoming Telegram message, updating it when Telegram
// sends the same message again as an edit.
func (p *Postgres) RecordMessage(ctx context.Context, message database.MessageRecord) error {
	_, err := p.pool.Exec(ctx,
		`INSERT INTO message_logs
		 (chat_id, message_id, chat_type, chat_title, chat_username, sender_id, sender_username, message_text, sent_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, to_timestamp($9))
		 ON CONFLICT (chat_id, message_id) DO UPDATE SET
		   message_text = EXCLUDED.message_text,
		   sender_id = EXCLUDED.sender_id,
		   sender_username = EXCLUDED.sender_username`,
		message.ChatID, message.MessageID, message.ChatType, message.ChatTitle, message.ChatUsername,
		message.SenderID, message.SenderUsername, message.Text, message.SentAt,
	)
	if err != nil {
		return fmt.Errorf("record message: %w", err)
	}
	return nil
}

// RecordAction stores a successful bot action and its target message.
func (p *Postgres) RecordAction(ctx context.Context, action database.ActionRecord) error {
	_, err := p.pool.Exec(ctx,
		`INSERT INTO action_logs
		 (chat_id, message_id, actor_id, actor_first_name, action, event_type, target_message_id, target_user_id, target_username)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		action.ChatID, action.MessageID, action.ActorID, action.ActorFirstName, action.Action, action.EventType,
		action.TargetMessageID, action.TargetUserID, action.TargetUsername,
	)
	if err != nil {
		return fmt.Errorf("record action: %w", err)
	}
	return nil
}

// ListTrackedChatIDs returns chats in which the bot has received updates.
func (p *Postgres) ListTrackedChatIDs(ctx context.Context) ([]int64, error) {
	rows, err := p.pool.Query(ctx, `SELECT chat_id FROM tracked_chats ORDER BY last_seen_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list tracked chats: %w", err)
	}
	defer rows.Close()

	chatIDs := make([]int64, 0)
	for rows.Next() {
		var chatID int64
		if err := rows.Scan(&chatID); err != nil {
			return nil, fmt.Errorf("scan tracked chat: %w", err)
		}
		chatIDs = append(chatIDs, chatID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tracked chats: %w", err)
	}
	return chatIDs, nil
}

// IsChatAdmin reports whether the user has been granted command rights in the chat.
func (p *Postgres) IsChatAdmin(ctx context.Context, chatID, userID int64) (bool, error) {
	var exists bool
	err := p.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM chat_admins WHERE chat_id = $1 AND user_id = $2)`,
		chatID, userID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check chat admin: %w", err)
	}
	return exists, nil
}

// GrantChatAdmin grants command rights to the user in the chat.
func (p *Postgres) GrantChatAdmin(ctx context.Context, chatID, userID, grantedBy int64) error {
	_, err := p.pool.Exec(ctx,
		`INSERT INTO chat_admins (chat_id, user_id, granted_by)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (chat_id, user_id) DO NOTHING`,
		chatID, userID, grantedBy,
	)
	if err != nil {
		return fmt.Errorf("grant chat admin: %w", err)
	}
	return nil
}

// RevokeChatAdmin revokes command rights from the user in the chat.
// It reports whether a row was removed.
func (p *Postgres) RevokeChatAdmin(ctx context.Context, chatID, userID int64) (bool, error) {
	tag, err := p.pool.Exec(ctx,
		`DELETE FROM chat_admins WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID,
	)
	if err != nil {
		return false, fmt.Errorf("revoke chat admin: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// DashboardData returns chat metrics and recent actions for tracked chats.
func (p *Postgres) DashboardData(ctx context.Context, chatIDs []int64) (database.DashboardData, error) {
	data := database.DashboardData{Chats: []database.DashboardChat{}, Activity: []database.DashboardActivity{}}
	if len(chatIDs) == 0 {
		return data, nil
	}

	data.ProtectedChats = int64(len(chatIDs))
	if err := p.pool.QueryRow(ctx,
		`SELECT count(*) FROM action_logs WHERE chat_id = ANY($1) AND created_at >= now() - interval '7 days'`, chatIDs,
	).Scan(&data.ActionsThisWeek); err != nil {
		return data, fmt.Errorf("count dashboard actions: %w", err)
	}
	if err := p.pool.QueryRow(ctx,
		`SELECT count(*) FROM action_logs WHERE chat_id = ANY($1) AND action IN ('!delete', '!mute', '!ban')`, chatIDs,
	).Scan(&data.MessagesCleaned); err != nil {
		return data, fmt.Errorf("count cleaned messages: %w", err)
	}

	rows, err := p.pool.Query(ctx, `
		SELECT t.chat_id, t.title, t.username,
		       count(DISTINCT m.sender_id) FILTER (WHERE m.sender_id <> 0),
		       count(DISTINCT a.id) FILTER (WHERE a.created_at >= now() - interval '7 days')
		FROM tracked_chats t
		LEFT JOIN message_logs m ON m.chat_id = t.chat_id
		LEFT JOIN action_logs a ON a.chat_id = t.chat_id
		WHERE t.chat_id = ANY($1)
		GROUP BY t.chat_id, t.title, t.username
		ORDER BY max(t.last_seen_at) DESC`, chatIDs)
	if err != nil {
		return data, fmt.Errorf("list dashboard chats: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var chat database.DashboardChat
		var title, username string
		if err := rows.Scan(&chat.ChatID, &title, &username, &chat.Participants, &chat.Actions); err != nil {
			return data, fmt.Errorf("scan dashboard chat: %w", err)
		}
		chat.Name = title
		if chat.Name == "" {
			chat.Name = "Untitled chat"
		}
		chat.Handle = ""
		if username != "" {
			chat.Handle = "@" + strings.TrimPrefix(username, "@")
		}
		chat.Status = "Healthy"
		chat.Initials = initials(chat.Name)
		data.Chats = append(data.Chats, chat)
	}
	if err := rows.Err(); err != nil {
		return data, fmt.Errorf("iterate dashboard chats: %w", err)
	}

	activity, err := p.ListActivity(ctx, chatIDs, "", 5, 0)
	if err != nil {
		return data, err
	}
	data.Activity = activity.Items
	return data, nil
}

func (p *Postgres) ListActivity(ctx context.Context, chatIDs []int64, eventType string, limit, offset int) (database.ActivityPage, error) {
	page := database.ActivityPage{Items: []database.DashboardActivity{}}
	if len(chatIDs) == 0 {
		return page, nil
	}
	if err := p.pool.QueryRow(ctx, `
		SELECT count(*) FROM action_logs
		WHERE chat_id = ANY($1) AND ($2 = '' OR event_type = $2)`, chatIDs, eventType).Scan(&page.Total); err != nil {
		return page, fmt.Errorf("count activity: %w", err)
	}
	rows, err := p.pool.Query(ctx, `
		SELECT a.action, a.event_type, COALESCE(NULLIF(a.actor_first_name, ''), 'Unknown user'), COALESCE(NULLIF(t.title, ''), t.username, 'Unknown chat'),
		       a.target_message_id, a.created_at
		FROM action_logs a
		JOIN tracked_chats t ON t.chat_id = a.chat_id
		WHERE a.chat_id = ANY($1) AND ($2 = '' OR a.event_type = $2)
		ORDER BY a.created_at DESC LIMIT $3 OFFSET $4`, chatIDs, eventType, limit, offset)
	if err != nil {
		return page, fmt.Errorf("list activity: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item database.DashboardActivity
		var createdAt time.Time
		if err := rows.Scan(&item.Action, &item.EventType, &item.Actor, &item.Chat, &item.TargetMessageID, &createdAt); err != nil {
			return page, fmt.Errorf("scan activity: %w", err)
		}
		item.CreatedAt = createdAt.Format(time.RFC3339)
		page.Items = append(page.Items, item)
	}
	if err := rows.Err(); err != nil {
		return page, fmt.Errorf("iterate activity: %w", err)
	}
	return page, nil
}

func (p *Postgres) ListCustomCommands(ctx context.Context, chatIDs []int64) ([]database.CustomCommand, error) {
	commands := make([]database.CustomCommand, 0)
	if len(chatIDs) == 0 {
		return commands, nil
	}
	rows, err := p.pool.Query(ctx, `
		SELECT id, chat_id, name, created_at
		FROM custom_commands WHERE chat_id = ANY($1) ORDER BY name`, chatIDs)
	if err != nil {
		return nil, fmt.Errorf("list custom commands: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var command database.CustomCommand
		var createdAt time.Time
		if err := rows.Scan(&command.ID, &command.ChatID, &command.Name, &createdAt); err != nil {
			return nil, fmt.Errorf("scan custom command: %w", err)
		}
		command.CreatedAt = createdAt.Format(time.RFC3339)
		command.Actions, err = p.listCustomCommandActions(ctx, command.ID)
		if err != nil {
			return nil, err
		}
		commands = append(commands, command)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate custom commands: %w", err)
	}
	return commands, nil
}

func (p *Postgres) CreateCustomCommand(ctx context.Context, chatID, createdBy int64, name string, actions []database.CustomCommandAction) (database.CustomCommand, error) {
	var command database.CustomCommand
	var createdAt time.Time
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return command, fmt.Errorf("begin custom command transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := tx.QueryRow(ctx, `
		INSERT INTO custom_commands (chat_id, name, response, created_by)
		VALUES ($1, $2, $3, $4)
		RETURNING id, chat_id, name, created_at`, chatID, name, actions[0].Payload, createdBy).
		Scan(&command.ID, &command.ChatID, &command.Name, &createdAt); err != nil {
		return command, fmt.Errorf("create custom command: %w", err)
	}
	if err := insertCustomCommandActions(ctx, tx, command.ID, actions); err != nil {
		return command, err
	}
	if err := tx.Commit(ctx); err != nil {
		return command, fmt.Errorf("commit custom command transaction: %w", err)
	}
	command.CreatedAt = createdAt.Format(time.RFC3339)
	command.Actions = actions
	return command, nil
}

func (p *Postgres) UpdateCustomCommand(ctx context.Context, id, chatID int64, chatIDs []int64, name string, actions []database.CustomCommandAction) (database.CustomCommand, bool, error) {
	var command database.CustomCommand
	var createdAt time.Time
	if len(chatIDs) == 0 {
		return command, false, nil
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return command, false, fmt.Errorf("begin custom command transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `
		UPDATE custom_commands SET chat_id = $2, name = $3, response = $4
		WHERE id = $1 AND chat_id = ANY($5)
		RETURNING id, chat_id, name, created_at`, id, chatID, name, actions[0].Payload, chatIDs).
		Scan(&command.ID, &command.ChatID, &command.Name, &createdAt)
	if err == pgx.ErrNoRows {
		return command, false, nil
	}
	if err != nil {
		return command, false, fmt.Errorf("update custom command: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM custom_command_actions WHERE command_id = $1`, id); err != nil {
		return command, false, fmt.Errorf("delete custom command actions: %w", err)
	}
	if err := insertCustomCommandActions(ctx, tx, id, actions); err != nil {
		return command, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return command, false, fmt.Errorf("commit custom command transaction: %w", err)
	}
	command.CreatedAt = createdAt.Format(time.RFC3339)
	command.Actions = actions
	return command, true, nil
}

func (p *Postgres) DeleteCustomCommand(ctx context.Context, id int64, chatIDs []int64) (database.CustomCommand, bool, error) {
	var command database.CustomCommand
	var createdAt time.Time
	if len(chatIDs) == 0 {
		return command, false, nil
	}
	err := p.pool.QueryRow(ctx, `
		DELETE FROM custom_commands WHERE id = $1 AND chat_id = ANY($2)
		RETURNING id, chat_id, name, created_at`, id, chatIDs).
		Scan(&command.ID, &command.ChatID, &command.Name, &createdAt)
	if err == pgx.ErrNoRows {
		return command, false, nil
	}
	if err != nil {
		return command, false, fmt.Errorf("delete custom command: %w", err)
	}
	command.CreatedAt = createdAt.Format(time.RFC3339)
	return command, true, nil
}

func (p *Postgres) FindCustomCommand(ctx context.Context, chatID int64, name string) (database.CustomCommand, bool, error) {
	var command database.CustomCommand
	var createdAt time.Time
	err := p.pool.QueryRow(ctx, `
		SELECT id, chat_id, name, created_at
		FROM custom_commands WHERE chat_id = $1 AND name = $2`, chatID, name).
		Scan(&command.ID, &command.ChatID, &command.Name, &createdAt)
	if err == pgx.ErrNoRows {
		return command, false, nil
	}
	if err != nil {
		return command, false, fmt.Errorf("find custom command: %w", err)
	}
	command.CreatedAt = createdAt.Format(time.RFC3339)
	command.Actions, err = p.listCustomCommandActions(ctx, command.ID)
	if err != nil {
		return command, false, err
	}
	return command, true, nil
}

func (p *Postgres) listCustomCommandActions(ctx context.Context, commandID int64) ([]database.CustomCommandAction, error) {
	rows, err := p.pool.Query(ctx, `SELECT action_type, payload FROM custom_command_actions WHERE command_id = $1 ORDER BY position`, commandID)
	if err != nil {
		return nil, fmt.Errorf("list custom command actions: %w", err)
	}
	defer rows.Close()
	actions := make([]database.CustomCommandAction, 0)
	for rows.Next() {
		var action database.CustomCommandAction
		if err := rows.Scan(&action.Type, &action.Payload); err != nil {
			return nil, fmt.Errorf("scan custom command action: %w", err)
		}
		actions = append(actions, action)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate custom command actions: %w", err)
	}
	return actions, nil
}

func insertCustomCommandActions(ctx context.Context, tx pgx.Tx, commandID int64, actions []database.CustomCommandAction) error {
	for position, action := range actions {
		if _, err := tx.Exec(ctx, `INSERT INTO custom_command_actions (command_id, action_type, payload, position) VALUES ($1, $2, $3, $4)`, commandID, action.Type, action.Payload, position); err != nil {
			return fmt.Errorf("create custom command action: %w", err)
		}
	}
	return nil
}

func initials(name string) string {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "??"
	}
	result := parts[0][:1]
	if len(parts) > 1 {
		result += parts[1][:1]
	}
	return strings.ToUpper(result)
}
