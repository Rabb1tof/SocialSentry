-- +goose Up
CREATE TABLE trigger_logs (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trigger_id        UUID NOT NULL REFERENCES triggers(id) ON DELETE CASCADE,
    account_id        UUID NOT NULL REFERENCES connected_accounts(id) ON DELETE CASCADE,
    event_type        TEXT NOT NULL,
    platform_event_id TEXT,
    sender_id         TEXT NOT NULL,
    sender_username   TEXT,
    incoming_text     TEXT,
    matched_keyword   TEXT,
    action_taken      TEXT NOT NULL,
    error_message     TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE trigger_logs;
