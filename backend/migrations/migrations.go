// Package migrations embeds the SQL migrations, so a built binary carries its own schema.
package migrations

import "embed"

// Dialect is the goose dialect the migrations are written in.
const Dialect = "sqlite3"

//go:embed *.sql
var FS embed.FS
