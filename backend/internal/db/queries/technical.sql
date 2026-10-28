-- name: Healthcheck :exec
SELECT 1;

-- name: SQLiteVersion :one
SELECT sqlite_version();
