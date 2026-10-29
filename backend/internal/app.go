package internal

import (
	"database/sql"

	"app/config"
	_ "app/docs"
	"app/internal/cache"
	"app/internal/db"
	"app/internal/logger"
	"app/internal/mail"
	"app/internal/tools"

	"github.com/gin-gonic/gin"
)

type App struct {
	Config  *config.Config
	DB      *sql.DB
	Queries *db.Queries
	Tx      *db.TxRunner
	Cache   *cache.MemoryCache
	Logger  *logger.Logger
	Mail    mail.Sender
	Tools   tools.Finder
	Api     *gin.RouterGroup

	// AuthMiddleware is set by auth.RegisterRoutes; modules registered after it use it to protect routes
	AuthMiddleware gin.HandlerFunc

	// WakeScans tells the scan worker in this process that a scan was queued. Nil when the worker
	// runs in cmd/cron instead, which polls.
	WakeScans func()
}
