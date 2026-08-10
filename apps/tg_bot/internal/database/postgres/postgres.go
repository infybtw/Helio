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

// ListBuiltInCommandSettings returns the explicit settings for a chat. Built-in
// commands without a row are enabled by default.
func (p *Postgres) ListBuiltInCommandSettings(ctx context.Context, chatID int64) ([]database.BuiltInCommandSetting, error) {
	rows, err := p.pool.Query(ctx, `SELECT chat_id, command, enabled, COALESCE(permission, ''), COALESCE(mute_duration, ''), COALESCE(reply_message, '') FROM built_in_command_settings WHERE chat_id = $1`, chatID)
	if err != nil {
		return nil, fmt.Errorf("list built-in command settings: %w", err)
	}
	defer rows.Close()

	settings := make([]database.BuiltInCommandSetting, 0)
	for rows.Next() {
		var setting database.BuiltInCommandSetting
		if err := rows.Scan(&setting.ChatID, &setting.Command, &setting.Enabled, &setting.Permission, &setting.MuteDuration, &setting.ReplyMessage); err != nil {
			return nil, fmt.Errorf("scan built-in command setting: %w", err)
		}
		settings = append(settings, setting)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate built-in command settings: %w", err)
	}
	return settings, nil
}

// SetBuiltInCommandEnabled creates or updates a command setting for a chat.
func (p *Postgres) SetBuiltInCommandEnabled(ctx context.Context, chatID int64, command string, enabled bool) error {
	_, err := p.pool.Exec(ctx,
		`INSERT INTO built_in_command_settings (chat_id, command, enabled)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (chat_id, command) DO UPDATE SET enabled = EXCLUDED.enabled, updated_at = now()`,
		chatID, command, enabled,
	)
	if err != nil {
		return fmt.Errorf("set built-in command enabled: %w", err)
	}
	return nil
}

// UpdateBuiltInCommandSetting creates or replaces an owner's configuration for
// a built-in command.
func (p *Postgres) UpdateBuiltInCommandSetting(ctx context.Context, setting database.BuiltInCommandSetting) error {
	_, err := p.pool.Exec(ctx,
		`INSERT INTO built_in_command_settings (chat_id, command, enabled, permission, mute_duration, reply_message)
		 VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''))
		 ON CONFLICT (chat_id, command) DO UPDATE SET
		   enabled = EXCLUDED.enabled,
		   permission = EXCLUDED.permission,
		   mute_duration = EXCLUDED.mute_duration,
		   reply_message = EXCLUDED.reply_message,
		   updated_at = now()`,
		setting.ChatID, setting.Command, setting.Enabled, setting.Permission, setting.MuteDuration, setting.ReplyMessage,
	)
	if err != nil {
		return fmt.Errorf("update built-in command setting: %w", err)
	}
	return nil
}

// ResetBuiltInCommandSettings removes all explicit overrides for a chat.
func (p *Postgres) ResetBuiltInCommandSettings(ctx context.Context, chatID int64) error {
	_, err := p.pool.Exec(ctx, `DELETE FROM built_in_command_settings WHERE chat_id = $1`, chatID)
	if err != nil {
		return fmt.Errorf("reset built-in command settings: %w", err)
	}
	return nil
}

// GetBuiltInCommandSetting returns a command's explicit per-chat setting.
func (p *Postgres) GetBuiltInCommandSetting(ctx context.Context, chatID int64, command string) (database.BuiltInCommandSetting, bool, error) {
	var setting database.BuiltInCommandSetting
	err := p.pool.QueryRow(ctx,
		`SELECT chat_id, command, enabled, COALESCE(permission, ''), COALESCE(mute_duration, ''), COALESCE(reply_message, '') FROM built_in_command_settings WHERE chat_id = $1 AND command = $2`,
		chatID, command,
	).Scan(&setting.ChatID, &setting.Command, &setting.Enabled, &setting.Permission, &setting.MuteDuration, &setting.ReplyMessage)
	if err == pgx.ErrNoRows {
		return database.BuiltInCommandSetting{}, false, nil
	}
	if err != nil {
		return database.BuiltInCommandSetting{}, false, fmt.Errorf("get built-in command setting: %w", err)
	}
	return setting, true, nil
}

