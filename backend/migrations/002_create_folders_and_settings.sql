-- +goose Up
-- +goose StatementBegin
CREATE TABLE folders (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    kind TEXT NOT NULL DEFAULT 'local',
    path TEXT NOT NULL UNIQUE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    scan_interval_minutes INTEGER NOT NULL DEFAULT 60,
    full_scan_interval_hours INTEGER NOT NULL DEFAULT 24,
    profile TEXT NOT NULL DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX idx_folders_name_lower ON folders (LOWER(name));

-- One row. workers = 0 means one less than the CPU count, decided at runtime.
CREATE TABLE settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    workers INTEGER NOT NULL DEFAULT 0,
    scan_workers INTEGER NOT NULL DEFAULT 2,
    nice BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO settings (id) VALUES (1);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS settings;
DROP TABLE IF EXISTS folders;
-- +goose StatementEnd
