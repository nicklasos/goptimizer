package tests

import (
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"app/internal/db"
	"app/migrations"

	"github.com/pressly/goose/v3"
)

var (
	templateOnce  sync.Once
	templateBytes []byte
	templateErr   error
)

// NewTestDB opens a fresh, migrated database file that only this test uses, removed when
// the test ends. The schema is migrated once per test binary and copied for each test.
func NewTestDB(t testing.TB) *sql.DB {
	t.Helper()

	templateOnce.Do(func() { templateBytes, templateErr = migrateTemplate() })
	if templateErr != nil {
		t.Fatalf("tests: migrate template database: %v", templateErr)
	}

	path := filepath.Join(t.TempDir(), "app_test.db")
	if err := os.WriteFile(path, templateBytes, 0o600); err != nil {
		t.Fatalf("tests: copy template database: %v", err)
	}

	conn, err := db.Open(path)
	if err != nil {
		t.Fatalf("tests: open database: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// migrateTemplate returns the bytes of a freshly migrated database. Closing the last
// connection checkpoints the WAL, so the single file holds the whole schema.
func migrateTemplate() ([]byte, error) {
	dir, err := os.MkdirTemp("", "goptimizer-template-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "template_test.db")
	conn, err := db.Open(path)
	if err != nil {
		return nil, err
	}

	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect(migrations.Dialect); err != nil {
		conn.Close()
		return nil, err
	}
	if err := goose.Up(conn, "."); err != nil {
		conn.Close()
		return nil, err
	}
	if err := conn.Close(); err != nil {
		return nil, err
	}

	return os.ReadFile(path)
}
