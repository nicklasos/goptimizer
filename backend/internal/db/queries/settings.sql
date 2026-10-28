-- name: GetSettings :one
SELECT * FROM settings
WHERE id = 1;

-- name: UpdateSettings :one
UPDATE settings
SET
    workers = sqlc.arg(workers),
    scan_workers = sqlc.arg(scan_workers),
    nice = sqlc.arg(nice),
    updated_at = CURRENT_TIMESTAMP
WHERE id = 1
RETURNING *;
