// Package testkit is what a test starts from:
//
//	func TestUsers_List(t *testing.T) {
//		app := testkit.New(t)
//		admin := app.Create.User(factory.Admin())
//		app.Create.User(factory.WithName("Ada"))
//
//		app.As(admin).GET("/api/v1/users").
//			AssertOK().
//			AssertJSONPath("data.0.name", "Ada")
//	}
//
// New gives each test its own freshly migrated SQLite file, so tests never see each other's
// rows. Everything hangs off the returned App: factories (app.Create), requests
// (app.GET, app.As(user).POST, ...), database assertions and the fakes (app.Mail).
package testkit

import (
	"context"
	"database/sql"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"app/config"
	"app/internal"
	"app/internal/cache"
	"app/internal/db"
	"app/internal/factory"
	"app/internal/logger"
	"app/internal/mail"
	"app/internal/middleware"
	"app/internal/optimizer"
	"app/internal/scanner"
	"app/internal/server"
	"app/internal/tools"
	"app/tests"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const JWTSecret = "test-secret-key"

// FakeToolVersions is what app.Tools reports for `<name> --version` at the start of a test.
var FakeToolVersions = map[string]string{
	"pngquant":  "3.0.3",
	"oxipng":    "oxipng 9.1.2",
	"optipng":   "OptiPNG version 0.7.8",
	"jpegoptim": "jpegoptim v1.5.5  x86_64-pc-linux-gnu",
	"gifsicle":  "LCDF Gifsicle 1.94",
}

type App struct {
	// Client sends requests without a token. Use As for an authenticated one.
	*Client

	Ctx     context.Context
	DB      *sql.DB
	Queries *db.Queries
	Create  *factory.Factory

	// TxRunner is for a service that opens transactions
	TxRunner *db.TxRunner

	Config *config.Config
	Logger *logger.Logger
	Cache  *cache.MemoryCache

	// Mail collects every message the application sent during the test
	Mail *mail.MemorySender

	// Tools stands in for the image programs on PATH: every one is installed until Remove
	Tools *tools.Fake

	t          *testing.T
	server     *httptest.Server
	scanWorker *scanner.Worker
	toolReport *tools.Reporter
	pool       *optimizer.Pool
}

// New starts a test. Pass configure functions to change the config:
//
//	app := testkit.New(t, func(cfg *config.Config) { cfg.AllowRegistration = false })
func New(t *testing.T, configure ...func(*config.Config)) *App {
	t.Helper()

	conn := tests.NewTestDB(t)

	log, err := logger.New(logger.Config{Level: "debug", Format: "text", Output: "stdout"})
	if err != nil {
		t.Fatalf("testkit: create logger: %v", err)
	}

	cfg := &config.Config{
		JWTSecret:   JWTSecret,
		AppName:     "TestApp",
		FrontendURL: "http://localhost:5747",

		AllowRegistration: true,
		AuthRateLimit:     true,
		TrustedProxies:    []string{"127.0.0.1", "::1"},
		WorkDir:           filepath.Join(t.TempDir(), "work"),
	}
	for _, fn := range configure {
		fn(cfg)
	}

	app := &App{
		Ctx:      context.Background(),
		DB:       conn,
		Queries:  db.New(conn),
		Create:   factory.New(t, conn),
		TxRunner: db.NewTxRunner(conn),
		Config:   cfg,
		Logger:   log,
		Cache:    cache.NewMemoryCache(),
		Mail:     mail.NewMemorySender(),
		Tools:    tools.NewFake(FakeToolVersions),
		t:        t,
	}
	app.Client = &Client{app: app}
	return app
}

// As returns a client that sends every request as the given user.
func (a *App) As(user *db.User) *Client {
	return a.WithToken(a.Token(user))
}

// WithToken returns a client that sends the access token, for one the application issued itself.
func (a *App) WithToken(token string) *Client {
	return a.WithHeader("Authorization", "Bearer "+token)
}

// Token is a valid access token for the user.
func (a *App) Token(user *db.User) string {
	a.t.Helper()

	claims := &middleware.Claims{
		UserID: user.ID,
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(a.Config.JWTSecret))
	if err != nil {
		a.t.Fatalf("testkit: sign token: %v", err)
	}
	return token
}

// URL is the address of the test server for a path, for the rare request the client cannot make.
func (a *App) URL(path string) string {
	return a.baseURL() + path
}

// baseURL starts the server on first use, so a test that makes no requests never builds one.
// It runs the same engine and routes as production, on the test's database.
func (a *App) baseURL() string {
	if a.server != nil {
		return a.server.URL
	}

	gin.SetMode(gin.TestMode)
	router := server.NewEngine(a.Config, a.Logger)
	server.RegisterRoutes(router, &internal.App{
		Config:     a.Config,
		DB:         a.DB,
		Queries:    a.Queries,
		Tx:         a.TxRunner,
		Cache:      a.Cache,
		Logger:     a.Logger,
		Mail:       a.Mail,
		Tools:      a.Tools,
		ToolReport: a.ToolReport(),
		Api:        router.Group("/api/v1"),

		WakeScans: a.ScanWorker().Wake,
	})

	a.server = httptest.NewServer(router)
	a.t.Cleanup(a.server.Close)
	return a.server.URL
}

// ScanWorker is the scan worker on the test's database. It is not started: Drain runs what is
// queued, synchronously. A test of the start and stop calls Start and registers Stop.
func (a *App) ScanWorker() *scanner.Worker {
	if a.scanWorker == nil {
		a.scanWorker = scanner.NewWorker(a.Queries, a.TxRunner, a.Logger, a.Pool().Wake)
	}
	return a.scanWorker
}

// ToolReport is what app.Tools has installed, as the API and the optimizer see it. It is cached
// like theirs: after changing app.Tools mid-test, call Refresh.
func (a *App) ToolReport() *tools.Reporter {
	if a.toolReport == nil {
		a.toolReport = tools.NewReporter(a.Tools, tools.ReportTTL)
	}
	return a.toolReport
}

// Pool is the optimizer's worker pool on the test's database, with a work dir of its own. It is
// not started: Drain optimizes what is claimable, synchronously.
func (a *App) Pool() *optimizer.Pool {
	if a.pool == nil {
		a.pool = optimizer.NewPool(a.Queries, a.TxRunner, a.Logger, a.ToolReport(), a.Config.WorkDir)
		if err := optimizer.PrepareWorkDir(a.Config.WorkDir); err != nil {
			a.t.Fatalf("testkit: work dir: %v", err)
		}
	}
	return a.pool
}

// Actor is the user as a policy sees them, for calling a service directly.
func Actor(user *db.User) middleware.Actor {
	return middleware.Actor{ID: user.ID, IsAdmin: user.IsAdmin}
}
