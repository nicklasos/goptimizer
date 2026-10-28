-- +goose Up
-- +goose StatementBegin
-- A restore puts an archive back into a target, by default the plan's own source. It goes
-- queued -> running -> succeeded or failed like a run, and never runs while a run or another
-- restore of its plan runs. target_config is the target as JSON, without its password;
-- target_secret is the password typed for it, sealed with ENCRYPTION_KEY, NULL to use the plan's.
CREATE TABLE restores (
    id INTEGER PRIMARY KEY,
    archive_id INTEGER REFERENCES archives(id) ON DELETE SET NULL,
    plan_id INTEGER NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    source_kind TEXT NOT NULL CHECK (source_kind IN ('postgres', 'mysql', 'sqlite', 'files')),
    target_config TEXT NOT NULL,
    target_secret BLOB,
    status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'succeeded', 'failed')),
    log TEXT NOT NULL DEFAULT '',
    error TEXT,
    created_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMP,
    finished_at TIMESTAMP
);
CREATE INDEX idx_restores_plan_id ON restores(plan_id, id);
CREATE INDEX idx_restores_active ON restores(status) WHERE status IN ('queued', 'running');
CREATE INDEX idx_restores_archive_id ON restores(archive_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS restores;
-- +goose StatementEnd
