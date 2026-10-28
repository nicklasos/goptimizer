<p align="center">
  <img src="../docs/images/logo.svg" alt="Gogobackup" width="120">
</p>

<h1 align="center">Gogobackup API</h1>

<p align="center">
  The Go API behind Gogobackup.<br>
  Gin, SQLite, sqlc, Goose, JWT auth with roles.
</p>

<p align="center">
  <img alt="Go" src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white">
  <img alt="Gin" src="https://img.shields.io/badge/Gin-1.10-008ECF?logo=gin&logoColor=white">
  <img alt="SQLite" src="https://img.shields.io/badge/SQLite-pure%20Go-003B57?logo=sqlite&logoColor=white">
  <img alt="sqlc" src="https://img.shields.io/badge/sqlc-type--safe%20SQL-2F7D95">
  <img alt="Swagger" src="https://img.shields.io/badge/docs-Swagger-85EA2D?logo=swagger&logoColor=black">
  <a href="../LICENSE"><img alt="License" src="https://img.shields.io/badge/license-WTFPL-lightgrey"></a>
</p>

<p align="center">
  <a href="#quick-start">Quick start</a> ·
  <a href="#features">Features</a> ·
  <a href="#api-endpoints">API</a> ·
  <a href="#adding-new-modules">Add a module</a> ·
  <a href="ROADMAP.md">Roadmap</a>
</p>

---

There is no ORM and no repository layer. Services call type-safe sqlc queries directly, and every test runs against its own freshly migrated SQLite file. The admin panel is in [../frontend](../frontend), and the [root README](../README.md) covers running and deploying the whole service.

## Quick Start

### Prerequisites
- Go 1.25+, Make. The database is a SQLite file; no server and no cgo.

### Setup
```bash
git clone git@github.com:nicklasos/gogo.git my-api && cd my-api

# Tools: sqlc, air
go mod tidy
make sqlc-install air-install

# Configuration (JWT_SECRET is required)
cp .env.example .env

# A mail catcher in Docker. Skip this if you do not need to read sent mail.
make up

# Creates the database file at DATABASE_PATH (./data/gogobackup.db)
make migrate-up

# Sample accounts (admin@example.com / password123, and more)
make seed

# Run with hot reload
make dev
```

The API listens on http://localhost:8181 and the Swagger UI is at http://localhost:8181/swagger/index.html.

```bash
curl -s localhost:8181/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"password123"}'
```

Run the tests with `make test`. They need no database setup: each test gets its own temporary SQLite file.

## Features

- **Authentication**: JWT auth with login, rotating refresh tokens, profile and password change, and logout (revokes refresh tokens)
- **Password reset and email verification**: single-use emailed links, stored only as hashes
- **Login rate limiting**: per-IP bursts and an escalating lockout per email, with `Retry-After`
- **Safe defaults**: registration closed unless `ALLOW_REGISTRATION=true`, and a weak `JWT_SECRET` is refused in production
- **Roles**: `super-admin`, `admin`, `user` with `RequireRole` middleware; roles are read from the database on every request
- **User management**: super admins manage super admins and admins, admins manage users
- **Mail**: SMTP service (`app.Mail`), logged instead of sent in debug mode, with an in-memory sender for tests
- **Policies**: who may do what to a record, one small `policy.go` per module
- **Factories and seed data**: `app.Create.User(factory.Admin())` in tests, `make seed` for a development database
- **Agent recipes**: step-by-step procedures in `docs/recipes/` for the common changes, also available as Claude Code skills
- **Transactions**: `app.Tx.WithTx(ctx, func(q *db.Queries) error { ... })`
- **Request ID**: `X-Request-ID` on every response and in every `*Context` log line
- **Graceful shutdown** and env-driven CORS
- **Pagination**: `?page` / `?page_size` parsing and a `pagination` block on every list
- **Type Safety**: SQLC for type-safe database operations
- **Swagger**: Auto-generated API documentation
- **Docker and CI**: `docker-compose.yml` for local services, a production `Dockerfile`, and a GitHub Actions workflow
- **Healthcheck**: `/health` checks the database and answers 503 when it is down
- **Scheduler**: cron jobs inside the API process, or in `cmd/cron` with `ENABLE_SCHEDULER=false` (refresh-token cleanup)

## Tech Stack

- **Go 1.25+** + **Gin** - API framework
- **SQLite** + **modernc.org/sqlite** - Database in one file, pure Go driver
- **SQLC** - Type-safe SQL code generation
- **Goose** - Database migrations
- **JWT** - Authentication
- **Swaggo** - API documentation

## Project Structure

