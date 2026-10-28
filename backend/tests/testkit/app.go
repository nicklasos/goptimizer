// Package testkit is what a test starts from:
//
//	func TestStorages_List(t *testing.T) {
//		app := testkit.New(t)
//		admin := app.Create.User(factory.Admin())
//		app.Create.Storage(factory.WithStorageName("Nightly"))
//
//		app.As(admin).GET("/api/v1/storages").
//			AssertOK().
//			AssertJSONPath("data.0.name", "Nightly")
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
	"app/internal/runs"
	"app/internal/secretbox"
	"app/internal/server"
	"app/internal/tools"
	"app/tests"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const JWTSecret = "test-secret-key"

// EncryptionKey seals storage credentials in tests.
var EncryptionKey = []byte("testkit-encryption-key-32-bytes!")

// FakeToolVersions is what app.Tools reports for `<name> --version` at the start of a test.
var FakeToolVersions = map[string]string{
	"pg_dump":   "pg_dump (PostgreSQL) 17.2",
	"mysqldump": "mysqldump  Ver 8.0.36 for Linux on x86_64 (MySQL Community Server - GPL)",
	"sqlite3":   "3.45.1 2024-01-30 16:01:20 e876e51a0ed5c5b3126f52e532044363a014bc594cfefa87ffb5b82257ccalt1",
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

	// Secrets seals and opens stored credentials with the test key
	Secrets *secretbox.Box

	// Tools stands in for the dump programs on PATH: every one is installed until Remove
	Tools *tools.Fake

	t      *testing.T
	server *httptest.Server
	worker *runs.Worker
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
		JWTSecret:     JWTSecret,
		EncryptionKey: EncryptionKey,
		AppName:       "TestApp",
		FrontendURL:   "http://localhost:5173",

		AllowRegistration: true,
		AuthRateLimit:     true,
		TrustedProxies:    []string{"127.0.0.1", "::1"},
		RunnerConcurrency: 2,
		WorkDir:           t.TempDir(),
		RestoreJobs:       2,
	}
	for _, fn := range configure {
		fn(cfg)
	}
	secrets, err := secretbox.New(cfg.EncryptionKey)
	if err != nil {
		t.Fatalf("testkit: %v", err)
	}

	app := &App{
		Ctx:      context.Background(),
		DB:       conn,
		Queries:  db.New(conn),
		Create:   factory.New(t, conn, secrets),
		TxRunner: db.NewTxRunner(conn),
		Config:   cfg,
		Logger:   log,
		Cache:    cache.NewMemoryCache(),
		Mail:     mail.NewMemorySender(),
		Secrets:  secrets,
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
		Config:  a.Config,
		DB:      a.DB,
		Queries: a.Queries,
		Tx:      a.TxRunner,
		Cache:   a.Cache,
		Logger:  a.Logger,
		Mail:    a.Mail,
		Secrets: a.Secrets,
		Tools:   a.Tools,
		Api:     router.Group("/api/v1"),

		WakeWorker: a.Worker().Wake,
	})

	a.server = httptest.NewServer(router)
	a.t.Cleanup(a.server.Close)
	return a.server.URL
}

// Worker is the runner on the test's database, tools and mail. It is not started: Drain runs
// what is queued, synchronously. A test of the start and stop calls Start and registers Stop.
func (a *App) Worker() *runs.Worker {
	if a.worker == nil {
		a.worker = runs.NewWorker(runs.Deps{
			Config:  a.Config,
			Queries: a.Queries,
			Tx:      a.TxRunner,
			Secrets: a.Secrets,
			Tools:   a.Tools,
			Mail:    a.Mail,
			Logger:  a.Logger,
		})
	}
	return a.worker
}

// UseRealTool puts the program from this machine's PATH in app.Tools, for a test that runs a
// real dump. CI installs the ones the tests need.
func (a *App) UseRealTool(name string) {
	a.t.Helper()
	tool, err := tools.System{}.Find(a.Ctx, name)
	if err != nil {
		a.t.Fatalf("testkit: %s is needed on PATH for this test: %v", name, err)
	}
	a.Tools.Install(name, tool.Path, tool.Version)
}

// Actor is the user as a policy sees them, for calling a service directly.
func Actor(user *db.User) middleware.Actor {
	return middleware.Actor{ID: user.ID, Roles: user.Roles}
}
