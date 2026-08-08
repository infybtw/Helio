-- +goose Up
-- +goose StatementBegin
ALTER TABLE custom_commands
    ADD COLUMN enabled BOOLEAN NOT NULL DEFAULT TRUE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE custom_commands
    DROP COLUMN IF EXISTS enabled;
-- +goose StatementEnd
