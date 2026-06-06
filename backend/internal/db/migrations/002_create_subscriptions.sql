-- +goose Up
CREATE TABLE subscriptions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    plan        TEXT NOT NULL DEFAULT 'basic',
    is_active   BOOLEAN NOT NULL DEFAULT true,
    starts_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ,
    note        TEXT,
    granted_by  UUID REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE subscriptions;
