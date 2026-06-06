-- +goose Up
ALTER TABLE triggers ADD COLUMN reply_delay_seconds INT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE triggers DROP COLUMN reply_delay_seconds;
