-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS built_in_command_settings (
    chat_id    BIGINT      NOT NULL,
    command    TEXT        NOT NULL,
    enabled    BOOLEAN     NOT NULL DEFAULT true,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (chat_id, command),
    CONSTRAINT built_in_command_settings_chat_fk
        FOREIGN KEY (chat_id) REFERENCES tracked_chats (chat_id) ON DELETE CASCADE,
    CONSTRAINT built_in_command_settings_command_check
        CHECK (command IN ('!delete', '!mute', '!ban', '!grant', '!revoke', '!help'))
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS built_in_command_settings;
-- +goose StatementEnd
