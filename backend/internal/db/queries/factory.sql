-- These queries exist for internal/factory, which writes test and seed data with columns the
-- API sets itself (a verified email). Application code uses the other files.

-- name: FactoryCreateUser :one
INSERT INTO users (email, name, password, is_admin, email_verified_at)
VALUES (
    sqlc.arg(email), sqlc.arg(name), sqlc.arg(password), sqlc.arg(is_admin),
    CASE WHEN CAST(sqlc.arg(verified) AS BOOLEAN) THEN CURRENT_TIMESTAMP END
)
RETURNING *;

-- name: FactoryCreateFolder :one
INSERT INTO folders (name, kind, path, enabled, scan_interval_minutes, full_scan_interval_hours, profile)
VALUES (
    sqlc.arg(name), sqlc.arg(kind), sqlc.arg(path), sqlc.arg(enabled),
    sqlc.arg(scan_interval_minutes), sqlc.arg(full_scan_interval_hours), sqlc.arg(profile)
)
RETURNING *;

-- name: FactoryCreateScan :one
INSERT INTO scans (
    folder_id, kind, trigger, status, queued_at, started_at, finished_at, error,
    dirs_seen, dirs_listed, files_seen, files_new, files_changed, files_gone
)
VALUES (
    sqlc.arg(folder_id), sqlc.arg(kind), sqlc.arg(trigger), sqlc.arg(status),
    sqlc.arg(queued_at), sqlc.arg(started_at), sqlc.arg(finished_at), sqlc.arg(error),
    sqlc.arg(dirs_seen), sqlc.arg(dirs_listed), sqlc.arg(files_seen), sqlc.arg(files_new), sqlc.arg(files_changed), sqlc.arg(files_gone)
)
RETURNING *;

-- name: FactoryCreateImage :one
INSERT INTO images (
    folder_id, dir, name, format, size, mtime_ns, status, original_size, optimized_size, mode, reason, attempts,
    next_attempt_at, output_sha256, optimized_at, discovered_at, updated_at
)
VALUES (
    sqlc.arg(folder_id), sqlc.arg(dir), sqlc.arg(name), sqlc.arg(format), sqlc.arg(size), sqlc.arg(mtime_ns),
    sqlc.arg(status), sqlc.arg(original_size), sqlc.arg(optimized_size), sqlc.arg(mode), sqlc.arg(reason), sqlc.arg(attempts),
    sqlc.arg(next_attempt_at), sqlc.arg(output_sha256), sqlc.arg(optimized_at), sqlc.arg(now), sqlc.arg(now)
)
RETURNING *;

-- name: FactoryCreateDailyStat :one
INSERT INTO daily_stats (folder_id, day, optimized, bytes_before, bytes_after)
VALUES (sqlc.arg(folder_id), sqlc.arg(day), sqlc.arg(optimized), sqlc.arg(bytes_before), sqlc.arg(bytes_after))
RETURNING *;