// IsBuiltInCommandEnabled reports whether a command is enabled for a chat.
// A missing setting preserves the default enabled behavior.
func (p *Postgres) IsBuiltInCommandEnabled(ctx context.Context, chatID int64, command string) (bool, error) {
	var enabled bool
	err := p.pool.QueryRow(ctx,
		`SELECT enabled FROM built_in_command_settings WHERE chat_id = $1 AND command = $2`,
		chatID, command,
	).Scan(&enabled)
	if err == pgx.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("get built-in command setting: %w", err)
	}
	return enabled, nil
}

// GetVoiceRecognitionSettings returns the effective voice transcription settings.
func (p *Postgres) GetVoiceRecognitionSettings(ctx context.Context, chatID int64) (database.VoiceRecognitionSettings, error) {
	settings := database.VoiceRecognitionSettings{
		ChatID: chatID, Enabled: true, Permission: "user", MaxDurationSeconds: 120,
	}
	err := p.pool.QueryRow(ctx, `
		SELECT enabled, permission, max_duration_seconds
		FROM voice_recognition_settings WHERE chat_id = $1`, chatID,
	).Scan(&settings.Enabled, &settings.Permission, &settings.MaxDurationSeconds)
	if err == pgx.ErrNoRows {
		return settings, nil
	}
	if err != nil {
		return settings, fmt.Errorf("get voice recognition settings: %w", err)
	}
	return settings, nil
}

