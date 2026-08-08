-- +goose Up
-- +goose StatementBegin
ALTER TABLE custom_command_actions
    DROP CONSTRAINT IF EXISTS custom_command_actions_action_type_check,
    ADD CONSTRAINT custom_command_actions_action_type_check CHECK (action_type IN ('send_message', 'reply_message', 'mute', 'delete_message'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM custom_command_actions WHERE action_type = 'reply_message';
ALTER TABLE custom_command_actions
    DROP CONSTRAINT IF EXISTS custom_command_actions_action_type_check,
    ADD CONSTRAINT custom_command_actions_action_type_check CHECK (action_type IN ('send_message', 'mute', 'delete_message'));
-- +goose StatementEnd
