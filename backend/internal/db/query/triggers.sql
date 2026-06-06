-- name: CreateTrigger :one
INSERT INTO triggers (
    account_id, name, is_active, event_type,
    match_mode, keywords, keywords_mode, case_sensitive,
    reply_to_comment, reply_comment_text,
    send_private_reply, private_reply_text,
    send_dm, dm_text,
    check_subscription, reply_if_subscribed, reply_if_unsubscribed,
    cooldown_seconds, max_replies_per_user, priority, reply_delay_seconds
)
VALUES ($1, $2, $3, $4,
        $5, $6, $7, $8,
        $9, $10,
        $11, $12,
        $13, $14,
        $15, $16, $17,
        $18, $19, $20, $21)
RETURNING id, account_id, name, is_active, event_type,
          match_mode, keywords, keywords_mode, case_sensitive,
          reply_to_comment, reply_comment_text,
          send_private_reply, private_reply_text,
          send_dm, dm_text,
          check_subscription, reply_if_subscribed, reply_if_unsubscribed,
          cooldown_seconds, max_replies_per_user, priority,
          created_at, updated_at, reply_delay_seconds;

-- name: GetTriggerByID :one
SELECT id, account_id, name, is_active, event_type,
       match_mode, keywords, keywords_mode, case_sensitive,
       reply_to_comment, reply_comment_text,
       send_private_reply, private_reply_text,
       send_dm, dm_text,
       check_subscription, reply_if_subscribed, reply_if_unsubscribed,
       cooldown_seconds, max_replies_per_user, priority,
       created_at, updated_at, reply_delay_seconds
FROM triggers
WHERE id = $1;

-- name: ListTriggersByAccount :many
SELECT id, account_id, name, is_active, event_type,
       match_mode, keywords, keywords_mode, case_sensitive,
       reply_to_comment, reply_comment_text,
       send_private_reply, private_reply_text,
       send_dm, dm_text,
       check_subscription, reply_if_subscribed, reply_if_unsubscribed,
       cooldown_seconds, max_replies_per_user, priority,
       created_at, updated_at, reply_delay_seconds
FROM triggers
WHERE account_id = $1
ORDER BY priority DESC, created_at ASC;

-- name: ListActiveTriggersByAccount :many
SELECT id, account_id, name, is_active, event_type,
       match_mode, keywords, keywords_mode, case_sensitive,
       reply_to_comment, reply_comment_text,
       send_private_reply, private_reply_text,
       send_dm, dm_text,
       check_subscription, reply_if_subscribed, reply_if_unsubscribed,
       cooldown_seconds, max_replies_per_user, priority,
       created_at, updated_at, reply_delay_seconds
FROM triggers
WHERE account_id = $1 AND is_active = true
ORDER BY priority DESC, created_at ASC;

-- name: CountTriggersByAccount :one
SELECT COUNT(*) FROM triggers WHERE account_id = $1;

-- name: UpdateTrigger :one
UPDATE triggers
SET name = $2,
    is_active = $3,
    event_type = $4,
    match_mode = $5,
    keywords = $6,
    keywords_mode = $7,
    case_sensitive = $8,
    reply_to_comment = $9,
    reply_comment_text = $10,
    send_private_reply = $11,
    private_reply_text = $12,
    send_dm = $13,
    dm_text = $14,
    check_subscription = $15,
    reply_if_subscribed = $16,
    reply_if_unsubscribed = $17,
    cooldown_seconds = $18,
    max_replies_per_user = $19,
    priority = $20,
    reply_delay_seconds = $21,
    updated_at = now()
WHERE id = $1
RETURNING id, account_id, name, is_active, event_type,
          match_mode, keywords, keywords_mode, case_sensitive,
          reply_to_comment, reply_comment_text,
          send_private_reply, private_reply_text,
          send_dm, dm_text,
          check_subscription, reply_if_subscribed, reply_if_unsubscribed,
          cooldown_seconds, max_replies_per_user, priority,
          created_at, updated_at, reply_delay_seconds;

-- name: ToggleTrigger :exec
UPDATE triggers SET is_active = $2, updated_at = now() WHERE id = $1;

-- name: DeleteTrigger :exec
DELETE FROM triggers WHERE id = $1;
