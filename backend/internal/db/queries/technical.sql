-- name: Healthcheck :exec
SELECT 1;

-- name: SQLiteVersion :one
SELECT sqlite_version();

-- name: TotalChanges :one
-- Rows inserted, updated or deleted on this connection since it opened. cmd/scanbench counts writes with it.
SELECT total_changes();
