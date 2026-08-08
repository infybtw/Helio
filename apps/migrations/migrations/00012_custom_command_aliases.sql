-- +goose Up
-- +goose StatementBegin
CREATE TABLE custom_command_aliases (
    id         BIGSERIAL PRIMARY KEY,
    command_id BIGINT NOT NULL REFERENCES custom_commands(id) ON DELETE CASCADE,
    chat_id    BIGINT NOT NULL,
    alias      TEXT NOT NULL,
    UNIQUE (chat_id, alias),
    UNIQUE (command_id, alias)
);

CREATE INDEX custom_command_aliases_lookup_idx ON custom_command_aliases (chat_id, alias);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS custom_command_aliases;
-- +goose StatementEnd
