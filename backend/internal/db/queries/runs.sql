-- name: GetRunByID :one
SELECT * FROM runs
WHERE id = ?1 LIMIT 1;

-- name: CreateManualRun :one
INSERT INTO runs (plan_id, trigger, status) VALUES (?1, 'manual', 'queued')
RETURNING *;

-- name: EnqueueScheduledRun :execrows
INSERT INTO runs (plan_id, trigger, scheduled_for, status)
VALUES (sqlc.arg(plan_id), 'schedule', sqlc.arg(scheduled_for), 'queued')
ON CONFLICT (plan_id, scheduled_for) WHERE trigger = 'schedule' DO NOTHING;

-- name: LastScheduledSlot :one
SELECT scheduled_for FROM runs
WHERE plan_id = ?1 AND trigger = 'schedule'
ORDER BY scheduled_for DESC
LIMIT 1;

-- name: CountActiveRunsOfPlan :one
SELECT COUNT(*) FROM runs
WHERE plan_id = ?1 AND status IN ('queued', 'running');

-- The queue of the worker: queued runs and restores, oldest first, whose plan has neither a run
-- nor a restore running. A plan runs one job at a time, so a backup never reads a database that
-- is half restored.
-- name: ListClaimableJobs :many
SELECT CAST('run' AS TEXT) AS kind, runs.id, runs.created_at FROM runs
WHERE runs.status = 'queued'
  AND NOT EXISTS (SELECT 1 FROM runs AS other WHERE other.plan_id = runs.plan_id AND other.status = 'running')
  AND NOT EXISTS (SELECT 1 FROM restores WHERE restores.plan_id = runs.plan_id AND restores.status = 'running')
UNION ALL
SELECT CAST('restore' AS TEXT) AS kind, restores.id, restores.created_at FROM restores
WHERE restores.status = 'queued'
  AND NOT EXISTS (SELECT 1 FROM restores AS other WHERE other.plan_id = restores.plan_id AND other.status = 'running')
  AND NOT EXISTS (SELECT 1 FROM runs WHERE runs.plan_id = restores.plan_id AND runs.status = 'running')
ORDER BY created_at, kind, id
LIMIT ?1;

-- A run is claimed only while queued and while its plan has no other run and no restore running,
-- in one statement, so neither two workers nor two processes can start it twice.
-- name: ClaimRun :execrows
UPDATE runs
SET status = 'running', started_at = sqlc.arg(started_at)
WHERE runs.id = sqlc.arg(id)
  AND runs.status = 'queued'
  AND NOT EXISTS (SELECT 1 FROM runs AS other WHERE other.plan_id = runs.plan_id AND other.status = 'running')
  AND NOT EXISTS (SELECT 1 FROM restores WHERE restores.plan_id = runs.plan_id AND restores.status = 'running');

-- name: FinishRun :execrows
UPDATE runs
SET status = sqlc.arg(status),
    finished_at = sqlc.arg(finished_at),
    size_bytes = sqlc.arg(size_bytes),
    log = sqlc.arg(log),
    error = sqlc.arg(error)
WHERE id = sqlc.arg(id) AND status = 'running';

-- name: UpdateRunLog :exec
UPDATE runs SET log = ?2
WHERE id = ?1 AND status = 'running';

-- name: FailRunningRuns :execrows
UPDATE runs
SET status = 'failed', finished_at = sqlc.arg(finished_at), error = sqlc.arg(error)
WHERE status = 'running';

-- name: ListRunsOfPlanPaginated :many
-- The log is left out of lists: it can be a megabyte per run. The columns stay in table order,
-- so a row converts to db.Run.
SELECT id, plan_id, trigger, scheduled_for, status, started_at, finished_at, size_bytes, CAST('' AS TEXT) AS log, error, created_at
FROM runs
WHERE plan_id = ?1
ORDER BY id DESC
LIMIT ?2 OFFSET ?3;

-- name: CountRunsOfPlan :one
SELECT COUNT(*) FROM runs
WHERE plan_id = ?1;

