-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS chat_admins (
    chat_id    BIGINT      NOT NULL,
    user_id    BIGINT      NOT NULL,
    granted_by BIGINT      NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (chat_id, user_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS chat_admins;
-- +goose StatementEnd
