package internal

import (
	"database/sql"

	"app/config"
	_ "app/docs"
	"app/internal/cache"
	"app/internal/db"
	"app/internal/logger"
	"app/internal/mail"
	"app/internal/secretbox"
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
	Secrets *secretbox.Box
	Tools   tools.Finder
	Api     *gin.RouterGroup

	// WakeWorker tells the runner in this process that a run was queued. Nil when the runner
	// lives in cmd/cron (ENABLE_SCHEDULER=false), which finds queued runs by polling.
	WakeWorker func()

	// AuthMiddleware is set by auth.RegisterRoutes; modules registered after it use it to protect routes
	AuthMiddleware gin.HandlerFunc
}
