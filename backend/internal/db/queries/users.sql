-- name: GetUserByID :one
SELECT * FROM users
WHERE id = ? LIMIT 1;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE LOWER(email) = LOWER(sqlc.arg(email)) LIMIT 1;

-- name: CreateUser :one
INSERT INTO users (
    email, name, password, is_admin, email_verified_at
) VALUES (
    sqlc.arg(email), sqlc.arg(name), sqlc.arg(password), sqlc.arg(is_admin),
    CASE WHEN CAST(sqlc.arg(email_verified) AS BOOLEAN) THEN CURRENT_TIMESTAMP END
)
RETURNING *;

-- Newest first, so an account that was just created is on the first page.
-- name: ListUsers :many
SELECT * FROM users
ORDER BY id DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountUsers :one
SELECT count(*) FROM users;

-- name: CountAdmins :one
SELECT count(*) FROM users WHERE is_admin;

-- name: SetUserAdmin :exec
UPDATE users
SET is_admin = sqlc.arg(is_admin), updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);

-- A changed email is unverified again unless keep_verified is set (an admin vouches for it).
-- name: UpdateUserProfile :one
UPDATE users
SET
    email_verified_at = CASE
        WHEN email = sqlc.arg(email) OR CAST(sqlc.arg(keep_verified) AS BOOLEAN) THEN email_verified_at
    END,
    email = sqlc.arg(email),
    name = sqlc.arg(name),
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: MarkUserEmailVerified :exec
UPDATE users
SET email_verified_at = COALESCE(email_verified_at, CURRENT_TIMESTAMP)
WHERE id = ?;

-- name: UpdateUserPassword :exec
UPDATE users
SET
    password = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = ?;

-- Refresh Token Queries. The token column holds a SHA-256 hash, never the token itself.
-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (
    user_id, token, expires_at
) VALUES (
    ?, ?, ?
)
RETURNING *;

-- datetime() reads both the format Go writes and the one CURRENT_TIMESTAMP writes;
-- comparing the stored text directly would not.
-- name: GetRefreshToken :one
SELECT * FROM refresh_tokens
WHERE token = ? AND datetime(expires_at) > datetime('now') AND is_revoked = FALSE
LIMIT 1;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens
SET is_revoked = TRUE
WHERE token = ?;

-- name: RevokeAllUserRefreshTokens :exec
UPDATE refresh_tokens
SET is_revoked = TRUE
WHERE user_id = ?;

-- name: DeleteExpiredRefreshTokens :exec
DELETE FROM refresh_tokens
WHERE datetime(expires_at) < datetime('now');

-- Auth Token Queries (password reset, email verification)
-- name: CreateAuthToken :exec
INSERT INTO auth_tokens (
    user_id, purpose, token_hash, expires_at
) VALUES (
    sqlc.arg(user_id), sqlc.arg(purpose), sqlc.arg(token_hash),
    datetime('now', CAST(sqlc.arg(ttl_seconds) AS INTEGER) || ' seconds')
);

-- Marks the token used and returns its owner in one statement, so a link works exactly once.
-- name: ConsumeAuthToken :one
UPDATE auth_tokens
SET used_at = CURRENT_TIMESTAMP
WHERE token_hash = sqlc.arg(token_hash)
  AND purpose = sqlc.arg(purpose)
  AND used_at IS NULL
  AND datetime(expires_at) > datetime('now')
RETURNING user_id;

-- name: DeleteUserAuthTokens :exec
DELETE FROM auth_tokens
WHERE user_id = sqlc.arg(user_id) AND purpose = sqlc.arg(purpose);

-- name: DeleteExpiredAuthTokens :exec
DELETE FROM auth_tokens
WHERE datetime(expires_at) < datetime('now') OR used_at IS NOT NULL;
