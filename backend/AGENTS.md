# Gogo – Agent Guide

## Start here
- **Recipes**: `docs/recipes/` has step-by-step procedures for the common changes: add a module, add an endpoint, change the schema, add a scheduled job. Use the matching one instead of improvising.
- **Reference module**: `internal/users`. Copy its shape.
- **Finish every task with `make test`** and fix what fails. Do not start the API; the owner runs it.
- **The kit**: this repo is the API. [gogo-front](https://github.com/nicklasos/gogo-front) is its admin UI and generates its TypeScript types from `docs/swagger.json`, so run `make swagger` after changing request or response types.

## Architecture

### Core Philosophy
- **Clean separation of concerns** with clear layer responsibilities
- **Simplicity over complexity** - avoid unnecessary abstractions
- **Type-safe database operations** using sqlc
- **Environment-driven configuration** - no hardcoded secrets

### Directory Structure
```
gogo/
├── cmd/api/main.go              # Main application entry (--port, --test-db)
├── cmd/cli/main.go              # CLI (migrate, create-user, seed, mail, smoke test); global --test flag
├── config/
│   ├── config.go                # Env → Config
│   └── testdb.go                # Test-database safety guard
├── internal/
│   ├── app.go                   # App context with DB, Tx, Cache, Logger, Mail, AuthMiddleware
│   ├── server/server.go         # NewEngine (middleware) + RegisterRoutes (the module list)
│   ├── auth/                    # Authentication module
│   │   ├── auth_service.go      # Business logic
│   │   ├── handlers.go          # HTTP handlers
│   │   ├── routes.go            # Route registration (sets app.AuthMiddleware)
│   │   └── types.go             # Request/response types
│   ├── users/                   # User management (super admins, admins, users)
│   ├── tools/                   # Finds programs on PATH and their versions (System), or pretends to (Fake)
│   ├── health/                  # /health and /api/v1/health
│   ├── db/
│   │   ├── tx.go                # TxRunner.WithTx transaction helper
│   │   └── queries/             # SQL queries (incl. technical Healthcheck)
│   ├── middleware/
│   │   ├── user_auth.go         # JWT authentication, loads roles from DB
│   │   ├── require_role.go      # Roles + RequireRole / RequireAnyRole
│   │   ├── cors.go              # CORS from CORS_ALLOWED_ORIGINS
│   │   ├── logging.go           # RequestID, Recovery, ErrorHandler
│   │   ├── request.go           # CurrentUserID, PathID, Page: handler helpers that answer the error themselves
│   │   └── pagination.go        # ?page / ?page_size parsing
│   ├── cache/                   # MemoryCache: in-process, with TTLs
│   ├── mail/                    # SMTP Service + MemorySender (tests)
│   ├── errs/                    # Domain errors + WrapDatabaseError
│   ├── scheduler/               # Cron jobs, run by the API or cmd/cron
│   └── responses.go             # PaginationMeta helper
├── migrations/                  # Goose migrations (SQLite), embedded in the binaries
└── Makefile                     # Development commands (make help)
```

### Layer Responsibilities
- **Routes**: Dependency injection, receives `*internal.App` and creates services/handlers with specific dependencies
- **Handlers**: HTTP request/response, basic validation, JSON serialization, receives only needed services
- **Services**: Business logic, input validation, complex workflows, uses sqlc directly
- **Queries**: SQL queries managed by sqlc, type-safe database operations

## Technology Stack
- Go 1.25+
- Gin
- SQLite (modernc.org/sqlite, pure Go: no cgo)
- sqlc - Type-safe SQL code generation
- Goose - Database migrations
- Swaggo - Swagger documentation
- JWT - Authentication
- Testify - Testing framework

## Module Pattern
When adding new modules:

```go
// internal/orders/
├── handler.go           # HTTP endpoints - receives only needed services
├── order_service.go     # Business logic
├── routes.go           # Route registration - receives *internal.App, handles DI
└── types.go            # All request/response types
```

### Dependency Injection Pattern
- **Routes** (`routes.go`): Only layer that knows about `*internal.App`
- **Handlers**: Receive specific services they need (e.g., `*OrderService`)
- **Services**: Receive specific dependencies (e.g., `*db.Queries`, logger, cache)
- **Auth**: `auth.RegisterRoutes(app)` sets `app.AuthMiddleware`; protect a route group with `group.Use(app.AuthMiddleware)`
- **Registration**: every module is listed once in `internal/server/server.go` `RegisterRoutes`. The API binary and the test server both call it, so a new module is one line there.

## Roles
- Roles live in `users.roles`, a JSON array in a TEXT column read as `db.Roles` (a `[]string`): `super-admin`, `admin`, `user` (constants in `middleware/require_role.go`).
- `UserAuthMiddleware` loads roles from the database on every request; never read roles from JWT claims.
- Guard routes with `middleware.RequireRole(middleware.RoleAdmin)` or `RequireAnyRole(...)`. A super admin passes every role check.
- In handlers use `middleware.GetUserRolesFromContext(c)`.
- Who manages whom is one rule, `users.CanManage`: super admins manage everyone, admins manage only accounts whose every role is `user`.
- The first super admin is created with `make cli-create-user EMAIL=... PASSWORD=...`.

## Policies
- Who may do what to a record lives in the module's `policy.go` as pure functions: `CanView(actor middleware.Actor, record db.X) bool`.
- A handler gets the actor with `middleware.CurrentActor(c)` and passes it to the service. The service loads the record, calls the policy, and returns a 403 domain error on a no (404 stays for "does not exist").
- Look records up by ID alone and let the policy decide; do not hide the rule inside a `WHERE user_id = ...`.
- `middleware.RequireRole` on a route group is for "this whole area is admins only". A policy is for rules that depend on the record.
- gogo-front mirrors each policy in `features/<module>/policy.ts` to hide what is not allowed. The API stays the authority.
- `internal/users/policy.go` is the reference; `tests/unit/policy_test.go` shows how to test one. A module that only admins reach needs no policy: `RequireRole` on its route group is the rule.

## Factories and seed data
- `internal/factory` creates records with defaults. In a test it is `app.Create`: `app.Create.User()`, `app.Create.User(factory.Admin(), factory.Unverified())`. Elsewhere: `factory.New(t, conn)`.
- Every new table gets a factory. Factories write through the `Factory*` queries in `internal/db/queries/factory.sql`; tests still never write `INSERT` statements by hand.
- Users from a factory have the password `factory.DefaultPassword` and a verified email unless `factory.Unverified()` is passed.
- `make seed` fills an empty development database through the same factories (`cmd/cli/internal/commands/seed.go`); add a new module's sample data there.

## Transactions
Services that need more than one write take `*db.TxRunner` (from `app.Tx`) and use:
```go
err := s.tx.WithTx(ctx, func(q *db.Queries) error {
    // use q, not s.queries; return an error to roll back
})
```
Transactions take SQLite's write lock when they begin (`_txlock=immediate`), so inside `fn` use only `q`: a write through `s.queries` waits for that lock and fails after the busy timeout.

## Auth
- Settings come from `config` and reach the module as `auth.Options` in `auth/routes.go`.
- Emailed links (password reset, email verification) are rows in `auth_tokens`: single-use, expiring, stored as SHA-256 hashes. Issue with `issueEmailToken`, redeem with `consumeEmailToken` inside `WithTx`.
- `forgot-password` must never reveal whether an email exists: no error, no different status.
- `middleware.AuthRateLimit` guards login (call `middleware.MarkAuthSuccess(c)` on success); `middleware.RateLimit(cache, log, scope, limit, window)` is the generic per-IP limiter for anything else.
- A 401 from an authenticated route makes clients try a token refresh, so use 400 or 403 for anything that is not "this session is invalid".
- Registration is a 403 unless `ALLOW_REGISTRATION=true`.
- Pass every email that enters the system through `internal.NormalizeEmail` before storing or looking it up.
- Refresh tokens and emailed tokens are stored as hashes (`hashToken`); the raw value exists only in the response or the email.
- `/health` reports the database. Add a check there for each new dependency the API cannot serve without.

## Scheduler
- By default the API process runs the cron jobs: `cmd/api/main.go` starts `internal/scheduler` on boot and stops it on shutdown.
- `ENABLE_SCHEDULER=false` turns that off, and `cmd/cron` (`bin/gogo-cron`) runs the same jobs as its own process. Run one or the other, never both.
- `testkit` never starts it; test a job by calling its `Execute` (`docs/recipes/add-scheduled-job.md`).

## Database
- One SQLite file at `DATABASE_PATH` (default `./data/gogobackup.db`). Every connection turns on foreign keys, WAL and a 5 s busy timeout (`internal/db/connection.go`).
- Timestamps are TEXT in UTC. Compare a stored timestamp with now through `datetime()` on both sides (`datetime(expires_at) > datetime('now')`): Go and `CURRENT_TIMESTAMP` write different formats.
- Nullable columns are `sql.NullString`, `sql.NullTime`, `sql.NullBool`; IDs are `int64`; a missing row is `sql.ErrNoRows`.
- Pass the request context into services so queries are cancelled with the request.
- Every query goes through sqlc, in `internal/db/queries`, including the factories. Hand-written SQL is
  only for testkit's generic `AssertDatabase*` helpers and `*_test.go` files that create states the app
  forbids (`tests/unit/raw_sql_test.go` enforces it).

## Mail
- Services take a `mail.Sender` (from `app.Mail`) and call `Send(ctx, mail.Message{To, Subject, Text, HTML})`.
- `Send` only logs when `APP_DEBUG=true`; `mail.Service.SendLive` always dials SMTP (used by `make cli-mail TO=...`).
- Tests get a `mail.MemorySender` from `testkit.New`; assert on `app.Mail.Sent()`. Never dial SMTP from a test.
- Send after the database work has committed, not inside `WithTx`.

## Context Patterns

### User ID from Context
`userID, ok := middleware.CurrentUserID(c)` in a handler; when `ok` is false it has already answered 401, so just return.

### Request ID
`middleware.RequestID` puts an ID on the request context and the `X-Request-ID` response header. Log with the `*Context` methods (`logger.ErrorContext(c.Request.Context(), ...)`) and the ID is added to the record automatically.

### Pagination from Context
`page, ok := middleware.Page(c)` gives `page.Page`, `page.PageSize` and `page.Offset()` with the standard limits (20 by default, 100 at most). `middleware.GetPaginationParamsFromContext(c, default, min, max)` is there for an endpoint that needs other limits.

### Path parameters
`id, ok := middleware.PathID(c, "id")` parses a positive integer and answers 400 otherwise.

### Handler shape
Keep handlers to: helpers above, bind the body (`errs.RespondWithValidationError` on failure), call the service, `errs.RespondWithError(c, err)` on failure, map to the response type with one small function per module. `internal/users/handler.go` is the reference.
- Do not log errors in handlers. `RespondWithError` hands every 5xx to the error middleware, which logs it once with the request ID; 4xx are not logged.
- `internal.RespondMessage(c, "...")` for actions with nothing to return; `internal.FormatTime(ts)` for timestamps.

### List caching
A list cached with `cache.Remember` gets a version in its key, changed on every write. Never try to enumerate the keys to forget.

## Types.go Pattern

### Rule
- **All request/response types** go in `types.go` within each module
- Swagger marks every field required (`--requiredByDefault`), because gogo-front generates its TypeScript types from it. Tag a field that may be absent with `validate:"optional"`, and an enum with `enums:"a,b,c"`.
- **Service types** (e.g., `PaginatedUsers`) are defined in service files
- **Handlers** use types from `types.go` for requests/responses
- **Services** use internal types and convert to handler types

## Database Management

### Migration Creation Process
```bash
# Create migrations manually with sequential numbering
# Format: migrations/001_description.sql, 002_description.sql, etc.

# Apply migrations
make migrate-up

# Generate sqlc after schema changes
make sqlc
```

### Migration Naming Convention
- **Format**: `001_description.sql`, `002_description.sql`, etc.
- **Location**: `migrations/` directory
- **Always include timestamps**: `created_at`, `updated_at` with `DEFAULT CURRENT_TIMESTAMP`

## Development Commands
```bash
make help             # List targets
make run              # Start server
make run-test-db      # Start against TEST_DATABASE_PATH
make build            # Build binary
make test             # Run all tests (each test gets its own SQLite file)
make seed             # Accounts and sample data for an empty development database
make cli migrate up   # Any console command; flags go after -- (make cli -- create-user --email a@b.c --password secret123)
make cli-create-user EMAIL=a@b.c PASSWORD=secret123   # Create a user (default role super-admin)
make migrate-up       # Apply migrations
make sqlc             # Generate sqlc code
make swagger          # Generate API docs (then run `make api-types` in `../frontend`)
make up / make down   # Mailpit in Docker
```

## Code Conventions
- **Handlers**: `GetUser`, `CreateUser`, `ListUsers`
- **Services**: `UserService`, `OrderService`
- **SQL queries**: `GetUserByID`, `CreateUser`, `ListUsers`
- **Files**: `user_service.go`, `order_handler.go`
- **Cache keys**: `user:123`, `users:v<version>:page:1:size:20`

## Key Principles
1. **Dependency Injection via Routes** - Only `routes.go` knows about `*internal.App`
2. **Handlers receive specific services** - No direct access to `*internal.App`
3. **Services own business logic** - Keep handlers thin
4. **Use sqlc directly** - No repository abstraction
5. **Environment-driven config** - No hardcoded values
6. **Module-based organization** - Self-contained domains
7. **Context patterns** - Use middleware for user ID and pagination
8. **Types.go pattern** - All request/response types in types.go
9. **Cache where useful** - `Remember` + invalidate on writes

## Testing Framework

### Laravel-Style Database Testing
- **A database per test** - `testkit.New` copies a migrated SQLite file into `t.TempDir()`; the schema is migrated once per test binary. Nothing to set up and nothing shared between tests.
- **Real Database Testing** - Uses actual SQLite (no mocking)
- **Test Database Guard** - `config.AssertTestDatabasePath` refuses a `TEST_DATABASE_PATH` whose file name does not end in `_test.db`. It guards `--test-db` (api) and `--test` (cli), which the frontend E2E suite uses.
- **Same wiring as production** - the test server uses `server.NewEngine` and `server.RegisterRoutes`, and starts on the first request
- **MemoryCache** - Tests get their own empty cache, as the API does on every start
- **MemorySender** - Tests collect mail in memory (`app.Mail.Sent()`)

### Test Patterns
Every test starts from `testkit.New(t)` (`tests/testkit`). `tests/README.md` is the cheat sheet: requests, response assertions, database assertions, factories.

```go
func TestUsersAPI_UpdateUser(t *testing.T) {
    app := testkit.New(t)
    admin := app.Create.User(factory.Admin())
    record := app.Create.User()
    body := users.UpdateUserRequest{Name: "Ada", Email: record.Email}

    app.As(admin).PUT("/api/v1/users/%d", body, record.ID).
        AssertOK().
        AssertJSONPath("data.name", "Ada")
    app.PUT("/api/v1/users/%d", body, record.ID).AssertUnauthorized()
}

func TestUserService_Update(t *testing.T) {
    app := testkit.New(t)
    admin := app.Create.User(factory.Admin())
    record := app.Create.User()

    result, err := users.NewUserService(app.Queries, app.TxRunner).Update(app.Ctx, testkit.Actor(admin), record.ID, users.UpdateUserRequest{Name: "Ada", Email: record.Email})

    require.NoError(t, err)
    assert.Equal(t, "Ada", result.Name)
}
```

- Assert responses with the `Assert*` methods and rows with `app.AssertDatabaseHas/Missing/Count`; decode with `.JSON(&v)` only when the typed struct is needed.
- No `// Setup`, `// Test`, `// Assert` comments, no hand-written `INSERT`, no per-test server or `defer`.

## What We DON'T Use
- NO Repository Pattern - Services use sqlc directly
- NO ORM - Raw SQL with sqlc for type safety
- NO complex abstractions - Keep it simple
- NO Test Mocking of DB - Real database with transaction rollback
