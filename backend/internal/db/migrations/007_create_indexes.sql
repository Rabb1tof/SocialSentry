-- +goose Up
CREATE INDEX idx_triggers_account_active
    ON triggers(account_id, is_active, priority DESC);

CREATE INDEX idx_triggers_keywords
    ON triggers USING GIN(keywords);

CREATE INDEX idx_trigger_logs_account_time
    ON trigger_logs(account_id, created_at DESC);

CREATE INDEX idx_trigger_logs_trigger
    ON trigger_logs(trigger_id, created_at DESC);

CREATE INDEX idx_accounts_user_active
    ON connected_accounts(user_id, is_active);

CREATE INDEX idx_subscriptions_user_active
    ON subscriptions(user_id, is_active, expires_at);

-- +goose Down
DROP INDEX idx_subscriptions_user_active;
DROP INDEX idx_accounts_user_active;
DROP INDEX idx_trigger_logs_trigger;
DROP INDEX idx_trigger_logs_account_time;
DROP INDEX idx_triggers_keywords;
DROP INDEX idx_triggers_account_active;
