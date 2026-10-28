# Gogo API Tests

Tests run against a real SQLite database. Each test gets its own freshly migrated database file in
`t.TempDir()`, removed when the test ends: the same idea as Laravel's `RefreshDatabase`. The schema is
migrated once per test binary and copied for each test, so this costs a file copy.

## Writing a test

Everything starts from `testkit.New(t)`. There is nothing to close and no context to pass around.

An API test (`tests/integration`):

```go
func TestStoragesAPI_CreateStorage(t *testing.T) {
    app := testkit.New(t)
    admin := app.Create.User(factory.Admin())
    body := storages.StorageRequest{Name: "Nightly", Kind: storages.KindLocal, Local: &storages.LocalInput{Path: t.TempDir()}}

    app.As(admin).POST("/api/v1/storages", body).
        AssertOK().
        AssertJSONPath("data.name", "Nightly")

    app.POST("/api/v1/storages", body).AssertUnauthorized()
    app.As(admin).POST("/api/v1/storages", `{}`).AssertInvalid("name")

    app.AssertDatabaseHas("storages", testkit.Row{"name": "Nightly", "kind": "local"})
}
```

A service test (`tests/unit`):

```go
func TestStorageService_GetStorage(t *testing.T) {
    app := testkit.New(t)
    record := app.Create.Storage()
    service := storages.NewStorageService(app.Queries, app.Secrets)

    result, err := service.GetStorage(app.Ctx, record.ID)

    require.NoError(t, err)
    assert.Equal(t, record.Name, result.Name)
}
```

## Cheat sheet

| Laravel | Here |
|---|---|
| `use RefreshDatabase` | `app := testkit.New(t)` |
| `User::factory()->create()` | `app.Create.User()` |
| `User::factory()->admin()->create(['email' => $e])` | `app.Create.User(factory.Admin(), factory.WithEmail(e))` |
| `$this->getJson($url)` | `app.GET("/api/v1/storages/%d", id)` |
| `$this->actingAs($user)->postJson($url, $data)` | `app.As(user).POST("/api/v1/storages", data)` |
| `->assertOk()`, `->assertForbidden()` | `.AssertOK()`, `.AssertForbidden()` |
| `->assertJsonPath('data.0.name', 'Hi')` | `.AssertJSONPath("data.0.name", "Hi")` |
| `->assertJsonCount(3, 'data')` | `.AssertJSONCount("data", 3)` |
| `->assertJsonValidationErrors(['name'])` | `.AssertInvalid("name")` |
| `$this->assertDatabaseHas('storages', [...])` | `app.AssertDatabaseHas("storages", testkit.Row{...})` |
| `Mail::fake()` / `Mail::assertSent` | `app.Mail.Sent()` |
| `config(['app.x' => false])` | `testkit.New(t, func(cfg *config.Config) { cfg.X = false })` |

### `app` (`tests/testkit/app.go`)

- `app.Create` — the factories, below
- `app.Ctx`, `app.Queries`, `app.DB` (`*sql.DB`, for raw SQL with `?` placeholders), `app.TxRunner`, `app.Config`, `app.Logger`, `app.Cache` — what a service constructor asks for
- `app.Mail.Sent()` — every email the application sent during the test
- `app.Secrets` — seals and opens stored credentials with the test key; `testkit.NewFakeS3(t)` and `testkit.NewFakeGCS(t)` are offline storage servers
- `app.Tools` — the dump programs the API finds on PATH: all of `testkit.FakeToolVersions` until `app.Tools.Remove("sqlite3")`. They do not exist; for a run that dumps, `app.UseRealTool("sqlite3")` puts in the real one and `app.Tools.Install(name, scriptPath, version)` a script
- `app.Worker()` — the backup runner on the test's database, not started: `app.Worker().Drain(app.Ctx)` runs everything queued, synchronously
- `app.Token(user)`, `testkit.Actor(user)`

### Requests (`tests/testkit/request.go`)

- `app.GET(path, args...)`, `app.DELETE(path, args...)`
- `app.POST(path, body, args...)`, `PUT`, `PATCH` — the body is a struct, a map or a string of raw JSON; `nil` sends none
- `app.As(user)` — the same verbs, logged in as the user
- `app.WithToken(token)`, `app.WithHeader(key, value)`

The path takes `fmt.Sprintf` arguments: `app.PUT("/api/v1/storages/%d", body, id)`.

