-- +goose Up
-- +goose StatementBegin

-- Every dashboard-owned record must belong to a tracked Telegram group.
ALTER TABLE tracked_chats
    ADD CONSTRAINT tracked_chats_id_unique UNIQUE (chat_id);

ALTER TABLE custom_commands
    ADD CONSTRAINT custom_commands_chat_fk
        FOREIGN KEY (chat_id) REFERENCES tracked_chats (chat_id) ON DELETE CASCADE;

ALTER TABLE custom_commands
    ADD CONSTRAINT custom_commands_id_chat_unique UNIQUE (id, chat_id);

ALTER TABLE custom_command_aliases
    ADD CONSTRAINT custom_command_aliases_chat_fk
        FOREIGN KEY (command_id, chat_id)
        REFERENCES custom_commands (id, chat_id) ON DELETE CASCADE;

ALTER TABLE action_logs
    ADD CONSTRAINT action_logs_chat_fk
        FOREIGN KEY (chat_id) REFERENCES tracked_chats (chat_id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS action_logs_chat_event_created_idx
    ON action_logs (chat_id, event_type, created_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS action_logs_chat_event_created_idx;
ALTER TABLE action_logs DROP CONSTRAINT IF EXISTS action_logs_chat_fk;
ALTER TABLE custom_command_aliases DROP CONSTRAINT IF EXISTS custom_command_aliases_chat_fk;
ALTER TABLE custom_commands DROP CONSTRAINT IF EXISTS custom_commands_id_chat_unique;
ALTER TABLE custom_commands DROP CONSTRAINT IF EXISTS custom_commands_chat_fk;
ALTER TABLE tracked_chats DROP CONSTRAINT IF EXISTS tracked_chats_id_unique;
-- +goose StatementEnd
