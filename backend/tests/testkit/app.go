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
	"app/internal/server"
	"app/tests"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const JWTSecret = "test-secret-key"

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

	t      *testing.T
	server *httptest.Server
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
		FrontendURL: "http://localhost:5173",

		AllowRegistration: true,
		AuthRateLimit:     true,
		TrustedProxies:    []string{"127.0.0.1", "::1"},
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
		Api:     router.Group("/api/v1"),
	})

	a.server = httptest.NewServer(router)
	a.t.Cleanup(a.server.Close)
	return a.server.URL
}

// Actor is the user as a policy sees them, for calling a service directly.
func Actor(user *db.User) middleware.Actor {
	return middleware.Actor{ID: user.ID, Roles: user.Roles}
}
