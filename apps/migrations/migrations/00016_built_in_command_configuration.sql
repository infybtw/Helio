-- +goose Up
-- +goose StatementBegin
ALTER TABLE built_in_command_settings
    ADD COLUMN permission TEXT,
    ADD COLUMN mute_duration TEXT,
    ADD COLUMN reply_message TEXT,
    ADD CONSTRAINT built_in_command_settings_permission_check
        CHECK (permission IS NULL OR permission IN ('user', 'moderator', 'owner'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE built_in_command_settings
    DROP CONSTRAINT IF EXISTS built_in_command_settings_permission_check,
    DROP COLUMN IF EXISTS reply_message,
    DROP COLUMN IF EXISTS mute_duration,
    DROP COLUMN IF EXISTS permission;
-- +goose StatementEnd