```
gogo/
├── cmd/api/main.go              # Application entry point (--port, --test-db)
├── cmd/cli/main.go              # CLI (migrate, create-user, seed, mail, test)
├── internal/
│   ├── app.go                   # App context with DB, Tx, Cache, Logger, Mail
│   ├── server/                  # Engine middleware + the list of modules
│   ├── auth/                    # Authentication module
│   ├── users/                   # User management
│   ├── tools/                   # Finds programs on PATH, and a fake for tests
│   ├── db/queries/              # SQL queries (sqlc)
│   ├── middleware/              # JWT, roles, CORS, request ID, pagination, recovery
│   ├── cache/                   # In-process cache with TTLs
│   ├── mail/                    # SMTP mail service + MemorySender
│   ├── errs/                    # Domain errors
│   └── scheduler/               # Cron jobs
├── migrations/                  # Goose migrations, embedded in the binaries
└── sqlc.yaml
```

## Development Commands

```bash
make help             # List all targets

# Development
make dev              # Hot reload server
make run              # Start API
make run-test-db      # Start API against TEST_DATABASE_PATH
make test             # Run all tests

# Database & SQLC
make migrate-up       # Apply migrations
make sqlc             # Generate SQLC code (run after SQL changes!)

# Documentation
make swagger          # Generate API docs
```

## API Endpoints

### Auth
- `POST /api/v1/auth/register` - Register new user (403 unless `ALLOW_REGISTRATION=true`)
- `POST /api/v1/auth/login` - Login
- `POST /api/v1/auth/refresh` - Refresh token
- `GET /api/v1/auth/me` - Get current user with roles (protected)
- `PUT /api/v1/auth/me` - Update name and email (protected)
- `PUT /api/v1/auth/me/password` - Change password, revokes refresh tokens (protected)
- `POST /api/v1/auth/logout` - Logout and revoke refresh tokens (protected)
- `POST /api/v1/auth/forgot-password` - Email a password reset link (always 200)
- `POST /api/v1/auth/reset-password` - Set a new password with the emailed token
- `POST /api/v1/auth/verify-email` - Confirm an email address with the emailed token
- `POST /api/v1/auth/me/verify-email` - Send the verification email again (protected)

### Users (admin and super admin)
- `GET /api/v1/users?role=user|admin|super-admin` - Paginated list of users with a role
- `POST /api/v1/users` - Create a user with one role
- `PUT /api/v1/users/:id` - Update name and email
- `POST /api/v1/users/:id/set-password` - Set a password, revokes the user's refresh tokens
- `DELETE /api/v1/users/:id` - Delete a user (not yourself)

Admins can only manage accounts whose role is `user`; anything else returns 403 `users.forbidden_role`.

### Other
- `GET /health` - Health check (database); 503 with a per-dependency `checks` map when one is down
- `GET /api/v1/health` - Same health check under API prefix
- `GET /swagger/*` - API documentation

## Environment Variables

```bash
DATABASE_PATH=./data/gogobackup.db            # SQLite file; the directory is created on start
TEST_DATABASE_PATH=./data/gogobackup_test.db  # for --test-db and cli --test; must end in _test.db
JWT_SECRET=your-secret-key-here   # production needs 32+ random characters: openssl rand -hex 32
JWT_ACCESS_TOKEN_TTL=1h
JWT_REFRESH_TOKEN_TTL=720h
ALLOW_REGISTRATION=false          # open POST /auth/register to anyone
AUTH_RATE_LIMIT=true              # throttle login, refresh and emailed-link endpoints
FRONTEND_URL=http://localhost:5173   # base of the links in emails
TRUSTED_PROXIES=127.0.0.1,::1     # whose X-Forwarded-For to believe for the client IP
PORT=8181
APP_ENV=development
APP_NAME=Gogobackup
CORS_ALLOWED_ORIGINS=          # comma-separated; empty or * allows any origin
MAIL_HOST=                     # plus MAIL_PORT, MAIL_SCHEME, MAIL_USERNAME, MAIL_PASSWORD, MAIL_FROM_ADDRESS, MAIL_FROM_NAME
LOG_LEVEL=info
```

## Patterns

### Context Pattern
Handlers start with small helpers that answer the error themselves and return `false`:

```go
userID, ok := middleware.CurrentUserID(c)   // 401 when not signed in
id, ok := middleware.PathID(c, "id")        // 400 when :id is not a positive integer
page, ok := middleware.Page(c)              // 400 when ?page / ?page_size are invalid
```

- `internal.RespondMessage(c, "...")` for actions with nothing to return, `internal.FormatTime(ts)` for timestamps, `internal.NewPaginationMeta(total, page, size)` for lists.
- Handlers do not log errors: a 5xx is logged once by the error middleware with the request ID, and a 4xx is not logged.

