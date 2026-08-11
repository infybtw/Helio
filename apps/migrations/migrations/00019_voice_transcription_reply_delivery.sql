-- +goose Up
-- +goose StatementBegin
ALTER TABLE voice_transcriptions
    ADD COLUMN reply_status TEXT NOT NULL DEFAULT 'pending' CHECK (reply_status IN ('pending', 'sending', 'sent')),
    ADD COLUMN reply_claim_token TEXT,
    ADD COLUMN reply_claimed_at TIMESTAMPTZ,
    ADD COLUMN reply_message_id BIGINT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE voice_transcriptions
    DROP COLUMN IF EXISTS reply_message_id,
    DROP COLUMN IF EXISTS reply_claimed_at,
    DROP COLUMN IF EXISTS reply_claim_token,
    DROP COLUMN IF EXISTS reply_status;
-- +goose StatementEnd
