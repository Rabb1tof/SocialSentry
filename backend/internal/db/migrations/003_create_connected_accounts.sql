-- +goose Up
CREATE TABLE connected_accounts (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform         TEXT NOT NULL,
    platform_id      TEXT NOT NULL,
    display_name     TEXT,
    avatar_url       TEXT,
    access_token     TEXT NOT NULL,
    token_expires_at TIMESTAMPTZ,
    page_id          TEXT,
    extra            JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_active        BOOLEAN NOT NULL DEFAULT true,
    status           TEXT NOT NULL DEFAULT 'running',
    status_message   TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, platform, platform_id)
);

-- +goose Down
DROP TABLE connected_accounts;
