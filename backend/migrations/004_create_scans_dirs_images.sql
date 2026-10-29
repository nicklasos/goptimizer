-- +goose Up
-- +goose StatementBegin
CREATE TABLE scans (
    id INTEGER PRIMARY KEY,
    folder_id INTEGER NOT NULL REFERENCES folders(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    trigger TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'queued',
    queued_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMP,
    finished_at TIMESTAMP,
    dirs_seen INTEGER NOT NULL DEFAULT 0,
    dirs_listed INTEGER NOT NULL DEFAULT 0,
    files_seen INTEGER NOT NULL DEFAULT 0,
    files_new INTEGER NOT NULL DEFAULT 0,
    files_changed INTEGER NOT NULL DEFAULT 0,
    files_gone INTEGER NOT NULL DEFAULT 0,
    error TEXT
);
-- One scan per folder at a time, whoever queues it: the API, the schedule, another process.
CREATE UNIQUE INDEX idx_scans_active ON scans (folder_id) WHERE status IN ('queued', 'running');
CREATE INDEX idx_scans_folder ON scans (folder_id, id);

-- One row per directory a scan has listed. mtime_ns = 0 means "list it again next time".
CREATE TABLE dirs (
    folder_id INTEGER NOT NULL REFERENCES folders(id) ON DELETE CASCADE,
    rel_path TEXT NOT NULL,
    mtime_ns INTEGER NOT NULL,
    PRIMARY KEY (folder_id, rel_path)
) WITHOUT ROWID;

-- size and mtime_ns are the file on disk now. original_size and the columns after it are the
-- optimizer's history, kept when the file changes or comes back after being gone.
CREATE TABLE images (
    id INTEGER PRIMARY KEY,
    folder_id INTEGER NOT NULL REFERENCES folders(id) ON DELETE CASCADE,
    dir TEXT NOT NULL,
    name TEXT NOT NULL,
    format TEXT NOT NULL,
    size INTEGER NOT NULL,
    mtime_ns INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    original_size INTEGER,
    optimized_size INTEGER,
    mode TEXT,
    reason TEXT,
    attempts INTEGER NOT NULL DEFAULT 0,
    discovered_at TIMESTAMP NOT NULL,
    optimized_at TIMESTAMP,
    updated_at TIMESTAMP NOT NULL
);
-- Also serves the scanner's lookups by (folder_id, dir), so that needs no index of its own.
CREATE UNIQUE INDEX idx_images_path ON images (folder_id, dir, name);
CREATE INDEX idx_images_status ON images (folder_id, status);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS images;
DROP TABLE IF EXISTS dirs;
DROP TABLE IF EXISTS scans;
-- +goose StatementEnd
