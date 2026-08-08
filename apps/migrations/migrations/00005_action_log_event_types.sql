-- +goose Up
-- +goose StatementBegin
ALTER TABLE action_logs
    ADD COLUMN event_type TEXT NOT NULL DEFAULT 'moderation',
    ADD CONSTRAINT action_logs_event_type_check CHECK (event_type IN ('custom', 'moderation', 'info'));

CREATE INDEX action_logs_chat_event_created_idx ON action_logs (chat_id, event_type, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS action_logs_chat_event_created_idx;
ALTER TABLE action_logs
    DROP CONSTRAINT IF EXISTS action_logs_event_type_check,
    DROP COLUMN IF EXISTS event_type;
-- +goose StatementEnd