### Response (`tests/testkit/response.go`)

Every `Assert` stops the test on failure, prints the response body, and returns the response so calls chain.

- `AssertOK()`, `AssertBadRequest()`, `AssertUnauthorized()`, `AssertForbidden()`, `AssertNotFound()`, `AssertTooManyRequests()`, `AssertStatus(code)`
- `AssertError(status, errs.ErrKeyX)` — status and `error_key`
- `AssertInvalid(fields...)` — validation failed, and each field has an error
- `AssertJSONPath(path, want)`, `AssertJSONCount(path, n)`, `AssertJSONMissing(path)`
- `JSON(&v)` decodes the body when you want the typed struct; `resp.StatusCode`, `resp.Body`, `resp.Header` are there too

### Database (`tests/testkit/database.go`)

- `app.AssertDatabaseHas(table, testkit.Row{...})`
- `app.AssertDatabaseMissing(table, testkit.Row{...})`
- `app.AssertDatabaseCount(table, n)` or `app.AssertDatabaseCount(table, n, testkit.Row{...})`

### Factories (`internal/factory`)

| Factory | Options |
|---|---|
| `app.Create.User()` | `WithEmail`, `WithName`, `WithPassword`, `WithRoles`, `Admin()`, `SuperAdmin()`, `Unverified()` |
| `app.Create.Storage()` | `WithStorageName`, `LocalStorage(path)`, `S3Storage(config, keyID, secret)`, `GCSStorage(config, credentialsJSON)` |
| `app.Create.Plan()` | `WithPlanName`, `FilesSource(paths...)`, `FilesSourceExcluding(paths, excludes)`, `SQLiteSource(path)`, `PostgresSource(config, password)`, `MySQLSource(config, password)`, `WithSchedules(...)`, `WithRetention(days, weeks, months)`, `WithStorages(...)`, `CreatedAt(t)`, `Disabled()` |
| `app.Create.Run(plan)` | `WithRunStatus(runs.StatusRunning)`, `ScheduledFor(slot)` |
| `app.Create.Archive(run, storage)` | `ArchiveKey(key)`, `ArchiveContent(bytes)`, `ArchiveCreatedAt(t)` |

A user has the password `factory.DefaultPassword`. Tests never write `INSERT` by hand, and neither
does a factory: every factory writes through a `Factory*` query in `internal/db/queries/factory.sql`,
the same as application code goes through its own module's queries.

## Adding a factory

A query in `internal/db/queries/factory.sql`, named `Factory<Thing>`, `RETURNING *` so it returns
the generated `db.X` struct directly, then `make sqlc`:

```sql
-- name: FactoryCreateOrder :one
INSERT INTO orders (user_id, status) VALUES (sqlc.arg(user_id), sqlc.arg(status))
RETURNING *;
```

One file in `internal/factory`, shaped like `storage.go`:

```go
type orderAttrs struct {
    status string
}

type OrderOption func(*orderAttrs)

func WithStatus(status string) OrderOption { return func(a *orderAttrs) { a.status = status } }

func (f *Factory) Order(userID int64, opts ...OrderOption) *db.Order {
    f.t.Helper()

    attrs := orderAttrs{status: "new"}
    for _, opt := range opts {
        opt(&attrs)
    }

    order, err := f.queries.FactoryCreateOrder(ctx, db.FactoryCreateOrderParams{
        UserID: userID,
        Status: attrs.status,
    })
    if err != nil {
        f.t.Fatalf("factory.Order: %v", err)
    }
    return &order
}
```

## Things to know

- A failed statement (a unique violation, say) does not spoil the rest of the test: nothing is wrapped in a shared transaction.
- Raw SQL in a test writes timestamps with `CURRENT_TIMESTAMP` or `datetime('now', '-1 minute')`.
- The test server is the production engine and routes (`server.NewEngine`, `server.RegisterRoutes`) and starts on the first request.
- `tests/unit` holds service, policy and pure-logic tests; `tests/integration` holds HTTP tests. Both use the database.
- A helper used by one test file lives at the top of that file (see `newAuthService` in `tests/unit/auth_service_test.go`).

## Running Tests

Nothing to set up: no database server, no environment variables.

```bash
# Run all tests
make test

# Run only unit tests
make test-unit

# Run only integration tests
make test-integration

# Run with verbose output
make test-verbose

# Run specific test
go test -v ./tests/unit -run TestAuthService_Register
```
