-- name: CreateRefreshToken :exec
INSERT INTO refresh_tokens (user_id, token, expires_at)
VALUES ($1, $2, $3);

-- name: GetRefreshToken :one
SELECT id, user_id, token, expires_at, created_at
FROM refresh_tokens
WHERE token = $1 AND expires_at > now();

-- name: DeleteRefreshToken :exec
DELETE FROM refresh_tokens WHERE token = $1;

-- name: DeleteAllRefreshTokensForUser :exec
DELETE FROM refresh_tokens WHERE user_id = $1;
