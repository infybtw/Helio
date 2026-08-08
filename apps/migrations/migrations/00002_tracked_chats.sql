-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS tracked_chats (
    chat_id       BIGINT       PRIMARY KEY,
    chat_type     TEXT         NOT NULL,
    title         TEXT         NOT NULL DEFAULT '',
    username      TEXT         NOT NULL DEFAULT '',
    last_seen_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tracked_chats;
-- +goose StatementEnd
