-- +goose Up
CREATE TABLE triggers (
    id                        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id                UUID NOT NULL REFERENCES connected_accounts(id) ON DELETE CASCADE,
    name                      TEXT NOT NULL,
    is_active                 BOOLEAN NOT NULL DEFAULT true,
    event_type                TEXT NOT NULL,
    match_mode                TEXT NOT NULL DEFAULT 'keyword',
    keywords                  TEXT[] NOT NULL DEFAULT '{}',
    keywords_mode             TEXT NOT NULL DEFAULT 'contains',
    case_sensitive            BOOLEAN NOT NULL DEFAULT false,
    reply_to_comment          BOOLEAN NOT NULL DEFAULT true,
    reply_comment_text        TEXT,
    send_private_reply        BOOLEAN NOT NULL DEFAULT false,
    private_reply_text        TEXT,
    send_dm                   BOOLEAN NOT NULL DEFAULT false,
    dm_text                   TEXT,
    check_subscription        BOOLEAN NOT NULL DEFAULT false,
    reply_if_subscribed       TEXT,
    reply_if_unsubscribed     TEXT,
    cooldown_seconds          INT NOT NULL DEFAULT 0,
    max_replies_per_user      INT NOT NULL DEFAULT 0,
    priority                  INT NOT NULL DEFAULT 0,
    created_at                TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE triggers;
