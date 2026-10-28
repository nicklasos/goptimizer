package internal

import (
	"database/sql"

	"app/config"
	"app/internal/db"
	"app/internal/logger"
)

// CLIApp contains shared components for CLI commands
type CLIApp struct {
	Config   *config.Config
	Database *sql.DB
	Queries  *db.Queries
	Logger   *logger.Logger
}
