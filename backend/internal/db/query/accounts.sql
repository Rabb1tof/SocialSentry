-- name: CreateConnectedAccount :one
INSERT INTO connected_accounts (
    user_id, platform, platform_id,
    display_name, avatar_url,
    access_token, token_expires_at,
    page_id, extra
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, user_id, platform, platform_id, display_name, avatar_url,
          access_token, token_expires_at, page_id, extra,
          is_active, status, status_message, created_at, updated_at;

-- name: GetAccountByID :one
SELECT id, user_id, platform, platform_id, display_name, avatar_url,
       access_token, token_expires_at, page_id, extra,
       is_active, status, status_message, created_at, updated_at
FROM connected_accounts
WHERE id = $1;

-- name: ListAccountsByUser :many
SELECT id, user_id, platform, platform_id, display_name, avatar_url,
       access_token, token_expires_at, page_id, extra,
       is_active, status, status_message, created_at, updated_at
FROM connected_accounts
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: CountActiveAccountsByUser :one
SELECT COUNT(*) FROM connected_accounts
WHERE user_id = $1 AND is_active = true;

-- name: ListActiveAccountsAllUsers :many
SELECT id, user_id, platform, platform_id, display_name, avatar_url,
       access_token, token_expires_at, page_id, extra,
       is_active, status, status_message, created_at, updated_at
FROM connected_accounts
WHERE is_active = true AND status != 'error';

-- name: DeleteAccount :exec
DELETE FROM connected_accounts WHERE id = $1;

-- name: SetAccountStatus :exec
UPDATE connected_accounts
SET status = $2, status_message = $3, updated_at = now()
WHERE id = $1;

-- name: SetAccountActive :exec
UPDATE connected_accounts
SET is_active = $2, status = $3, updated_at = now()
WHERE id = $1;

-- name: UpdateAccountToken :exec
UPDATE connected_accounts
SET access_token = $2, token_expires_at = $3, updated_at = now()
WHERE id = $1;

-- name: ListIGAccountsNearExpiry :many
SELECT id, user_id, platform, platform_id, display_name, avatar_url,
       access_token, token_expires_at, page_id, extra,
       is_active, status, status_message, created_at, updated_at
FROM connected_accounts
WHERE platform = 'instagram'
  AND is_active = true
  AND status != 'error'
  AND token_expires_at IS NOT NULL
  AND token_expires_at < now() + ($1::int || ' days')::interval;

-- name: GetAccountByPageID :one
SELECT id, user_id, platform, platform_id, display_name, avatar_url,
       access_token, token_expires_at, page_id, extra,
       is_active, status, status_message, created_at, updated_at
FROM connected_accounts
WHERE platform = $1 AND (page_id = $2 OR platform_id = $2)
LIMIT 1;
