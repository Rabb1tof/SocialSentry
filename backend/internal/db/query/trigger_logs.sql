-- name: CreateTriggerLog :one
INSERT INTO trigger_logs (
    trigger_id, account_id, event_type, platform_event_id,
    sender_id, sender_username, incoming_text, matched_keyword,
    action_taken, error_message
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id, trigger_id, account_id, event_type, platform_event_id,
          sender_id, sender_username, incoming_text, matched_keyword,
          action_taken, error_message, created_at;

-- name: ListLogsByAccount :many
SELECT id, trigger_id, account_id, event_type, platform_event_id,
       sender_id, sender_username, incoming_text, matched_keyword,
       action_taken, error_message, created_at
FROM trigger_logs
WHERE account_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountLogsByAccount :one
SELECT COUNT(*) FROM trigger_logs WHERE account_id = $1;

-- name: ListLogsByUser :many
SELECT sqlc.embed(l)
FROM trigger_logs l
JOIN connected_accounts a ON a.id = l.account_id
WHERE a.user_id = $1
ORDER BY l.created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListLogsByTrigger :many
SELECT id, trigger_id, account_id, event_type, platform_event_id,
       sender_id, sender_username, incoming_text, matched_keyword,
       action_taken, error_message, created_at
FROM trigger_logs
WHERE trigger_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: DeleteLogsOlderThan :exec
DELETE FROM trigger_logs
WHERE account_id IN (
    SELECT a.id FROM connected_accounts a
    JOIN subscriptions s ON s.user_id = a.user_id AND s.is_active = true
    WHERE s.plan = $1
)
AND created_at < now() - ($2::int || ' days')::interval;
