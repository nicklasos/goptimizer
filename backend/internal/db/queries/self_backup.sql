-- name: ListSQLitePlansByPath :many
SELECT * FROM plans
WHERE source_kind = 'sqlite' AND json_extract(source_config, '$.path') = sqlc.arg(path)
ORDER BY id;

-- name: ListLocalStoragesByPath :many
SELECT * FROM storages
WHERE kind = 'local' AND json_extract(config, '$.path') = sqlc.arg(path)
ORDER BY id;
