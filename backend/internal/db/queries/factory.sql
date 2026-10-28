-- These queries exist for internal/factory, which writes test and seed data with columns the
-- API sets itself (backdated timestamps, a chosen status, a stored log). Application code uses
-- the other files.

-- name: FactoryCreateUser :one
INSERT INTO users (email, name, password, roles, email_verified_at)
VALUES (
    sqlc.arg(email), sqlc.arg(name), sqlc.arg(password), sqlc.arg(roles),
    CASE WHEN CAST(sqlc.arg(verified) AS BOOLEAN) THEN CURRENT_TIMESTAMP END
)
RETURNING *;

-- name: FactoryCreateStorage :one
INSERT INTO storages (name, kind, config, secret)
VALUES (sqlc.arg(name), sqlc.arg(kind), sqlc.arg(config), sqlc.arg(secret))
RETURNING *;

-- name: FactoryCreatePlan :one
INSERT INTO plans (
    name, source_kind, source_config, source_secret, schedules,
    max_age_days, keep_weekly, keep_monthly, enabled, created_at, updated_at
) VALUES (
    sqlc.arg(name), sqlc.arg(source_kind), sqlc.arg(source_config), sqlc.arg(source_secret), sqlc.arg(schedules),
    sqlc.arg(max_age_days), sqlc.arg(keep_weekly), sqlc.arg(keep_monthly), sqlc.arg(enabled),
    COALESCE(sqlc.narg(created_at), CURRENT_TIMESTAMP), COALESCE(sqlc.narg(created_at), CURRENT_TIMESTAMP)
)
RETURNING *;

-- name: FactoryLinkPlanStorage :one
INSERT INTO plan_storages (plan_id, storage_id) VALUES (sqlc.arg(plan_id), sqlc.arg(storage_id))
RETURNING plan_id;

-- name: FactoryCreateRun :one
INSERT INTO runs (plan_id, trigger, scheduled_for, status, started_at, finished_at, size_bytes, error)
VALUES (
    sqlc.arg(plan_id), sqlc.arg(trigger), sqlc.arg(scheduled_for), sqlc.arg(status),
    sqlc.arg(started_at), sqlc.arg(finished_at), sqlc.arg(size_bytes), sqlc.arg(error)
)
RETURNING *;

-- name: FactoryCreateArchive :one
INSERT INTO archives (run_id, plan_id, storage_id, object_key, size_bytes, sha256, created_at, deleted_at)
VALUES (
    sqlc.arg(run_id), sqlc.arg(plan_id), sqlc.arg(storage_id), sqlc.arg(object_key),
    sqlc.arg(size_bytes), sqlc.arg(sha256), sqlc.arg(created_at), sqlc.arg(deleted_at)
)
RETURNING *;

-- name: FactoryCreateRestore :one
INSERT INTO restores (archive_id, plan_id, source_kind, target_config, status, started_at, finished_at, error, created_by)
VALUES (
    sqlc.arg(archive_id), sqlc.arg(plan_id), sqlc.arg(source_kind), sqlc.arg(target_config), sqlc.arg(status),
    sqlc.arg(started_at), sqlc.arg(finished_at), sqlc.arg(error), sqlc.arg(created_by)
)
RETURNING *;
