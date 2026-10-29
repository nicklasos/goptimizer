-- +goose Up
-- +goose StatementBegin
-- One index per order of the folder page's image list, so each keyset page is one seek however
-- many images the folder has.
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
