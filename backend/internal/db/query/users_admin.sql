-- name: ListUsers :many
SELECT id, email, role, is_blocked, created_at, updated_at
FROM users
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: GetStats :one
SELECT
    (SELECT COUNT(*) FROM users)              AS total_users,
    (SELECT COUNT(*) FROM subscriptions WHERE is_active = true AND (expires_at IS NULL OR expires_at > now())) AS active_subscriptions,
    (SELECT COUNT(*) FROM connected_accounts WHERE is_active = true) AS active_accounts;
