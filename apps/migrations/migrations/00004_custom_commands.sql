-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS custom_commands (
    id         BIGSERIAL    PRIMARY KEY,
    chat_id    BIGINT       NOT NULL,
    name       TEXT         NOT NULL,
    response   TEXT         NOT NULL,
    created_by BIGINT       NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (chat_id, name)
);

CREATE INDEX IF NOT EXISTS custom_commands_chat_idx ON custom_commands (chat_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS custom_commands;
-- +goose StatementEnd
