# Gogo API Tests

Tests run against a real SQLite database. Each test gets its own freshly migrated database file in
`t.TempDir()`, removed when the test ends: the same idea as Laravel's `RefreshDatabase`. The schema is
migrated once per test binary and copied for each test, so this costs a file copy.

## Writing a test

Everything starts from `testkit.New(t)`. There is nothing to close and no context to pass around.

An API test (`tests/integration`):

```go
func TestUsersAPI_CreateUser(t *testing.T) {
    app := testkit.New(t)
    admin := app.Create.User(factory.Admin())
    body := users.CreateUserRequest{Name: "Ada", Email: "ada@example.com", Password: "password123", Role: middleware.RoleUser}

    app.As(admin).POST("/api/v1/users", body).
        AssertOK().
        AssertJSONPath("data.name", "Ada")

    app.POST("/api/v1/users", body).AssertUnauthorized()
    app.As(admin).POST("/api/v1/users", `{}`).AssertInvalid("name")

    app.AssertDatabaseHas("users", testkit.Row{"name": "Ada", "email": "ada@example.com"})
}
```

A service test (`tests/unit`):

```go
func TestUserService_Update(t *testing.T) {
    app := testkit.New(t)
    admin := app.Create.User(factory.Admin())
    record := app.Create.User()
    service := users.NewUserService(app.Queries, app.TxRunner)

    result, err := service.Update(app.Ctx, testkit.Actor(admin), record.ID, users.UpdateUserRequest{Name: "Ada", Email: record.Email})

    require.NoError(t, err)
    assert.Equal(t, "Ada", result.Name)
}
```

## Cheat sheet

| Laravel | Here |
|---|---|
| `use RefreshDatabase` | `app := testkit.New(t)` |
| `User::factory()->create()` | `app.Create.User()` |
| `User::factory()->admin()->create(['email' => $e])` | `app.Create.User(factory.Admin(), factory.WithEmail(e))` |
| `$this->getJson($url)` | `app.GET("/api/v1/users?page=%d", page)` |
| `$this->actingAs($user)->postJson($url, $data)` | `app.As(user).POST("/api/v1/users", data)` |
| `->assertOk()`, `->assertForbidden()` | `.AssertOK()`, `.AssertForbidden()` |
| `->assertJsonPath('data.0.name', 'Hi')` | `.AssertJSONPath("data.0.name", "Hi")` |
| `->assertJsonCount(3, 'data')` | `.AssertJSONCount("data", 3)` |
| `->assertJsonValidationErrors(['name'])` | `.AssertInvalid("name")` |
| `$this->assertDatabaseHas('users', [...])` | `app.AssertDatabaseHas("users", testkit.Row{...})` |
| `Mail::fake()` / `Mail::assertSent` | `app.Mail.Sent()` |
| `config(['app.x' => false])` | `testkit.New(t, func(cfg *config.Config) { cfg.X = false })` |

### `app` (`tests/testkit/app.go`)

- `app.Create` — the factories, below
- `app.Ctx`, `app.Queries`, `app.DB` (`*sql.DB`, for raw SQL with `?` placeholders), `app.TxRunner`, `app.Config`, `app.Logger`, `app.Cache` — what a service constructor asks for
- `app.Mail.Sent()` — every email the application sent during the test
- `app.Token(user)`, `testkit.Actor(user)`

### Requests (`tests/testkit/request.go`)

- `app.GET(path, args...)`, `app.DELETE(path, args...)`
- `app.POST(path, body, args...)`, `PUT`, `PATCH` — the body is a struct, a map or a string of raw JSON; `nil` sends none
- `app.As(user)` — the same verbs, logged in as the user
- `app.WithToken(token)`, `app.WithHeader(key, value)`

The path takes `fmt.Sprintf` arguments: `app.PUT("/api/v1/users/%d", body, id)`.

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

One file in `internal/factory`, shaped like `user.go`:

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
