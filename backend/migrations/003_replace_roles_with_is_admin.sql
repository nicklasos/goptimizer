-- +goose Up
-- +goose StatementBegin
ALTER TABLE users ADD COLUMN is_admin BOOLEAN NOT NULL DEFAULT FALSE;
UPDATE users SET is_admin = TRUE
WHERE EXISTS (SELECT 1 FROM json_each(users.roles) WHERE json_each.value IN ('admin', 'super-admin'));
ALTER TABLE users DROP COLUMN roles;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users ADD COLUMN roles TEXT NOT NULL DEFAULT '["user"]';
UPDATE users SET roles = CASE WHEN is_admin THEN '["admin"]' ELSE '["user"]' END;
ALTER TABLE users DROP COLUMN is_admin;
-- +goose StatementEnd