-- name: LatestRunsOfPlans :many
SELECT id, plan_id, trigger, scheduled_for, status, started_at, finished_at, size_bytes, CAST('' AS TEXT) AS log, error, created_at
FROM runs
WHERE id IN (SELECT MAX(r.id) FROM runs AS r WHERE r.plan_id IN (sqlc.slice(plan_ids)) GROUP BY r.plan_id);

-- name: LatestSuccessesOfPlans :many
SELECT id, plan_id, finished_at, size_bytes
FROM runs
WHERE id IN (
    SELECT MAX(r.id) FROM runs AS r
    WHERE r.plan_id IN (sqlc.slice(plan_ids)) AND r.status = 'succeeded'
    GROUP BY r.plan_id
);

-- One row per plan and UTC day with a finished run on or after the day `since` (YYYY-MM-DD), for
-- the dashboard's strip of days. A run counts on the day it started.
-- name: CountFinishedRunsPerDay :many
SELECT plan_id,
       CAST(date(started_at) AS TEXT) AS day,
       CAST(SUM(status = 'succeeded') AS INTEGER) AS succeeded,
       CAST(SUM(status = 'failed') AS INTEGER) AS failed
FROM runs
WHERE status IN ('succeeded', 'failed')
  AND date(started_at) >= CAST(sqlc.arg(since) AS TEXT)
GROUP BY plan_id, day
ORDER BY plan_id, day;

-- name: ListEnabledPlans :many
SELECT * FROM plans
WHERE enabled
ORDER BY id;

-- name: ListStoragesForPlan :many
SELECT storages.* FROM storages
JOIN plan_storages ON plan_storages.storage_id = storages.id
WHERE plan_storages.plan_id = ?1
ORDER BY storages.name COLLATE NOCASE, storages.id;

-- name: ListAlertRecipients :many
SELECT email FROM users
WHERE email_verified_at IS NOT NULL
  AND EXISTS (SELECT 1 FROM json_each(users.roles) WHERE json_each.value IN ('admin', 'super-admin'))
ORDER BY id;

-- name: CreateArchive :one
INSERT INTO archives (run_id, plan_id, storage_id, object_key, size_bytes, sha256, created_at)
VALUES (
    sqlc.arg(run_id), sqlc.arg(plan_id), sqlc.arg(storage_id), sqlc.arg(object_key),
    sqlc.arg(size_bytes), sqlc.arg(sha256), sqlc.arg(created_at)
)
RETURNING *;

-- name: ListArchivesOfRun :many
SELECT sqlc.embed(archives), storages.name AS storage_name, storages.kind AS storage_kind
FROM archives
JOIN storages ON storages.id = archives.storage_id
WHERE archives.run_id = ?1
ORDER BY storages.name COLLATE NOCASE, archives.id;

-- name: ListLiveArchivesOfPlan :many
SELECT sqlc.embed(archives), storages.name AS storage_name, storages.kind AS storage_kind
FROM archives
JOIN storages ON storages.id = archives.storage_id
WHERE archives.plan_id = sqlc.arg(plan_id)
  AND archives.deleted_at IS NULL
  AND (CAST(sqlc.arg(storage_id) AS INTEGER) = 0 OR archives.storage_id = sqlc.arg(storage_id))
ORDER BY archives.created_at DESC, archives.id DESC
LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset);

-- name: CountLiveArchivesOfPlan :one
SELECT COUNT(*) FROM archives
WHERE plan_id = sqlc.arg(plan_id)
  AND deleted_at IS NULL
  AND (CAST(sqlc.arg(storage_id) AS INTEGER) = 0 OR storage_id = sqlc.arg(storage_id));

-- name: ListLiveArchivesOfPlanStorage :many
SELECT * FROM archives
WHERE plan_id = ?1 AND storage_id = ?2 AND deleted_at IS NULL
ORDER BY created_at, id;

-- name: MarkArchiveDeleted :exec
UPDATE archives SET deleted_at = ?2
WHERE id = ?1 AND deleted_at IS NULL;

-- name: GetArchiveByID :one
SELECT * FROM archives
WHERE id = ?1 LIMIT 1;
