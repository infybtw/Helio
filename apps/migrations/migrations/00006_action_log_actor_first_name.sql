-- +goose Up
-- +goose StatementBegin
ALTER TABLE action_logs
    ADD COLUMN actor_first_name TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE action_logs
    DROP COLUMN IF EXISTS actor_first_name;
-- +goose StatementEnd
