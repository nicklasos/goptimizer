-- +goose Up
-- +goose StatementBegin
-- Images skipped for data beyond what browsers show, or for content another format than their
-- extension, are optimized now: the optimizer strips that data and goes by the content. Nothing
-- retries a skipped image on its own, so they go back to pending. A wrong_content image whose
-- content is no image at all is skipped again on its next claim.
UPDATE images
SET status = 'pending', reason = NULL, attempts = 0, next_attempt_at = NULL
WHERE status = 'skipped' AND (reason = 'wrong_content' OR reason LIKE 'extra_data%');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
