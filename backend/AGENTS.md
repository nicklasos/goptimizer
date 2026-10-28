# Gogo – Agent Guide

## Start here
- **Recipes**: `docs/recipes/` has step-by-step procedures for the common changes: add a module, add an endpoint, change the schema, add a scheduled job. Use the matching one instead of improvising.
- **Reference module**: `internal/storages`. Copy its shape.
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
├── cmd/cli/main.go              # CLI (migrate, create-user, self-backup, seed, mail, smoke test); global --test flag
├── cmd/screenshots-seed/        # Demo data for the README screenshots; scripts/screenshots/shoot.mjs runs it
├── config/
│   ├── config.go                # Env → Config
│   └── testdb.go                # Test-database safety guard
├── internal/
│   ├── app.go                   # App context with DB, Tx, Cache, Logger, Mail, Secrets, Tools, AuthMiddleware
│   ├── server/server.go         # NewEngine (middleware) + RegisterRoutes (the module list)
│   ├── auth/                    # Authentication module
│   │   ├── auth_service.go      # Business logic
│   │   ├── handlers.go          # HTTP handlers
│   │   ├── routes.go            # Route registration (sets app.AuthMiddleware)
│   │   └── types.go             # Request/response types
│   ├── users/                   # User management (super admins, admins, users)
│   ├── storages/                # Where backups go: kinds registry (kinds.go), service, handlers
│   │   └── driver/              # Driver interface (Put/Open/List/Delete/Close), local, S3, GCS, Probe
│   ├── plans/                   # Backup plans: sources registry (sources.go), schedules (NextRun, PrevRun), health (PlanHealth), dashboard, source tests, dumps (dump.go), restores (restore.go, restore_databases.go, extract.go)
│   ├── runs/                    # The runner: run and restore lifecycle, worker, fan-out upload, retention (Prune), run, archive and restore endpoints, downloads
│   ├── runlog/                  # A run's log: timestamped lines, capped at 1 MB
│   ├── tools/                   # Finds pg_dump, mysqldump, sqlite3 on PATH (System) or pretends to (Fake)
│   ├── selfbackup/              # Ensure: the plan that backs up DATABASE_PATH, for `cli self-backup`
│   ├── secretbox/               # AES-256-GCM sealing of credentials with ENCRYPTION_KEY
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
│   ├── scheduler/               # Cron jobs and the backup worker, run by the API or cmd/cron
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
- `internal/users/policy.go` is the reference; `tests/unit/policy_test.go` shows how to test one. A module that only admins reach, such as `storages`, needs no policy: `RequireRole` on its route group is the rule.

## Factories and seed data
- `internal/factory` creates records with defaults. In a test it is `app.Create`: `app.Create.User()`, `app.Create.User(factory.Admin(), factory.Unverified())`, `app.Create.Storage(factory.LocalStorage(dir))`, `app.Create.Plan(factory.SQLiteSource(path), factory.WithStorages(storage))`. Elsewhere: `factory.New(t, conn, secrets)`, where `secrets` seals storage credentials.
- Every new table gets a factory. Factories write through the `Factory*` queries in `internal/db/queries/factory.sql`; tests still never write `INSERT` statements by hand.
- Users from a factory have the password `factory.DefaultPassword` and a verified email unless `factory.Unverified()` is passed.
- `make seed` fills an empty development database through the same factories (`cmd/cli/internal/commands/seed.go`); add a new module's sample data there.

## Storages
- A storage kind (`local`, `s3`, `gcs`) is one entry in the `kinds` registry in `internal/storages/kinds.go`: its request block, the fields of its credentials, how to open its driver, and how to show its config. Adding a kind is one entry and one driver, not a new branch in the service.
- `config` holds the kind's settings as JSON; `secret` holds its credentials as JSON sealed by `app.Secrets` (`internal/secretbox`). No response type carries a credential: responses say `secret_set`.
- Drivers take keys relative to the storage's directory or bucket prefix and refuse `..` and absolute keys (`driver.CleanKey`); the local driver goes through `os.Root`, so a symlink cannot lead out either. `Put` and `Open` stream: phases 5 and 7 send archives of any size through them.
- `StorageService.Driver(ctx, storage)` opens a saved storage for other modules. Whoever opens a driver closes it (`Close`): a GCS driver holds its own client.
- `storages.Unreachable(err)` is the 502 `storages.unreachable` for a storage that failed, with the reason in `details.reason`.
- Tests run offline: `testkit.NewFakeS3(t)` (gofakes3 on a loopback port) and `testkit.NewFakeGCS(t)` (fake-gcs-server in process). `app.Create.Storage(factory.S3Storage(...))` seals credentials with the test key.

