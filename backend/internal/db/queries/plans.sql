-- name: GetPlanByID :one
SELECT * FROM plans
WHERE id = ?1 LIMIT 1;

-- name: CreatePlan :one
INSERT INTO plans (
    name, source_kind, source_config, source_secret, schedules,
    max_age_days, keep_weekly, keep_monthly, enabled
) VALUES (
    sqlc.arg(name), sqlc.arg(source_kind), sqlc.arg(source_config), sqlc.arg(source_secret), sqlc.arg(schedules),
    sqlc.arg(max_age_days), sqlc.arg(keep_weekly), sqlc.arg(keep_monthly), sqlc.arg(enabled)
)
RETURNING *;

-- name: UpdatePlan :one
UPDATE plans
SET
    name = sqlc.arg(name),
    source_kind = sqlc.arg(source_kind),
    source_config = sqlc.arg(source_config),
    source_secret = sqlc.arg(source_secret),
    schedules = sqlc.arg(schedules),
    max_age_days = sqlc.arg(max_age_days),
    keep_weekly = sqlc.arg(keep_weekly),
    keep_monthly = sqlc.arg(keep_monthly),
    enabled = sqlc.arg(enabled),
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: DeletePlan :exec
DELETE FROM plans
WHERE id = ?1;

-- name: ListPlansPaginated :many
SELECT * FROM plans
ORDER BY name COLLATE NOCASE, id
LIMIT ?1 OFFSET ?2;

-- name: ListPlans :many
SELECT * FROM plans
ORDER BY name COLLATE NOCASE, id;

-- Only a change counts as a save: the scheduler skips slots from before the last save, so
-- resuming a plan never starts the backup it missed while paused.
-- name: SetPlanEnabled :one
UPDATE plans
SET enabled = sqlc.arg(enabled),
    updated_at = CASE WHEN enabled = sqlc.arg(enabled) THEN updated_at ELSE CURRENT_TIMESTAMP END
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: CountPlans :one
SELECT COUNT(*) FROM plans;

-- name: AddPlanStorage :exec
INSERT INTO plan_storages (plan_id, storage_id) VALUES (?1, ?2);

-- name: ClearPlanStorages :exec
DELETE FROM plan_storages
WHERE plan_id = ?1;

-- name: ListStoragesOfPlans :many
SELECT plan_storages.plan_id, storages.id, storages.name, storages.kind
FROM plan_storages
JOIN storages ON storages.id = plan_storages.storage_id
WHERE plan_storages.plan_id IN (sqlc.slice(plan_ids))
ORDER BY storages.name COLLATE NOCASE, storages.id;

-- name: CountStoragesByIDs :one
SELECT COUNT(*) FROM storages
WHERE id IN (sqlc.slice(ids));
