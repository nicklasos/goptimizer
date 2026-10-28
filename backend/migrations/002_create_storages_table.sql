-- +goose Up
-- +goose StatementBegin
-- config is the kind's settings as JSON. secret is the kind's credentials as JSON, sealed with
-- ENCRYPTION_KEY (AES-256-GCM); NULL for a kind that has none.
CREATE TABLE storages (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    kind TEXT NOT NULL CHECK (kind IN ('local', 's3', 'gcs')),
    config TEXT NOT NULL,
    secret BLOB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS storages;
-- +goose StatementEnd
