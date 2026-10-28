-- +goose Up
-- +goose StatementBegin
-- A run goes queued -> running -> succeeded or failed, each step one conditional UPDATE, so two
-- workers can never both take it. scheduled_for is the slot a scheduled run is for: the unique
-- index lets a slot be enqueued once, whichever process tries.
CREATE TABLE runs (
    id INTEGER PRIMARY KEY,
    plan_id INTEGER NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    trigger TEXT NOT NULL CHECK (trigger IN ('schedule', 'manual')),
    scheduled_for TIMESTAMP,
    status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'succeeded', 'failed')),
    started_at TIMESTAMP,
    finished_at TIMESTAMP,
    size_bytes INTEGER,
    log TEXT NOT NULL DEFAULT '',
    error TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CHECK ((trigger = 'schedule') = (scheduled_for IS NOT NULL))
);
CREATE UNIQUE INDEX idx_runs_plan_slot ON runs(plan_id, scheduled_for) WHERE trigger = 'schedule';
CREATE INDEX idx_runs_plan_id ON runs(plan_id, id);
CREATE INDEX idx_runs_active ON runs(status) WHERE status IN ('queued', 'running');

-- One row per storage an archive was stored on. Retention reads these rows, not the bucket, so it
-- never touches an object Gogobackup did not write. deleted_at is set once the object is gone.
-- Deleting a storage forgets its rows and leaves its objects where they are.
CREATE TABLE archives (
    id INTEGER PRIMARY KEY,
    run_id INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    plan_id INTEGER NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    storage_id INTEGER NOT NULL REFERENCES storages(id) ON DELETE CASCADE,
    object_key TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    sha256 TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);
CREATE INDEX idx_archives_plan_storage ON archives(plan_id, storage_id, created_at) WHERE deleted_at IS NULL;
CREATE INDEX idx_archives_run_id ON archives(run_id);
CREATE INDEX idx_archives_storage_id ON archives(storage_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS archives;
DROP TABLE IF EXISTS runs;
-- +goose StatementEnd
