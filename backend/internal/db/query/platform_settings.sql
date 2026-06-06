-- name: ListPlatformSettings :many
SELECT platform, enabled, updated_at
FROM platform_settings
ORDER BY platform;

-- name: GetPlatformSetting :one
SELECT platform, enabled, updated_at
FROM platform_settings
WHERE platform = $1;

-- name: SetPlatformEnabled :one
UPDATE platform_settings
SET enabled = $2, updated_at = now()
WHERE platform = $1
RETURNING platform, enabled, updated_at;
