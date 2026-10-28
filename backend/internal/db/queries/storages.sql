-- name: GetStorageByID :one
SELECT * FROM storages
WHERE id = ?1 LIMIT 1;

-- name: CreateStorage :one
INSERT INTO storages (
    name, kind, config, secret
) VALUES (
    sqlc.arg(name), sqlc.arg(kind), sqlc.arg(config), sqlc.arg(secret)
)
RETURNING *;

-- name: UpdateStorage :one
UPDATE storages
SET
    name = sqlc.arg(name),
    kind = sqlc.arg(kind),
    config = sqlc.arg(config),
    secret = sqlc.arg(secret),
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: DeleteStorage :exec
DELETE FROM storages
WHERE id = ?1;

-- name: ListStoragesPaginated :many
SELECT * FROM storages
ORDER BY name COLLATE NOCASE, id
LIMIT ?1 OFFSET ?2;

-- name: CountStorages :one
SELECT COUNT(*) FROM storages;

-- name: ListPlansUsingStorage :many
SELECT plans.id, plans.name
FROM plans
JOIN plan_storages ON plan_storages.plan_id = plans.id
WHERE plan_storages.storage_id = ?1
ORDER BY plans.name COLLATE NOCASE, plans.id;
