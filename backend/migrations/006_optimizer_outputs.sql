-- +goose Up
-- +goose StatementBegin
-- Every output the optimizer wrote, by its SHA-256. Unlike images.output_sha256 it survives the
-- image row, the folder and a new upload at the same path, so bytes the optimizer wrote are never
-- compressed lossily again wherever they turn up.
CREATE TABLE optimizer_outputs (
    sha256 BLOB PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
) WITHOUT ROWID;
INSERT OR IGNORE INTO optimizer_outputs (sha256) SELECT output_sha256 FROM images WHERE output_sha256 IS NOT NULL;
DROP INDEX idx_images_output;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE INDEX idx_images_output ON images (output_sha256) WHERE output_sha256 IS NOT NULL;
DROP TABLE IF EXISTS optimizer_outputs;
-- +goose StatementEnd
