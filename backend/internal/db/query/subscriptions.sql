-- name: CreateSubscription :one
INSERT INTO subscriptions (user_id, plan, is_active, starts_at, expires_at, note, granted_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, user_id, plan, is_active, starts_at, expires_at, note, granted_by, created_at;

-- name: GetActiveSubscriptionByUserID :one
SELECT id, user_id, plan, is_active, starts_at, expires_at, note, granted_by, created_at
FROM subscriptions
WHERE user_id = $1
  AND is_active = true
  AND (expires_at IS NULL OR expires_at > now())
ORDER BY created_at DESC
LIMIT 1;

-- name: GetSubscriptionByID :one
SELECT id, user_id, plan, is_active, starts_at, expires_at, note, granted_by, created_at
FROM subscriptions
WHERE id = $1;

-- name: GetSubscriptionsByUserID :many
SELECT id, user_id, plan, is_active, starts_at, expires_at, note, granted_by, created_at
FROM subscriptions
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: ListSubscriptions :many
SELECT id, user_id, plan, is_active, starts_at, expires_at, note, granted_by, created_at
FROM subscriptions
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountSubscriptions :one
SELECT COUNT(*) FROM subscriptions;

-- name: UpdateSubscription :one
UPDATE subscriptions
SET plan       = $2,
    is_active  = $3,
    expires_at = $4,
    note       = $5
WHERE id = $1
RETURNING id, user_id, plan, is_active, starts_at, expires_at, note, granted_by, created_at;

-- name: DeactivateSubscription :exec
UPDATE subscriptions SET is_active = false WHERE id = $1;

-- name: DeactivateAllUserSubscriptions :exec
UPDATE subscriptions SET is_active = false WHERE user_id = $1;
