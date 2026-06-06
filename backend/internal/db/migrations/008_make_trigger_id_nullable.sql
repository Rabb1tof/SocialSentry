-- +goose Up
-- Allow trigger_logs rows that are not tied to a specific trigger.
-- These are "skipped" / ingress events (cooldown, max_replies_reached, no_action_text)
-- recorded for observability even though no trigger ultimately fired.
-- The ON DELETE CASCADE foreign key remains in force for non-NULL rows.
ALTER TABLE trigger_logs ALTER COLUMN trigger_id DROP NOT NULL;

-- +goose Down
-- Restore the NOT NULL constraint. Skipped logs (trigger_id IS NULL) cannot satisfy
-- it, so drop them first to keep the roundtrip clean.
DELETE FROM trigger_logs WHERE trigger_id IS NULL;
ALTER TABLE trigger_logs ALTER COLUMN trigger_id SET NOT NULL;
