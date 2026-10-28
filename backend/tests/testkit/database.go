package testkit

import (
	"sort"
	"strings"
)

// Row is the columns a row must have, for the database assertions:
//
//	app.AssertDatabaseHas("storages", testkit.Row{"name": "Nightly", "kind": "local"})
//
// A nil value matches NULL.
type Row map[string]any

func (a *App) AssertDatabaseHas(table string, row Row) {
	a.t.Helper()
	if a.count(table, row) == 0 {
		a.t.Fatalf("expected a row in %s matching %v", table, row)
	}
}

func (a *App) AssertDatabaseMissing(table string, row Row) {
	a.t.Helper()
	if count := a.count(table, row); count != 0 {
		a.t.Fatalf("expected no row in %s matching %v, found %d", table, row, count)
	}
}

// AssertDatabaseCount checks how many rows match. Pass no row to count the whole table.
func (a *App) AssertDatabaseCount(table string, want int, row ...Row) {
	a.t.Helper()
	var where Row
	if len(row) > 0 {
		where = row[0]
	}
	if count := a.count(table, where); count != want {
		a.t.Fatalf("expected %d rows in %s matching %v, found %d", want, table, where, count)
	}
}

func (a *App) count(table string, row Row) int {
	a.t.Helper()

	columns := make([]string, 0, len(row))
	for column := range row {
		columns = append(columns, column)
	}
	sort.Strings(columns)

	var conditions []string
	var args []any
	for _, column := range columns {
		name := quoteIdentifier(column)
		if row[column] == nil {
			conditions = append(conditions, name+" IS NULL")
			continue
		}
		args = append(args, row[column])
		conditions = append(conditions, name+" = ?")
	}

	query := "SELECT count(*) FROM " + quoteIdentifier(table)
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	var count int
	if err := a.DB.QueryRowContext(a.Ctx, query, args...).Scan(&count); err != nil {
		a.t.Fatalf("testkit: count rows in %s: %v", table, err)
	}
	return count
}

func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
