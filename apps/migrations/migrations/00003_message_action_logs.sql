-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS message_logs (
    id              BIGSERIAL    PRIMARY KEY,
    chat_id         BIGINT       NOT NULL,
    message_id      BIGINT       NOT NULL,
    chat_type       TEXT         NOT NULL DEFAULT '',
    chat_title      TEXT         NOT NULL DEFAULT '',
    chat_username   TEXT         NOT NULL DEFAULT '',
    sender_id       BIGINT       NOT NULL DEFAULT 0,
    sender_username TEXT         NOT NULL DEFAULT '',
    message_text    TEXT         NOT NULL DEFAULT '',
    sent_at         TIMESTAMPTZ  NOT NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (chat_id, message_id)
);

CREATE TABLE IF NOT EXISTS action_logs (
    id                BIGSERIAL    PRIMARY KEY,
    chat_id           BIGINT       NOT NULL,
    message_id        BIGINT       NOT NULL,
    actor_id          BIGINT       NOT NULL DEFAULT 0,
    actor_username    TEXT         NOT NULL DEFAULT '',
    action            TEXT         NOT NULL,
    target_message_id BIGINT       NOT NULL DEFAULT 0,
    target_user_id    BIGINT       NOT NULL DEFAULT 0,
    target_username   TEXT         NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS action_logs_chat_created_idx ON action_logs (chat_id, created_at DESC);
CREATE INDEX IF NOT EXISTS action_logs_created_idx ON action_logs (created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS action_logs;
DROP TABLE IF EXISTS message_logs;
-- +goose StatementEnd
