-- name: CreateUser :one
INSERT INTO users (email, password)
VALUES ($1, $2)
RETURNING id, email, role, is_blocked, created_at, updated_at;

-- CreateUserAutoRole is used by the public registration flow. The very first
-- user ever inserted becomes an admin so the system has a bootstrap operator;
-- everyone else gets the default 'user' role. The CASE-WHEN runs inside the
-- INSERT so MVCC keeps it atomic against concurrent inserts (in the
-- extraordinarily rare case of two simultaneous first-registers both seeing
-- an empty table, both would become admin — accepted edge case).
--
-- name: CreateUserAutoRole :one
INSERT INTO users (email, password, role)
VALUES (
    $1,
    $2,
    CASE WHEN NOT EXISTS (SELECT 1 FROM users) THEN 'admin' ELSE 'user' END
)
RETURNING id, email, role, is_blocked, created_at, updated_at;

-- CreateUserAsAdmin lets an existing admin provision a new account with any
-- role. Distinct from CreateUserAutoRole so the role can be specified explicitly.
--
-- name: CreateUserAsAdmin :one
INSERT INTO users (email, password, role)
VALUES ($1, $2, $3)
RETURNING id, email, role, is_blocked, created_at, updated_at;

-- name: GetUserByEmail :one
SELECT id, email, password, role, is_blocked, created_at, updated_at
FROM users
WHERE email = $1;

-- name: GetUserByID :one
SELECT id, email, role, is_blocked, created_at, updated_at
FROM users
WHERE id = $1;

-- name: UpdateUserRole :exec
UPDATE users SET role = $2, updated_at = now() WHERE id = $1;

-- name: SetUserBlocked :exec
UPDATE users SET is_blocked = $2, updated_at = now() WHERE id = $1;

-- name: UpdateUserEmail :exec
UPDATE users SET email = $2, updated_at = now() WHERE id = $1;

-- name: UpdateUserPassword :exec
UPDATE users SET password = $2, updated_at = now() WHERE id = $1;