## Plans
- A plan is what to back up (a source), when (schedules), where to (one or more storages) and how long to keep it (retention). `internal/runs` runs them (see Runner).
- A source kind (`postgres`, `mysql`, `sqlite`, `files`) is one entry in the `sources` registry in `internal/plans/sources.go`: its request block, its password field, how to show its config, how to test it, how to dump it and the extension of its archives. Like storage kinds, `source_config` is JSON and `source_secret` is the password sealed by `app.Secrets`; responses say `secret_set`.
- Schedules are `[]plans.Schedule` (`{time: "HH:MM" UTC, days: [...]}`, no days is every day), stored as JSON. `plans.NextRun(schedules, after)` says when a plan runs next, and the admin panel's `nextRun` runs the same cases (`tests/unit/testdata/next_run.json`). `plans.PrevRun(schedules, at)` is the latest slot at or before `at`, which the scheduler runs (`testdata/prev_run.json`; the panel has no port of it).
- Plan responses carry `last_run` (the latest run, any status), `last_success_at` and `last_size_bytes` (that backup's size), `folder` (`plans.Folder`, the `<plan-slug>/` its archives go in) and `health`.
- Health is one pure function, `plans.PlanHealth` (table test in `tests/unit/health_test.go`). The first that applies wins: `restoring` (a restore is queued or running), `running` (the latest run is queued or running), `failed` (it failed), `paused` (disabled), `overdue` (a slot at least an hour old and later than the last save has no successful run at or after it), `never` (no runs), `ok`. The admin panel colours cards from it and has no copy of the rule.
- `GET /dashboard` is every plan, not paginated, as a plan response plus `days`: the last 30 UTC days, oldest first, each with how many runs that started that day succeeded and failed (`CountFinishedRunsPerDay`, one grouped query for all plans), and a `summary` of plans per health.
- `PUT /plans/:id/enabled` pauses or resumes a plan. Only a change counts as a save, and resuming is one, so the scheduler does not run the slots missed while paused.
- `plan_storages` links plans to storages. Deleting a plan removes its links; deleting a storage that a plan uses is a 409 `storages.in_use` listing the plans.
- A source test connects (Postgres with pgx, MySQL with go-sql-driver) or opens the files, and looks for the dump program through `app.Tools`. Tests get `tools.Fake` as `app.Tools`: every program is installed until `app.Tools.Remove(name)`.
- `plans.PgDumpCompatible` is the rule that pg_dump must be at least the server's major version.

## Runner
- A run is a row in `runs` that goes `queued` → `running` → `succeeded` or `failed` (`runs.Status`). Every step is one conditional UPDATE on the current status (`runs.Claim`, `runs.Finish`), so two workers or two processes can never both start a run. `Claim` also refuses while another run or a restore of the same plan is running: a plan runs one job at a time (see Restore).
- Runs are queued two ways. "Run now" (`POST /plans/:id/run`, 202) inserts a manual run, or answers 409 `plans.run_in_progress` when the plan has one queued or running. The scheduler's `enqueue-runs` job runs at the start of every minute and queues, per enabled plan, `runs.Due`: the latest slot at or before now (`plans.PrevRun`) when it is later than both the last queued slot and the plan's `created_at`. After downtime that is the latest missed slot, once. A partial unique index on `(plan_id, scheduled_for)` makes a second insert of a slot a no-op, whoever tries.
- `runs.Worker` runs them and restores, at most `RUNNER_CONCURRENCY` (default 2) jobs at once. Its queue is `ListClaimableJobs`: queued runs and restores, oldest first, of plans with nothing running; `claimers` maps each job kind to how it is claimed. It lives in the process that runs the scheduler and starts and stops with it: the API by default, `cmd/cron` with `ENABLE_SCHEDULER=false`. It wakes at once for a run this process queued (`app.WakeWorker`) and polls every 5 s for the rest, such as a manual run the API queued while the worker is in `cmd/cron`.
- On start it makes `WORK_DIR` and fails every run and restore left `running` with "interrupted: the service restarted"; on stop it cancels the jobs in progress, which kills their programs, and records them as failed with "interrupted: the service stopped".
- A run dumps the source once (`PlanService.Dump`) into a `runs.FanOut`, which writes the stream to every storage of the plan at once, one pipe and one `Put` per storage, and counts and hashes it on the way. No temp file holds the archive. A storage that fails is dropped and the others carry on; the run succeeds only when the dump succeeded and every storage stored it. Each storage that stored it gets an `archives` row with the size and sha256. The object key is `<plan-slug>/<plan-slug>-<UTC start yyyymmddTHHMMSSZ>-<run id>.<ext>` (`runs.ObjectKey`).
- After a run that succeeded, retention runs per storage of the plan: `runs.Prune(archives, policy, now)` is pure and returns the archives to remove (table tests in `tests/unit/runs_test.go`). It never removes the newest archive. Deleting removes the object, then sets `deleted_at`; an object already gone counts as deleted, and one that cannot be deleted keeps its row for the next run to try again.
- The log is a `runlog.Log`: timestamped lines, the last 1 MB kept. Dump programs' stderr goes into it. It is written to `runs.log` every 2 s while the run goes on and at the end, so `GET /runs/:id` shows it live.
- A failed run emails every admin and super admin with a verified email: the error, the last 40 log lines and a link to the plan's page (`/plans/:id`). It is sent after the run is recorded.
- Run responses carry `duration_ms` once the run finished.
- Archives download two ways, both streaming from the storage with `Content-Disposition: attachment` and `Content-Length` from the archive's size. `GET /archives/:id/download` takes the admin's bearer token, for scripts. A browser cannot send that header on a plain navigation, so `POST /archives/:id/download-link` gives an admin a token that `GET /downloads/:token` accepts without a session for one minute (`runs.ArchiveService`: the archive id and expiry, sealed by `app.Secrets`). The browser then saves an archive of any size straight to disk. A deleted archive is 404 `archives.not_found`, a bad or expired token 403 `archives.invalid_link`, a storage that fails 502 `storages.unreachable`.
- Dump programs come from `app.Tools` (`tools.Finder`). Their passwords never go in arguments: `PGPASSWORD` in pg_dump's environment, a 0600 `--defaults-extra-file` for mysqldump, removed after.
- **Adding a source kind's dump** is one `dump` function and one `extension` in its `sources` entry. The function writes the compressed archive to `w`, logs its steps, sends the program's stderr to `log.Writer(name + ": ")` (`runTool` does that) and returns when the archive is complete. Nothing outside `internal/plans` changes.
- Tests drive the worker with `app.Worker().Drain(ctx)`, which runs everything queued synchronously, and enqueue slots with `runs.NewRunService(...).EnqueueDue(ctx, clock)`. `tools.Fake` installs a real program with `app.UseRealTool("sqlite3")` or a script with `app.Tools.Install(name, path, version)`. Postgres and MySQL dumps of real servers run when `TEST_POSTGRES_URL` and `TEST_MYSQL_URL` are set (CI sets both); otherwise they skip.
- Factories: `app.Create.Run(plan, factory.WithRunStatus(runs.StatusRunning), factory.StartedAt(t), factory.RunSize(n))`, `app.Create.Archive(run, storage, factory.ArchiveCreatedAt(t), factory.ArchiveDeleted())`.

## Restore
- A restore is a row in `restores`: the archive, the plan, `target_config` (the target as JSON, no password), `target_secret` (a password typed for it, sealed; NULL uses the plan's), the same status machine as runs, the log and who started it. `runs.ClaimRestore` / `runs.FinishRestore` are the steps; `ClaimRestore` refuses while a run or another restore of the plan is running, and `Claim` refuses while a restore runs, so a backup never dumps a half restored database (`tests/unit/restore_test.go`, `TestClaim_OneJobPerPlan`).
- `POST /archives/:id/restore` (202) checks the request with `plans.ParseRestoreTarget` and queues it. The body has the block of the plan's kind, like a plan request. Postgres and MySQL blanks (host, port, user, ssl_mode, password) are the plan's; `create_database` defaults to true for a database other than the plan's. SQLite takes a path; files take an absolute `directory` or `original_locations: true`. A deleted archive is 404 `archives.not_found`, a second restore of a plan with one queued or running 409 `restores.in_progress`, an archive of another source kind than the plan's now 409 `restores.kind_changed`. A run queued meanwhile simply waits.
- Restoring over the plan's own source needs `confirm`: the database name, the SQLite path or `original locations` (`plans.RestoreTargetIsOwn`: same server and database, where localhost, 127.x and ::1 are one server; the same path; the original locations). Otherwise it is a 400 field error `restores.confirm_mismatch` on `confirm`. The admin panel runs the same cases (`tests/unit/testdata/own_target.json`). The SQLite target can never be this service's own `DATABASE_PATH` (`restores.own_database`).
- `GET /archives/:id/restore-target` is the target a form starts from: the plan's own source, or for files `<WORK_DIR>/restores/<plan-slug>-<UTC time>`. `GET /restores/:id` has the log, written every 2 s like a run's; `GET /plans/:id/restores` lists them. A failed restore emails admins like a failed run ("Restore failed: <plan>", a link to the restore).
- Each restore works in a directory of its own under `WORK_DIR` (default `./data/work`), removed after. The archive is read through `plans.Archive`, which checks its size and sha256: `download` saves a verified copy, `stream` fails the last read on a mismatch.
  - Postgres: connect to the target (pgx), create the database from `postgres` when it is missing and allowed, download, read `pg_restore --list` for the versions (`plans.PgRestoreCompatible`: pg_restore at least the archive's pg_dump, the server at least the dumped one), then `pg_restore --clean --if-exists --no-owner --no-acl --exit-on-error --verbose --jobs RESTORE_JOBS` with `PGPASSWORD` in its environment.
  - MySQL: check or create the database, then download and verify the archive, and gunzip it into `mysql --database` with the 0600 defaults file of the dump. mysql applies statements as they arrive, so it must never see an unverified archive.
  - SQLite: gunzip next to the target, check the sha256 and `PRAGMA integrity_check`, move the current file and its `-wal`/`-shm` to `<path>.before-restore-<UTC time>`, then rename the new one into place.
  - Files: download and verify first, so a corrupt archive touches nothing, then extract through `os.Root` (`plans.ExtractLayout`): a name that is absolute or has `..` fails the restore, a symbolic link that would lead out of the target (or, for the original locations, out of the plan's paths) is skipped with a warning, and nothing is written through a link. Modes and mtimes are kept, owners and device files are not.
- **Adding a source kind's restore** is one `restoreKind` in its `sources` entry: its request block (a type with `resolve`), its `defaults`, `respond`, the `own` rule (and its cases in `own_target.json`, which the panel mirrors in `features/plans/restore.ts`) and the `restore` function, which reads `job.Archive` through `stream` or `download`, logs its steps and runs its program through `runTool`.
- Factory: `app.Create.Restore(archive, factory.WithRestoreStatus(runs.StatusRunning), factory.RestoreTarget(plans.SQLiteTarget{...}), factory.RestoredBy(user))`. Postgres and MySQL restores of real servers run in `tests/integration/database_dumps_test.go` when `TEST_POSTGRES_URL` and `TEST_MYSQL_URL` are set.

## Self-backup
- `cli self-backup --dir DIR` (`internal/selfbackup.Ensure`) makes sure a plan backs up the service's own `DATABASE_PATH`; `deploy/` runs it on every release. It finds the plan by its source (`ListSQLitePlansByPath`), not its name, and leaves a found plan untouched. Without one it reuses a local storage at `DIR` (`ListLocalStoragesByPath`) or creates "Gogobackup itself (local)", then creates "Gogobackup itself". Both go through `StorageService` and `PlanService` after the binding validator, as an API request would.
- The command prints `Created ...` only when it created something; the deploy task reads that as a change.

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
- By default the API process runs the cron jobs and the backup worker: `cmd/api/main.go` starts `internal/scheduler` on boot and stops it on shutdown.
- `ENABLE_SCHEDULER=false` turns that off, and `cmd/cron` (`bin/gogo-cron`) runs the same jobs and the worker as its own process. Run one or the other, never both.
- `testkit` never starts it; test a job by calling its `Execute` (`docs/recipes/add-scheduled-job.md`).

## Database
- One SQLite file at `DATABASE_PATH` (default `./data/gogobackup.db`). Every connection turns on foreign keys, WAL and a 5 s busy timeout (`internal/db/connection.go`).
- Timestamps are TEXT in UTC. Compare a stored timestamp with now through `datetime()` on both sides (`datetime(expires_at) > datetime('now')`): Go and `CURRENT_TIMESTAMP` write different formats.
- Nullable columns are `sql.NullString`, `sql.NullTime`, `sql.NullBool`; IDs are `int64`; a missing row is `sql.ErrNoRows`.
- Pass the request context into services so queries are cancelled with the request.
- Every query against Gogobackup's own database goes through sqlc, in `internal/db/queries`, including the
  factories. Hand-written SQL is only for the databases Gogobackup backs up or restores into
  (`internal/plans/probe.go`, `restore_databases.go`), testkit's generic `AssertDatabase*` helpers, and
  `*_test.go` files that create states the app forbids.

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
Keep handlers to: helpers above, bind the body (`errs.RespondWithValidationError` on failure), call the service, `errs.RespondWithError(c, err)` on failure, map to the response type with one small function per module. `internal/storages/handler.go` is the reference.
- Do not log errors in handlers. `RespondWithError` hands every 5xx to the error middleware, which logs it once with the request ID; 4xx are not logged.
- `internal.RespondMessage(c, "...")` for actions with nothing to return; `internal.FormatTime(ts)` for timestamps.

### List caching
A list cached with `cache.Remember` gets a version in its key, changed on every write. Never try to enumerate the keys to forget.

## Types.go Pattern

### Rule
- **All request/response types** go in `types.go` within each module
- Swagger marks every field required (`--requiredByDefault`), because gogo-front generates its TypeScript types from it. Tag a field that may be absent with `validate:"optional"`, and an enum with `enums:"a,b,c"`.
- **Service types** (e.g., `PaginatedStoragesResult`) are defined in service files
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
- **Cache keys**: `user:123`, `storages:v<version>:page:1:size:20`

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
func TestStoragesAPI_GetStorage(t *testing.T) {
    app := testkit.New(t)
    admin := app.Create.User(factory.Admin())
    record := app.Create.Storage()

    app.As(admin).GET("/api/v1/storages/%d", record.ID).
        AssertOK().
        AssertJSONPath("data.name", record.Name)
    app.GET("/api/v1/storages/%d", record.ID).AssertUnauthorized()
}

func TestStorageService_GetStorage(t *testing.T) {
    app := testkit.New(t)
    record := app.Create.Storage()

    result, err := storages.NewStorageService(app.Queries, app.Secrets).GetStorage(app.Ctx, record.ID)

    require.NoError(t, err)
    assert.Equal(t, record.ID, result.ID)
}
```

- Assert responses with the `Assert*` methods and rows with `app.AssertDatabaseHas/Missing/Count`; decode with `.JSON(&v)` only when the typed struct is needed.
- No `// Setup`, `// Test`, `// Assert` comments, no hand-written `INSERT`, no per-test server or `defer`.

## What We DON'T Use
- NO Repository Pattern - Services use sqlc directly
- NO ORM - Raw SQL with sqlc for type safety
- NO complex abstractions - Keep it simple
- NO Test Mocking of DB - Real database with transaction rollback
