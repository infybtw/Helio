-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS voice_transcriptions (
    job_id                 TEXT        PRIMARY KEY,
    chat_id                BIGINT      NOT NULL REFERENCES tracked_chats (chat_id) ON DELETE CASCADE,
    message_id             BIGINT      NOT NULL,
    transcript             TEXT        NOT NULL,
    language               TEXT        NOT NULL,
    language_probability   DOUBLE PRECISION NOT NULL,
    transcription_seconds  DOUBLE PRECISION NOT NULL,
    audio_duration_seconds DOUBLE PRECISION NOT NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (chat_id, message_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS voice_transcriptions;
-- +goose StatementEnd
