-- name: EnqueueScan :one
-- idx_scans_active refuses a second queued or running scan of the same folder.
INSERT INTO scans (folder_id, kind, trigger, queued_at)
VALUES (sqlc.arg(folder_id), sqlc.arg(kind), sqlc.arg(trigger), sqlc.arg(queued_at))
RETURNING *;

-- name: ClaimNextScan :one
-- The oldest queued scan becomes running in one statement, so two workers never claim the same one.
-- The IN term repeats idx_scans_active's WHERE, which lets SQLite read only the active scans.
UPDATE scans
SET status = 'running', started_at = sqlc.arg(started_at)
WHERE id = (
    SELECT queued.id FROM scans AS queued
    WHERE queued.status IN ('queued', 'running') AND queued.status = 'queued'
    ORDER BY queued.queued_at, queued.id
    LIMIT 1
  )
  AND status = 'queued'
RETURNING *;

-- name: UpdateScanProgress :exec
UPDATE scans
SET dirs_seen = sqlc.arg(dirs_seen), dirs_listed = sqlc.arg(dirs_listed), files_seen = sqlc.arg(files_seen),
    files_new = sqlc.arg(files_new), files_changed = sqlc.arg(files_changed), files_gone = sqlc.arg(files_gone)
WHERE id = sqlc.arg(id) AND status = 'running';

-- name: FinishScan :one
UPDATE scans
SET status = sqlc.arg(status), finished_at = sqlc.arg(finished_at), error = sqlc.arg(error),
    dirs_seen = sqlc.arg(dirs_seen), dirs_listed = sqlc.arg(dirs_listed), files_seen = sqlc.arg(files_seen),
    files_new = sqlc.arg(files_new), files_changed = sqlc.arg(files_changed), files_gone = sqlc.arg(files_gone)
WHERE id = sqlc.arg(id) AND status = 'running'
RETURNING folder_id;

-- name: PruneScans :execrows
-- Every scan of the folder older than its newest keep, except its latest successful full scan,
-- which Due reads to space the full scans out. NULL from either subquery deletes nothing.
DELETE FROM scans
WHERE scans.folder_id = sqlc.arg(folder_id)
  AND scans.id <= (
    SELECT kept.id FROM scans AS kept WHERE kept.folder_id = sqlc.arg(folder_id)
    ORDER BY kept.id DESC LIMIT 1 OFFSET sqlc.arg(keep)
  )
  AND scans.id IS NOT (
    SELECT last_full.id FROM scans AS last_full
    WHERE last_full.folder_id = sqlc.arg(folder_id) AND last_full.kind = 'full' AND last_full.status = 'succeeded'
    ORDER BY last_full.id DESC LIMIT 1
  );

-- name: FailRunningScans :execrows
UPDATE scans
SET status = 'failed', finished_at = sqlc.arg(finished_at), error = sqlc.arg(error)
WHERE status IN ('queued', 'running') AND status = 'running';

-- name: ListScansOfFolder :many
SELECT * FROM scans
WHERE folder_id = ?
ORDER BY id DESC
LIMIT ? OFFSET ?;

-- name: CountScansOfFolder :one
SELECT COUNT(*) FROM scans WHERE folder_id = ?;

-- name: GetLatestScanOfFolder :one
SELECT * FROM scans
WHERE folder_id = ?
ORDER BY id DESC
LIMIT 1;

-- name: ListLatestScans :many
-- One index lookup per folder, not a pass over every scan.
SELECT scans.* FROM folders
JOIN scans ON scans.id = (SELECT latest.id FROM scans AS latest WHERE latest.folder_id = folders.id ORDER BY latest.id DESC LIMIT 1);

-- name: LastSucceededFullScanStart :one
-- Scans of one folder never overlap, so the highest id started last.
SELECT started_at FROM scans
WHERE folder_id = ? AND kind = 'full' AND status = 'succeeded'
ORDER BY id DESC
LIMIT 1;

-- name: LastFinishedScanStart :one
SELECT started_at FROM scans
WHERE folder_id = ? AND status IN ('succeeded', 'failed')
ORDER BY id DESC
LIMIT 1;
