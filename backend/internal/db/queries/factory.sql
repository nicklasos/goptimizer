-- These queries exist for internal/factory, which writes test and seed data with columns the
-- API sets itself (a verified email). Application code uses the other files.

-- name: FactoryCreateUser :one
INSERT INTO users (email, name, password, roles, email_verified_at)
VALUES (
    sqlc.arg(email), sqlc.arg(name), sqlc.arg(password), sqlc.arg(roles),
    CASE WHEN CAST(sqlc.arg(verified) AS BOOLEAN) THEN CURRENT_TIMESTAMP END
)
RETURNING *;
