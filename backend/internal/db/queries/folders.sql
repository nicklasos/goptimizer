-- name: ListFolders :many
SELECT * FROM folders
ORDER BY name COLLATE NOCASE, id;

-- name: GetFolderByID :one
SELECT * FROM folders
WHERE id = ? LIMIT 1;

-- name: CreateFolder :one
INSERT INTO folders (
    name, kind, path, scan_interval_minutes, full_scan_interval_hours, profile
) VALUES (
    sqlc.arg(name), sqlc.arg(kind), sqlc.arg(path),
    sqlc.arg(scan_interval_minutes), sqlc.arg(full_scan_interval_hours), sqlc.arg(profile)
)
RETURNING *;

-- name: UpdateFolder :one
UPDATE folders
SET
    name = sqlc.arg(name),
    kind = sqlc.arg(kind),
    path = sqlc.arg(path),
    scan_interval_minutes = sqlc.arg(scan_interval_minutes),
    full_scan_interval_hours = sqlc.arg(full_scan_interval_hours),
    profile = sqlc.arg(profile),
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: SetFolderEnabled :one
UPDATE folders
SET enabled = sqlc.arg(enabled), updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: DeleteFolder :execrows
DELETE FROM folders
WHERE id = ?;
