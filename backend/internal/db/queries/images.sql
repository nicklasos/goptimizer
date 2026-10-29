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
-- The file is new on disk again. What the optimizer recorded before stays, but a new file gets its
-- attempts back, and the reason the old one was left alone no longer applies. The scan writes only
-- when the row is still as it read it, so a stale write never undoes a swap the optimizer recorded
-- since, and a row being optimized is never touched.
UPDATE images
SET status = 'pending', size = sqlc.arg(size), mtime_ns = sqlc.arg(mtime_ns), attempts = 0, next_attempt_at = NULL,
    reason = NULL, updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id)
  AND status = sqlc.arg(read_status) AND size = sqlc.arg(read_size) AND mtime_ns = sqlc.arg(read_mtime_ns)
  AND status != 'optimizing';

-- name: MarkImageGone :exec
-- Compare-and-set on the row as the scan read it, as MarkImagePending.
UPDATE images
SET status = 'gone', updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id)
  AND status = sqlc.arg(read_status) AND size = sqlc.arg(read_size) AND mtime_ns = sqlc.arg(read_mtime_ns)
  AND status != 'optimizing';

-- name: MarkImagesGoneUnder :execrows
-- Every image in the directory and below it, with the range of DeleteDirsUnder on idx_images_path.
-- An image being optimized is left to the optimizer, which finds the file missing.
UPDATE images
SET status = 'gone', updated_at = sqlc.arg(now)
WHERE folder_id = sqlc.arg(folder_id)
  AND dir >= sqlc.arg(dir) AND dir < sqlc.arg(dir) || '0'
  AND (dir = sqlc.arg(dir) OR dir >= sqlc.arg(dir) || '/')
  AND status NOT IN ('gone', 'optimizing');

-- name: CountImagesByStatus :many
SELECT folder_id, status, COUNT(*) AS count FROM images
GROUP BY folder_id, status;

-- name: CountImagesOfFolderByStatus :many
SELECT status, COUNT(*) AS count FROM images
WHERE folder_id = ?
GROUP BY status;

-- name: CountOptimizingByFolder :many
-- Served by idx_images_optimizing, which holds only the images in flight.
SELECT folder_id, COUNT(*) AS count FROM images
WHERE status = 'optimizing'
GROUP BY folder_id;

-- name: OldestClaimableImage :one
-- The pending image of one folder and format with the oldest mtime that is at most cutoff: one
-- seek on idx_images_claim, however many images the folder has.
SELECT id, mtime_ns FROM images
WHERE folder_id = sqlc.arg(folder_id) AND status = 'pending' AND format = sqlc.arg(format) AND mtime_ns <= sqlc.arg(cutoff)
ORDER BY mtime_ns
LIMIT 1;

-- name: ClaimImage :one
UPDATE images
SET status = 'optimizing', updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'pending'
RETURNING *;

-- name: RequeueDueImages :execrows
-- A failed image whose retry time has come is pending again. idx_images_retry holds only those waiting.
UPDATE images
SET status = 'pending', next_attempt_at = NULL, updated_at = sqlc.arg(now)
WHERE status = 'failed' AND next_attempt_at IS NOT NULL AND datetime(next_attempt_at) <= datetime(sqlc.arg(now));

-- name: RecoverOptimizingImages :execrows
-- A stop records its images as pending, so an image still optimizing at the next start was being
-- worked on when the process died. That counts as an attempt: an image that kills the process
-- every time, such as by running it out of memory, ends failed instead of being claimed first
-- after every restart.
UPDATE images
SET status = CASE WHEN attempts + 1 >= sqlc.arg(max_attempts) THEN 'failed' ELSE 'pending' END,
    attempts = attempts + 1, reason = sqlc.arg(reason), next_attempt_at = NULL, updated_at = sqlc.arg(now)
WHERE status = 'optimizing';

-- name: ReleaseImage :execrows
-- Back to pending with the file as it is now: it changed, is too new, or the work was interrupted.
UPDATE images
SET status = 'pending', size = sqlc.arg(size), mtime_ns = sqlc.arg(mtime_ns), reason = sqlc.arg(reason), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'optimizing';

-- name: LeaveImage :execrows
-- Kept, skipped or gone: the file was not touched. size and mtime_ns are the file as it was read.
UPDATE images
SET status = sqlc.arg(status), reason = sqlc.arg(reason), size = sqlc.arg(size), mtime_ns = sqlc.arg(mtime_ns), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'optimizing';

-- name: FailImage :execrows
UPDATE images
SET status = 'failed', reason = sqlc.arg(reason), attempts = sqlc.arg(attempts), next_attempt_at = sqlc.arg(next_attempt_at),
    size = sqlc.arg(size), mtime_ns = sqlc.arg(mtime_ns), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'optimizing';

-- name: SetImageOutputHash :execrows
-- Written before the rename, so a crash between the rename and RecordOptimized still leaves the
-- file recognizable as the optimizer's own output.
UPDATE images
SET output_sha256 = sqlc.arg(output_sha256), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'optimizing';

-- name: RecordOptimized :execrows
UPDATE images
SET status = 'optimized', updated_at = sqlc.arg(now), optimized_at = sqlc.arg(now), size = sqlc.arg(size),
    mtime_ns = sqlc.arg(mtime_ns), original_size = sqlc.arg(original_size), optimized_size = sqlc.arg(size),
    mode = sqlc.arg(mode), output_sha256 = sqlc.arg(output_sha256), reason = NULL, attempts = 0, next_attempt_at = NULL
