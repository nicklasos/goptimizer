# Change the schema or a query

## A new migration

1. Create `migrations/NNN_description.sql`, where `NNN` is the highest existing number plus one.
   Never edit a migration that has been committed: other databases have already run it.
2. Use this shape:

   ```sql
   -- +goose Up
   -- +goose StatementBegin
   ALTER TABLE orders ADD COLUMN note TEXT;
   -- +goose StatementEnd

   -- +goose Down
   -- +goose StatementBegin
   ALTER TABLE orders DROP COLUMN note;
   -- +goose StatementEnd
   ```

3. Write SQLite: `INTEGER PRIMARY KEY` for IDs, `TEXT` for strings, `TIMESTAMP DEFAULT CURRENT_TIMESTAMP`
   for times, `BOOLEAN` for flags, `REFERENCES users(id) ON DELETE CASCADE` for owners.
   `ALTER TABLE` can add, rename and drop a column but cannot change one: to change a type or add
   `NOT NULL` to an existing column, create the new table, `INSERT INTO new SELECT ... FROM old`,
   drop the old table and rename the new one, all in the same migration.
4. A `NOT NULL` column added to a table with rows needs a `DEFAULT`.
5. Apply and regenerate:

   ```bash
   make migrate-up   # development database
   make sqlc         # sqlc reads the schema from migrations/
   ```

   `make test` needs nothing more: each test starts from a database migrated from `migrations/`.

## A new or changed query

1. Edit `internal/db/queries/<module>.sql`. One `-- name: VerbNoun :one|:many|:exec` per query.
   Use `?` for parameters, and `sqlc.arg(name)` when there are more than two or their meaning is not obvious.
   Cast a parameter whose type sqlc cannot infer: `CAST(sqlc.arg(flag) AS BOOLEAN)`.
   Compare a stored timestamp with now as `datetime(expires_at) > datetime('now')`, never as raw text.
2. `make sqlc`. Never edit `internal/db/*.sql.go` by hand.
3. Fix the callers the compiler points at.

## After either

- A changed column usually means a changed factory (`internal/factory/`) and response type (`types.go`).
- `make test`. If a response type changed: `make swagger`, then `make api-types` in `../frontend`.

## Check

- `go build ./...` is clean and `make test` passes.
- `make migrate-status` shows the new migration applied.
- The `Down` section really undoes the `Up` section.
