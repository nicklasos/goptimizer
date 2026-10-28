-- +goose Up
-- +goose StatementBegin
-- source_config is the source kind's settings as JSON. source_secret is its credentials as JSON,
-- sealed with ENCRYPTION_KEY like storages.secret; NULL for a kind that has none.
-- schedules is a JSON array of {time: "HH:MM" in UTC, days: ["mon", ...]}; empty days is every day.
CREATE TABLE plans (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    source_kind TEXT NOT NULL CHECK (source_kind IN ('postgres', 'mysql', 'sqlite', 'files')),
    source_config TEXT NOT NULL,
    source_secret BLOB,
    schedules TEXT NOT NULL DEFAULT '[]',
    max_age_days INTEGER NOT NULL DEFAULT 0,
    keep_weekly INTEGER NOT NULL DEFAULT 0,
    keep_monthly INTEGER NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- A storage that a plan uploads to cannot be deleted until no plan uses it.
CREATE TABLE plan_storages (
    plan_id INTEGER NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    storage_id INTEGER NOT NULL REFERENCES storages(id) ON DELETE RESTRICT,
    PRIMARY KEY (plan_id, storage_id)
);
CREATE INDEX idx_plan_storages_storage_id ON plan_storages(storage_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS plan_storages;
DROP TABLE IF EXISTS plans;
-- +goose StatementEnd
