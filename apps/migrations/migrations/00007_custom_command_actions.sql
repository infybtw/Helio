-- +goose Up
-- +goose StatementBegin
CREATE TABLE custom_command_actions (
    id         BIGSERIAL    PRIMARY KEY,
    command_id BIGINT       NOT NULL REFERENCES custom_commands(id) ON DELETE CASCADE,
    action_type TEXT        NOT NULL CHECK (action_type IN ('send_message')),
    payload    TEXT         NOT NULL,
    position   INTEGER      NOT NULL CHECK (position >= 0),
    UNIQUE (command_id, position)
);

INSERT INTO custom_command_actions (command_id, action_type, payload, position)
SELECT id, 'send_message', response, 0
FROM custom_commands;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS custom_command_actions;
-- +goose StatementEnd
