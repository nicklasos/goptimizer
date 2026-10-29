-- +goose Up
-- +goose StatementBegin
-- The folder page lists a folder's images one keyset page at a time: every image or one status,
-- the latest change first, and the optimized ones by the bytes they saved. Each page is one seek
-- and a short range scan on one of these, however many images the folder has. A search by path
-- reads idx_images_path instead.
CREATE INDEX idx_images_updated ON images (folder_id, updated_at);
CREATE INDEX idx_images_status_updated ON images (folder_id, status, updated_at);
CREATE INDEX idx_images_saved ON images (folder_id, (original_size - optimized_size)) WHERE status = 'optimized';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_images_saved;
DROP INDEX IF EXISTS idx_images_status_updated;
DROP INDEX IF EXISTS idx_images_updated;
-- +goose StatementEnd