// UpdateVoiceRecognitionSettings creates or updates a group's voice settings.
func (p *Postgres) UpdateVoiceRecognitionSettings(ctx context.Context, settings database.VoiceRecognitionSettings) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO voice_recognition_settings (chat_id, enabled, permission, max_duration_seconds)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (chat_id) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			permission = EXCLUDED.permission,
			max_duration_seconds = EXCLUDED.max_duration_seconds,
			updated_at = now()`,
		settings.ChatID, settings.Enabled, settings.Permission, settings.MaxDurationSeconds,
	)
	if err != nil {
		return fmt.Errorf("update voice recognition settings: %w", err)
	}
	return nil
}

// ClaimVoiceTranscriptionReply stores a result and atomically claims its Telegram reply.
func (p *Postgres) ClaimVoiceTranscriptionReply(ctx context.Context, transcription database.VoiceTranscription, claimToken string) (database.VoiceTranscriptionReplyClaim, error) {
	claim := database.VoiceTranscriptionReplyClaim{}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return claim, fmt.Errorf("begin voice transcription reply claim: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO voice_transcriptions
			(job_id, chat_id, message_id, transcript, language, language_probability, transcription_seconds, audio_duration_seconds)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (chat_id, message_id) DO UPDATE SET
			job_id = EXCLUDED.job_id,
			transcript = EXCLUDED.transcript,
			language = EXCLUDED.language,
			language_probability = EXCLUDED.language_probability,
			transcription_seconds = EXCLUDED.transcription_seconds,
			audio_duration_seconds = EXCLUDED.audio_duration_seconds,
			created_at = now()`,
		transcription.JobID, transcription.ChatID, transcription.MessageID, transcription.Transcript,
		transcription.Language, transcription.LanguageProbability, transcription.TranscriptionSeconds,
		transcription.AudioDurationSeconds,
	)
	if err != nil {
		return claim, fmt.Errorf("store voice transcription: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		UPDATE voice_transcriptions
		SET reply_status = 'sending', reply_claim_token = $3, reply_claimed_at = now()
		WHERE chat_id = $1 AND message_id = $2
		  AND (reply_status = 'pending' OR (reply_status = 'sending' AND reply_claimed_at < now() - interval '5 minutes'))`,
		transcription.ChatID, transcription.MessageID, claimToken,
	)
	if err != nil {
		return claim, fmt.Errorf("claim voice transcription reply: %w", err)
	}
	claim.Claimed = tag.RowsAffected() == 1
	if !claim.Claimed {
		if err := tx.QueryRow(ctx, `SELECT reply_status = 'sent' FROM voice_transcriptions WHERE chat_id = $1 AND message_id = $2`, transcription.ChatID, transcription.MessageID).Scan(&claim.Sent); err != nil {
			return claim, fmt.Errorf("read voice transcription reply status: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return claim, fmt.Errorf("commit voice transcription reply claim: %w", err)
	}
	return claim, nil
}

// MarkVoiceTranscriptionReplySent records the Telegram reply created by this claim.
func (p *Postgres) MarkVoiceTranscriptionReplySent(ctx context.Context, chatID, messageID int64, claimToken string, replyMessageID int64) error {
	tag, err := p.pool.Exec(ctx, `
		UPDATE voice_transcriptions
		SET reply_status = 'sent', reply_message_id = NULLIF($4, 0), reply_claim_token = NULL, reply_claimed_at = NULL
		WHERE chat_id = $1 AND message_id = $2 AND reply_status = 'sending' AND reply_claim_token = $3`,
		chatID, messageID, claimToken, replyMessageID,
	)
	if err != nil {
		return fmt.Errorf("mark voice transcription reply sent: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("mark voice transcription reply sent: claim no longer owned")
	}
	return nil
}

// ReleaseVoiceTranscriptionReply makes a failed Telegram reply available for retry.
func (p *Postgres) ReleaseVoiceTranscriptionReply(ctx context.Context, chatID, messageID int64, claimToken string) error {
	_, err := p.pool.Exec(ctx, `
		UPDATE voice_transcriptions
		SET reply_status = 'pending', reply_claim_token = NULL, reply_claimed_at = NULL
		WHERE chat_id = $1 AND message_id = $2 AND reply_status = 'sending' AND reply_claim_token = $3`,
		chatID, messageID, claimToken,
	)
	if err != nil {
		return fmt.Errorf("release voice transcription reply: %w", err)
	}
	return nil
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
		SELECT id, chat_id, name, enabled, permission, created_at
		FROM custom_commands WHERE chat_id = ANY($1) ORDER BY name`, chatIDs)
	if err != nil {
		return nil, fmt.Errorf("list custom commands: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var command database.CustomCommand
		var createdAt time.Time
		if err := rows.Scan(&command.ID, &command.ChatID, &command.Name, &command.Enabled, &command.Permission, &createdAt); err != nil {
			return nil, fmt.Errorf("scan custom command: %w", err)
		}
		command.CreatedAt = createdAt.Format(time.RFC3339)
		command.Actions, err = p.listCustomCommandActions(ctx, command.ID)
		if err != nil {
			return nil, err
		}
		command.Aliases, err = p.listCustomCommandAliases(ctx, command.ID)
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

func (p *Postgres) CreateCustomCommand(ctx context.Context, chatID, createdBy int64, name, permission string, aliases []string, actions []database.CustomCommandAction) (database.CustomCommand, error) {
	var command database.CustomCommand
	var createdAt time.Time
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return command, fmt.Errorf("begin custom command transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := tx.QueryRow(ctx, `
		INSERT INTO custom_commands (chat_id, name, response, created_by, permission)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, chat_id, name, enabled, permission, created_at`, chatID, name, actions[0].Payload, createdBy, permission).
		Scan(&command.ID, &command.ChatID, &command.Name, &command.Enabled, &command.Permission, &createdAt); err != nil {
		return command, fmt.Errorf("create custom command: %w", err)
	}
	if err := insertCustomCommandActions(ctx, tx, command.ID, actions); err != nil {
		return command, err
	}
	if err := insertCustomCommandAliases(ctx, tx, command.ID, chatID, aliases); err != nil {
		return command, err
	}
	if err := tx.Commit(ctx); err != nil {
		return command, fmt.Errorf("commit custom command transaction: %w", err)
	}
	command.CreatedAt = createdAt.Format(time.RFC3339)
	command.Actions = actions
	command.Aliases = aliases
	return command, nil
}

func (p *Postgres) UpdateCustomCommand(ctx context.Context, id, chatID int64, chatIDs []int64, name, permission string, aliases []string, actions []database.CustomCommandAction) (database.CustomCommand, bool, error) {
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
		UPDATE custom_commands SET chat_id = $2, name = $3, response = $4, permission = $5
		WHERE id = $1 AND chat_id = ANY($6)
		RETURNING id, chat_id, name, enabled, permission, created_at`, id, chatID, name, actions[0].Payload, permission, chatIDs).
		Scan(&command.ID, &command.ChatID, &command.Name, &command.Enabled, &command.Permission, &createdAt)
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
	if _, err := tx.Exec(ctx, `DELETE FROM custom_command_aliases WHERE command_id = $1`, id); err != nil {
		return command, false, fmt.Errorf("delete custom command aliases: %w", err)
	}
	if err := insertCustomCommandAliases(ctx, tx, id, chatID, aliases); err != nil {
		return command, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return command, false, fmt.Errorf("commit custom command transaction: %w", err)
	}
	command.CreatedAt = createdAt.Format(time.RFC3339)
	command.Actions = actions
	command.Aliases = aliases
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
		RETURNING id, chat_id, name, enabled, permission, created_at`, id, chatIDs).
		Scan(&command.ID, &command.ChatID, &command.Name, &command.Enabled, &command.Permission, &createdAt)
	if err == pgx.ErrNoRows {
		return command, false, nil
	}
	if err != nil {
		return command, false, fmt.Errorf("delete custom command: %w", err)
	}
	command.CreatedAt = createdAt.Format(time.RFC3339)
	return command, true, nil
}

func (p *Postgres) SetCustomCommandEnabled(ctx context.Context, id int64, enabled bool, chatIDs []int64) (bool, error) {
	if len(chatIDs) == 0 {
		return false, nil
	}
	tag, err := p.pool.Exec(ctx, `UPDATE custom_commands SET enabled = $2 WHERE id = $1 AND chat_id = ANY($3)`, id, enabled, chatIDs)
	if err != nil {
		return false, fmt.Errorf("set custom command enabled: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (p *Postgres) FindCustomCommand(ctx context.Context, chatID int64, name string) (database.CustomCommand, bool, error) {
	var command database.CustomCommand
	var createdAt time.Time
	err := p.pool.QueryRow(ctx, `
		SELECT id, chat_id, name, enabled, permission, created_at
		FROM custom_commands c
		WHERE c.chat_id = $1 AND c.enabled = TRUE
		  AND (c.name = $2 OR EXISTS (SELECT 1 FROM custom_command_aliases ca WHERE ca.command_id = c.id AND ca.chat_id = $1 AND ca.alias = $2))
		ORDER BY CASE WHEN c.name = $2 THEN 0 ELSE 1 END
		LIMIT 1`, chatID, name).
		Scan(&command.ID, &command.ChatID, &command.Name, &command.Enabled, &command.Permission, &createdAt)
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
	command.Aliases, err = p.listCustomCommandAliases(ctx, command.ID)
	if err != nil {
		return command, false, err
	}
	return command, true, nil
}

func (p *Postgres) listCustomCommandAliases(ctx context.Context, commandID int64) ([]string, error) {
	rows, err := p.pool.Query(ctx, `SELECT alias FROM custom_command_aliases WHERE command_id = $1 ORDER BY alias`, commandID)
	if err != nil {
		return nil, fmt.Errorf("list custom command aliases: %w", err)
	}
	defer rows.Close()
	aliases := make([]string, 0)
	for rows.Next() {
		var alias string
		if err := rows.Scan(&alias); err != nil {
			return nil, fmt.Errorf("scan custom command alias: %w", err)
		}
		aliases = append(aliases, alias)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate custom command aliases: %w", err)
	}
	return aliases, nil
}

func insertCustomCommandAliases(ctx context.Context, tx pgx.Tx, commandID, chatID int64, aliases []string) error {
	for _, alias := range aliases {
		if _, err := tx.Exec(ctx, `INSERT INTO custom_command_aliases (command_id, chat_id, alias) VALUES ($1, $2, $3)`, commandID, chatID, alias); err != nil {
			return fmt.Errorf("create custom command alias: %w", err)
		}
	}
	return nil
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