### Types.go Pattern
- All request/response types go in `types.go` within each module
- Services define internal types (e.g., `PaginatedUsers`) in service files
- Handlers convert service types to response types from `types.go`

### Cache Pattern
- Use `cache.Remember` for short-lived list responses
- The cache is `cache.MemoryCache`, in the API process: it starts empty on every restart
- Invalidate on create/update/delete

## Auth

- **Tokens**: a short-lived access JWT (`JWT_ACCESS_TOKEN_TTL`) and a single-use refresh token (`JWT_REFRESH_TOKEN_TTL`) that is rotated on every refresh and stored only as a hash. Tokens are read from the `Authorization` header, never from the URL.
- **Emails** are stored lower-case and matched case-insensitively, with a unique index on `LOWER(email)`.
- **Password reset**: `forgot-password` emails `FRONTEND_URL/reset-password?token=...`. The link works once, expires after `PASSWORD_RESET_TTL`, and using it signs out every session. The endpoint answers 200 whether or not the email exists.
- **Email verification**: registration and a changed email send `FRONTEND_URL/verify-email?token=...`. Users expose `email_verified`; accounts created by an admin or the CLI start verified. Nothing is blocked for unverified users by default, so add your own check where a project needs one.
- **Rate limiting**: login allows 60 requests a minute per IP and 40 failures an hour per email and IP, then locks that pair out for 5 minutes, 15 minutes, 1 hour, 6 hours, 24 hours. Emailed-link endpoints allow 10 requests per 10 minutes per IP. A 429 carries `Retry-After` and `details.retry_after_seconds`. Behind a proxy, set `TRUSTED_PROXIES` or every client shares the proxy's IP.
- **Local development**: with `APP_DEBUG=true` emails are written to the log, link included. Or run `make up` and point `MAIL_HOST` at Mailpit (http://localhost:8025).

## Mail

Modules send mail through `app.Mail` (a `mail.Sender`), passed into the service from `routes.go`:

```go
err := s.mail.Send(ctx, mail.Message{
    To:      []string{user.Email},
    Subject: "Welcome",
    Text:    "Plain-text body",
    HTML:    "<p>Optional HTML body</p>", // sent as an alternative when Text is set too
})
```

- Configure SMTP with the `MAIL_*` variables. `MAIL_SCHEME` is `smtp` (STARTTLS, default), `smtps` (implicit TLS) or `none` (plain, for local catchers such as Mailpit).
- With `APP_DEBUG=true`, `Send` only logs the recipient and subject. Nothing leaves the machine.
- `make cli -- mail -to you@example.com` sends a real test email, even in debug mode. `make cli-mail TO=you@example.com` is the same command.
- In tests, `testkit.New` wires a `mail.MemorySender`; assert on `app.Mail.Sent()`.

## Adding New Modules

Follow [docs/recipes/add-module.md](docs/recipes/add-module.md). In short:

1. Create migration: `migrations/XXX_create_table.sql`
2. Write SQL queries in `internal/db/queries/module.sql`
3. Generate code: `make sqlc`
4. Create module: `internal/module/{types,policy,service,handler,routes}.go`
5. Add `module.RegisterRoutes(app)` to `internal/server/server.go` (used by the API and the test server)
6. Add a factory in `internal/factory` and tests

## Docker

```bash
make up            # Mailpit
make down
make docker-build  # production image: api and cli binaries
docker compose --profile app up --build   # everything, API included
```

The image keeps the database at `/data/db/gogobackup.db` (mount a volume there) and runs `cli migrate up` before the API starts (`RUN_MIGRATIONS=false` turns that off). The API runs the scheduled jobs itself unless `ENABLE_SCHEDULER=false`, in which case run `/app/cron` as a second container.

## Test database

`go test` needs no database: every test copies a migrated SQLite file into its own temporary directory.

`TEST_DATABASE_PATH` is the database of `--test-db` (api) and `--test` (cli), which the frontend E2E suite runs against (`make test-db-setup` migrates it). Its file name must end in `_test.db`; anything else is refused.

## Error Handling

The project uses structured error handling with the `errs` package. See `docs/ERRORS.md` for complete guide and examples.

**Quick reference:**
- Services return domain errors: `errs.NewNotFoundError(key, message)`
- Handlers use: `errs.RespondWithError(c, err)` or `errs.RespondWithValidationError(c, err)`
- DB helpers: `errs.WrapDatabaseError`, `errs.DomainErrorFromUniqueViolation`
- See `internal/users/` for a complete module

## Links

- **API Docs**: `/swagger/index.html` when running
- **Error Handling**: See `docs/ERRORS.md` for error handling guide
- **Architecture**: See `AGENTS.md` for detailed patterns
- **Deployment**: See `../deploy/README.md`
- **Roadmap**: See `ROADMAP.md` for what is worth adding next
