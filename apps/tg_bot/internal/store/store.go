// Package store provides database access for the bot.
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store wraps a pgx connection pool.
type Store struct {
	pool *pgxpool.Pool
}

// New connects to the database and verifies the connection.
func New(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &Store{pool: pool}, nil
}

// Close releases the connection pool.
func (s *Store) Close() {
	s.pool.Close()
}

// IsChatAdmin reports whether the user has been granted command rights in the chat.
func (s *Store) IsChatAdmin(ctx context.Context, chatID, userID int64) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM chat_admins WHERE chat_id = $1 AND user_id = $2)`,
		chatID, userID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check chat admin: %w", err)
	}
	return exists, nil
}

// GrantChatAdmin grants command rights to the user in the chat.
func (s *Store) GrantChatAdmin(ctx context.Context, chatID, userID, grantedBy int64) error {
	_, err := s.pool.Exec(ctx,
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
func (s *Store) RevokeChatAdmin(ctx context.Context, chatID, userID int64) (bool, error) {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM chat_admins WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID,
	)
	if err != nil {
		return false, fmt.Errorf("revoke chat admin: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}
