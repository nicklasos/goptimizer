-- name: GetRestoreByID :one
SELECT sqlc.embed(restores),
       archives.object_key AS archive_key, archives.size_bytes AS archive_size, archives.created_at AS archive_created_at,
       storages.name AS storage_name,
       users.name AS created_by_name, users.email AS created_by_email
FROM restores
LEFT JOIN archives ON archives.id = restores.archive_id
LEFT JOIN storages ON storages.id = archives.storage_id
LEFT JOIN users ON users.id = restores.created_by
WHERE restores.id = ?1 LIMIT 1;

-- name: GetRestoreJob :one
SELECT * FROM restores
WHERE id = ?1 LIMIT 1;

-- name: CreateRestore :one
INSERT INTO restores (archive_id, plan_id, source_kind, target_config, target_secret, created_by)
VALUES (sqlc.arg(archive_id), sqlc.arg(plan_id), sqlc.arg(source_kind), sqlc.arg(target_config), sqlc.arg(target_secret), sqlc.arg(created_by))
RETURNING *;

-- name: CountActiveRestoresOfPlan :one
SELECT COUNT(*) FROM restores
WHERE plan_id = ?1 AND status IN ('queued', 'running');

-- A restore is claimed only while queued and while its plan has no other restore and no run
-- running, in one statement, the same rule as ClaimRun from the other side.
-- name: ClaimRestore :execrows
UPDATE restores
SET status = 'running', started_at = sqlc.arg(started_at)
WHERE restores.id = sqlc.arg(id)
  AND restores.status = 'queued'
  AND NOT EXISTS (SELECT 1 FROM restores AS other WHERE other.plan_id = restores.plan_id AND other.status = 'running')
  AND NOT EXISTS (SELECT 1 FROM runs WHERE runs.plan_id = restores.plan_id AND runs.status = 'running');

-- name: FinishRestore :execrows
UPDATE restores
SET status = sqlc.arg(status),
    finished_at = sqlc.arg(finished_at),
    log = sqlc.arg(log),
    error = sqlc.arg(error)
WHERE id = sqlc.arg(id) AND status = 'running';

-- name: UpdateRestoreLog :exec
UPDATE restores SET log = ?2
WHERE id = ?1 AND status = 'running';

-- name: FailRunningRestores :execrows
UPDATE restores
SET status = 'failed', finished_at = sqlc.arg(finished_at), error = sqlc.arg(error)
WHERE status = 'running';

-- The log is left out of lists, as for runs.
-- name: ListRestoresOfPlanPaginated :many
SELECT restores.id, restores.archive_id, restores.plan_id, restores.source_kind, restores.target_config,
       restores.target_secret, restores.status, CAST('' AS TEXT) AS log, restores.error, restores.created_by,
       restores.created_at, restores.started_at, restores.finished_at,
       archives.object_key AS archive_key, archives.size_bytes AS archive_size, archives.created_at AS archive_created_at,
       storages.name AS storage_name,
       users.name AS created_by_name, users.email AS created_by_email
FROM restores
LEFT JOIN archives ON archives.id = restores.archive_id
LEFT JOIN storages ON storages.id = archives.storage_id
LEFT JOIN users ON users.id = restores.created_by
WHERE restores.plan_id = ?1
ORDER BY restores.id DESC
LIMIT ?2 OFFSET ?3;

-- name: CountRestoresOfPlan :one
SELECT COUNT(*) FROM restores
WHERE plan_id = ?1;

-- name: ListRestoringPlans :many
SELECT DISTINCT plan_id FROM restores
WHERE plan_id IN (sqlc.slice(plan_ids)) AND status IN ('queued', 'running');