WHERE id = sqlc.arg(id) AND status = 'optimizing';

-- name: RestoreOptimized :execrows
-- The file is still the optimizer's output, only touched: optimized again, with the new mtime.
UPDATE images
SET status = 'optimized', size = sqlc.arg(size), mtime_ns = sqlc.arg(mtime_ns), reason = NULL, updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'optimizing';

-- name: AddOptimizerOutput :exec
INSERT OR IGNORE INTO optimizer_outputs (sha256) VALUES (?);

-- name: IsOptimizerOutput :one
SELECT EXISTS (SELECT 1 FROM optimizer_outputs WHERE sha256 = ?) AS found;

-- name: AddDailyStat :exec
INSERT INTO daily_stats (folder_id, day, optimized, bytes_before, bytes_after)
VALUES (sqlc.arg(folder_id), sqlc.arg(day), 1, sqlc.arg(bytes_before), sqlc.arg(bytes_after))
ON CONFLICT (folder_id, day) DO UPDATE SET
    optimized = optimized + 1,
    bytes_before = bytes_before + excluded.bytes_before,
    bytes_after = bytes_after + excluded.bytes_after;

-- name: SumBytesSavedByFolder :many
SELECT folder_id, CAST(COALESCE(SUM(bytes_before - bytes_after), 0) AS INTEGER) AS saved FROM daily_stats
GROUP BY folder_id;

-- name: SumBytesSavedOfFolder :one
SELECT CAST(COALESCE(SUM(bytes_before - bytes_after), 0) AS INTEGER) AS saved FROM daily_stats
WHERE folder_id = ?;

-- name: GetImage :one
SELECT * FROM images WHERE id = ?;

-- name: ListImagesByUpdated :many
-- One keyset page of a folder's images, the latest change first: a range scan on idx_images_updated
-- from the row after the cursor.
SELECT * FROM images
WHERE folder_id = sqlc.arg(folder_id) AND (updated_at, id) < (sqlc.arg(updated_at), CAST(sqlc.arg(id) AS INTEGER))
ORDER BY updated_at DESC, id DESC
LIMIT sqlc.arg(page_limit);

-- name: ListImagesOfStatusByUpdated :many
SELECT * FROM images
WHERE folder_id = sqlc.arg(folder_id) AND status = sqlc.arg(status) AND (updated_at, id) < (sqlc.arg(updated_at), CAST(sqlc.arg(id) AS INTEGER))
ORDER BY updated_at DESC, id DESC
LIMIT sqlc.arg(page_limit);

-- name: ListOptimizedImagesBySaved :many
-- The status is a literal so that idx_images_saved, which holds only optimized images, applies.
-- SQLite seeks that index on the <= term, which neither a row value on an expression nor a CAST
-- of the parameter gives it; the OR then drops the rows of the same saving up to the cursor.
SELECT * FROM images
WHERE folder_id = sqlc.arg(folder_id) AND status = 'optimized'
  AND original_size - optimized_size <= sqlc.arg(saved)
  AND (original_size - optimized_size < sqlc.arg(saved) OR id < sqlc.arg(id))
ORDER BY original_size - optimized_size DESC, id DESC
LIMIT sqlc.arg(page_limit);

-- name: SearchImages :many
-- Paths that contain a text, ignoring ASCII case, in path order. The match reads only dir and name,
-- which idx_images_path holds, so a row is read only for a path that matches: one pass over the
-- index at worst, not over the table. needle is lower case.
SELECT * FROM images
WHERE folder_id = sqlc.arg(folder_id) AND (dir, name) > (sqlc.arg(dir), sqlc.arg(name))
  AND instr(lower(dir || '/' || name), sqlc.arg(needle)) > 0
ORDER BY dir, name
LIMIT sqlc.arg(page_limit);

-- name: SearchImagesOfStatus :many
SELECT * FROM images
WHERE folder_id = sqlc.arg(folder_id) AND status = sqlc.arg(status) AND (dir, name) > (sqlc.arg(dir), sqlc.arg(name))
  AND instr(lower(dir || '/' || name), sqlc.arg(needle)) > 0
ORDER BY dir, name
LIMIT sqlc.arg(page_limit);

-- name: RetryImage :execrows
-- A failed image is pending again with its attempts back, as for a new file. Its reason stays
-- until the next attempt replaces it, as when a retry comes due on its own.
UPDATE images
SET status = 'pending', attempts = 0, next_attempt_at = NULL, updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'failed';

-- name: RetryFailedImages :execrows
UPDATE images
SET status = 'pending', attempts = 0, next_attempt_at = NULL, updated_at = sqlc.arg(now)
WHERE folder_id = sqlc.arg(folder_id) AND status = 'failed';

-- name: ListDailySavings :many
-- Every folder's optimizations per day, from a day on. Days with none have no row.
SELECT day, CAST(SUM(optimized) AS INTEGER) AS optimized, CAST(SUM(bytes_before - bytes_after) AS INTEGER) AS saved
FROM daily_stats
WHERE day >= sqlc.arg(since)
GROUP BY day
ORDER BY day;

-- name: ListDailySavingsOfFolder :many
SELECT day, optimized, CAST(bytes_before - bytes_after AS INTEGER) AS saved
FROM daily_stats
WHERE folder_id = sqlc.arg(folder_id) AND day >= sqlc.arg(since)
ORDER BY day;
