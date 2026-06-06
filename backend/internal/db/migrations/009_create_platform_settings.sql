-- +goose Up
-- Global per-platform kill-switch toggled by admins. One row per platform; absence of a
-- row is treated by the application as "enabled" (defensive default), but both rows are
-- seeded here so the admin UI always has something to display.
CREATE TABLE platform_settings (
    platform   TEXT PRIMARY KEY,            -- 'instagram' | 'vk'
    enabled    BOOLEAN NOT NULL DEFAULT true,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO platform_settings (platform, enabled) VALUES
    ('instagram', true),
    ('vk', true);

-- +goose Down
DROP TABLE platform_settings;
