-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS voice_recognition_settings (
    chat_id              BIGINT      PRIMARY KEY REFERENCES tracked_chats (chat_id) ON DELETE CASCADE,
    enabled              BOOLEAN     NOT NULL DEFAULT true,
    permission           TEXT        NOT NULL DEFAULT 'user' CHECK (permission IN ('user', 'moderator')),
    max_duration_seconds INTEGER     NOT NULL DEFAULT 120 CHECK (max_duration_seconds > 0 AND max_duration_seconds <= 3600),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS voice_recognition_settings;
-- +goose StatementEnd
