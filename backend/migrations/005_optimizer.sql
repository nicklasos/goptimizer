-- +goose Up
-- +goose StatementBegin
-- output_sha256 is the SHA-256 of the bytes the optimizer last wrote to the file, stored before the
-- rename. A file whose bytes have that hash, in any row, is the optimizer's own output and is never
-- compressed lossily again. next_attempt_at is when a failed image is tried again; NULL once it
-- has failed 3 times.
ALTER TABLE images ADD COLUMN output_sha256 BLOB;
ALTER TABLE images ADD COLUMN next_attempt_at TIMESTAMP;

-- The claim looks up the oldest pending image of one folder and format whose mtime is old enough.
-- It also serves the counts by folder and status, which idx_images_status did.
DROP INDEX idx_images_status;
CREATE INDEX idx_images_claim ON images (folder_id, status, format, mtime_ns);
-- At most a few rows each: the images being optimized, and the failed ones waiting for a retry.
CREATE INDEX idx_images_optimizing ON images (folder_id) WHERE status = 'optimizing';
CREATE INDEX idx_images_retry ON images (next_attempt_at) WHERE status = 'failed' AND next_attempt_at IS NOT NULL;
CREATE INDEX idx_images_output ON images (output_sha256) WHERE output_sha256 IS NOT NULL;

-- One row per folder and UTC day with optimizations, for the folder totals and the 30-day chart.
CREATE TABLE daily_stats (
    folder_id INTEGER NOT NULL REFERENCES folders(id) ON DELETE CASCADE,
    day TEXT NOT NULL,
    optimized INTEGER NOT NULL DEFAULT 0,
    bytes_before INTEGER NOT NULL DEFAULT 0,
    bytes_after INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (folder_id, day)
) WITHOUT ROWID;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS daily_stats;
DROP INDEX IF EXISTS idx_images_output;
DROP INDEX IF EXISTS idx_images_retry;
DROP INDEX IF EXISTS idx_images_optimizing;
DROP INDEX IF EXISTS idx_images_claim;
CREATE INDEX idx_images_status ON images (folder_id, status);
ALTER TABLE images DROP COLUMN next_attempt_at;
ALTER TABLE images DROP COLUMN output_sha256;
-- +goose StatementEnd
