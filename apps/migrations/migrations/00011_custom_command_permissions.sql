-- +goose Up
-- +goose StatementBegin
ALTER TABLE custom_commands
    ADD COLUMN permission TEXT NOT NULL DEFAULT 'user',
    ADD CONSTRAINT custom_commands_permission_check CHECK (permission IN ('user', 'moderator', 'owner'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE custom_commands
    DROP CONSTRAINT IF EXISTS custom_commands_permission_check,
    DROP COLUMN IF EXISTS permission;
-- +goose StatementEnd
