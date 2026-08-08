// Package postgres implements database.Store on top of PostgreSQL.
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
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
