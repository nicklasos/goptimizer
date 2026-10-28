-- These queries exist for internal/factory, which writes test and seed data with columns the
-- API sets itself (a verified email). Application code uses the other files.

-- name: FactoryCreateUser :one
INSERT INTO users (email, name, password, is_admin, email_verified_at)
VALUES (
    sqlc.arg(email), sqlc.arg(name), sqlc.arg(password), sqlc.arg(is_admin),
    CASE WHEN CAST(sqlc.arg(verified) AS BOOLEAN) THEN CURRENT_TIMESTAMP END
)
RETURNING *;

-- name: FactoryCreateFolder :one
INSERT INTO folders (name, kind, path, enabled, scan_interval_minutes, full_scan_interval_hours, profile)
VALUES (
    sqlc.arg(name), sqlc.arg(kind), sqlc.arg(path), sqlc.arg(enabled),
    sqlc.arg(scan_interval_minutes), sqlc.arg(full_scan_interval_hours), sqlc.arg(profile)
)
RETURNING *;
