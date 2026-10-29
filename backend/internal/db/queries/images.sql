-- name: ListDirs :many
SELECT rel_path, mtime_ns FROM dirs
WHERE folder_id = ?
ORDER BY rel_path;

-- name: UpsertDir :exec
INSERT INTO dirs (folder_id, rel_path, mtime_ns)
VALUES (sqlc.arg(folder_id), sqlc.arg(rel_path), sqlc.arg(mtime_ns))
ON CONFLICT (folder_id, rel_path) DO UPDATE SET mtime_ns = excluded.mtime_ns;

-- name: DeleteDirsUnder :exec
-- The directory and everything below it. '0' is the character after '/', so the range is a prefix
-- scan of the primary key; the last line drops siblings such as "a b" that sort inside it.
DELETE FROM dirs
WHERE folder_id = sqlc.arg(folder_id)
  AND rel_path >= sqlc.arg(rel_path) AND rel_path < sqlc.arg(rel_path) || '0'
  AND (rel_path = sqlc.arg(rel_path) OR rel_path >= sqlc.arg(rel_path) || '/');

-- name: ListImagesInDir :many
SELECT id, name, size, mtime_ns, status FROM images
WHERE folder_id = ? AND dir = ?;

-- name: InsertImage :exec
INSERT INTO images (folder_id, dir, name, format, size, mtime_ns, status, discovered_at, updated_at)
VALUES (
    sqlc.arg(folder_id), sqlc.arg(dir), sqlc.arg(name), sqlc.arg(format), sqlc.arg(size), sqlc.arg(mtime_ns),
    'pending', sqlc.arg(now), sqlc.arg(now)
);

-- name: MarkImagePending :exec
-- The file is new on disk again. What the optimizer recorded before stays.
UPDATE images
SET status = 'pending', size = sqlc.arg(size), mtime_ns = sqlc.arg(mtime_ns), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: MarkImageGone :exec
UPDATE images
SET status = 'gone', updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: MarkImagesGoneUnder :execrows
-- Every image in the directory and below it, with the range of DeleteDirsUnder on idx_images_path.
UPDATE images
SET status = 'gone', updated_at = sqlc.arg(now)
WHERE folder_id = sqlc.arg(folder_id)
  AND dir >= sqlc.arg(dir) AND dir < sqlc.arg(dir) || '0'
  AND (dir = sqlc.arg(dir) OR dir >= sqlc.arg(dir) || '/')
  AND status != 'gone';

-- name: CountImagesByStatus :many
SELECT folder_id, status, COUNT(*) AS count FROM images
GROUP BY folder_id, status;

-- name: CountImagesOfFolderByStatus :many
SELECT status, COUNT(*) AS count FROM images
WHERE folder_id = ?
GROUP BY status;
