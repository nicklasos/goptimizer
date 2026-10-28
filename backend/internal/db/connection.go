package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"app/config"

	_ "modernc.org/sqlite"
)

// dsnOptions apply to every connection the pool opens. Times are written in UTC in a format
// SQLite's date functions read, so SQL can compare them with datetime('now').
const dsnOptions = "_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)" +
	"&_txlock=immediate&_time_format=sqlite&_timezone=UTC"

// NewConnection opens the database at cfg.DatabasePath, creating its directory if needed.
func NewConnection(cfg *config.Config) (*sql.DB, error) {
	return Open(cfg.DatabasePath)
}

// Open opens the SQLite file at path with the application's connection settings.
func Open(path string) (*sql.DB, error) {
	if path == "" {
		return nil, fmt.Errorf("DATABASE_PATH is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	conn, err := sql.Open("sqlite", "file:"+path+"?"+dsnOptions)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := conn.PingContext(context.Background()); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return conn, nil
}
