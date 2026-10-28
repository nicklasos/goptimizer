-- +goose Up
-- +goose StatementBegin
DROP TABLE IF EXISTS examples;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE TABLE examples (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_examples_user_id ON examples(user_id);
CREATE INDEX idx_examples_created_at ON examples(created_at DESC);
-- +goose StatementEnd
