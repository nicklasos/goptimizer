package integration

import (
	"database/sql"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"app/internal/errs"
	"app/internal/factory"
	"app/internal/plans"
	"app/tests/testkit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sourcePassword = "pg-password-never-in-a-response"

// sqliteFile is a real SQLite database for a source to point at.
func sqliteFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "app.db")
	conn, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	_, err = conn.Exec("CREATE TABLE notes (id INTEGER PRIMARY KEY)")
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	return path
}

// closedPort is a local port nothing listens on, so a connection is refused at once.
func closedPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	return port
}

func sqlitePlan(name, path string, storageIDs ...int64) plans.PlanRequest {
	return plans.PlanRequest{
		Name:          name,
		SourceRequest: plans.SourceRequest{SourceKind: plans.SourceSQLite, SQLite: &plans.SQLiteInput{Path: path}},
		Schedules: []plans.ScheduleInput{
			{Time: "03:00", Days: []string{}},
			{Time: "18:00", Days: []string{"fri", "mon", "tue", "wed", "thu"}},
		},
		MaxAgeDays:  30,
		KeepWeekly:  8,
		KeepMonthly: 12,
		StorageIDs:  storageIDs,
		Enabled:     true,
	}
}

func postgresSource(port int, password string) plans.SourceRequest {
	return plans.SourceRequest{SourceKind: plans.SourcePostgres, Postgres: &plans.PostgresInput{
		Host: "127.0.0.1", Port: port, User: "backup", Database: "app", Password: password,
	}}
}

func postgresPlan(name string, port int, password string, storageIDs ...int64) plans.PlanRequest {
	req := sqlitePlan(name, "", storageIDs...)
	req.SourceRequest = postgresSource(port, password)
	return req
}

func storedSourceSecret(t *testing.T, app *testkit.App, id int64) []byte {
	t.Helper()
	plan, err := app.Queries.GetPlanByID(app.Ctx, id)
	require.NoError(t, err)
	return plan.SourceSecret
}

func TestPlansAPI_CRUD(t *testing.T) {
	app := testkit.New(t)
	admin := app.Create.User(factory.Admin())
	nightly := app.Create.Storage(factory.WithStorageName("Nightly"))
	offsite := app.Create.Storage(factory.WithStorageName("Offsite"))
	path := sqliteFile(t)

	resp := app.As(admin).POST("/api/v1/plans", sqlitePlan("App database", path, offsite.ID, nightly.ID, nightly.ID)).
		AssertOK().
		AssertJSONPath("data.name", "App database").
		AssertJSONPath("data.source_kind", "sqlite").
		AssertJSONPath("data.sqlite.path", path).
		AssertJSONMissing("data.postgres").
		AssertJSONPath("data.secret_set", false).
		AssertJSONPath("data.schedules.0.time", "03:00").
		AssertJSONCount("data.schedules.0.days", 0).
		AssertJSONPath("data.schedules.1.days.0", "mon").
		AssertJSONPath("data.schedules.1.days.4", "fri").
		AssertJSONPath("data.max_age_days", float64(30)).
		AssertJSONPath("data.keep_weekly", float64(8)).
		AssertJSONPath("data.keep_monthly", float64(12)).
		AssertJSONCount("data.storages", 2).
		AssertJSONPath("data.storages.0.name", "Nightly").
		AssertJSONPath("data.storages.0.kind", "local").
		AssertJSONPath("data.storages.1.name", "Offsite").
		AssertJSONPath("data.enabled", true)
	plan := testkit.Decode[plans.PlanDataResponse](resp).Data
	require.NotNil(t, plan.NextRunAt)
	nextRun, err := time.Parse(time.RFC3339, *plan.NextRunAt)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now(), nextRun, 24*time.Hour)
	assert.True(t, nextRun.After(time.Now()))
	app.AssertDatabaseCount("plan_storages", 2, testkit.Row{"plan_id": plan.ID})

	app.As(admin).GET("/api/v1/plans/%d", plan.ID).AssertOK().AssertJSONPath("data.name", "App database")
	app.As(admin).GET("/api/v1/plans").
		AssertOK().
		AssertJSONCount("data", 1).
		AssertJSONPath("data.0.storages.1.name", "Offsite").
		AssertJSONPath("pagination.total", float64(1))

	update := sqlitePlan("App database renamed", path, offsite.ID)
	update.Schedules = update.Schedules[1:]
	update.Enabled = false
	app.As(admin).PUT("/api/v1/plans/%d", update, plan.ID).
		AssertOK().
		AssertJSONPath("data.name", "App database renamed").
		AssertJSONCount("data.schedules", 1).
		AssertJSONCount("data.storages", 1).
		AssertJSONPath("data.storages.0.name", "Offsite").
		AssertJSONPath("data.enabled", false).
		AssertJSONPath("data.next_run_at", nil)
	app.AssertDatabaseMissing("plan_storages", testkit.Row{"plan_id": plan.ID, "storage_id": nightly.ID})

	app.As(admin).DELETE("/api/v1/plans/%d", plan.ID).AssertOK()
	app.As(admin).GET("/api/v1/plans/%d", plan.ID).AssertError(http.StatusNotFound, errs.ErrKeyPlansNotFound)
	app.As(admin).DELETE("/api/v1/plans/%d", plan.ID).AssertNotFound()
	app.As(admin).PUT("/api/v1/plans/%d", update, plan.ID).AssertNotFound()
	app.AssertDatabaseCount("plan_storages", 0, testkit.Row{"plan_id": plan.ID})
	app.AssertDatabaseHas("storages", testkit.Row{"id": nightly.ID})
}

func TestPlansAPI_ManualOnlyPlansHaveNoNextRun(t *testing.T) {
	app := testkit.New(t)
	admin := app.Create.User(factory.Admin())
	storage := app.Create.Storage()

	req := sqlitePlan("Manual", sqliteFile(t), storage.ID)
	req.Schedules = []plans.ScheduleInput{}
	app.As(admin).POST("/api/v1/plans", req).
		AssertOK().
		AssertJSONCount("data.schedules", 0).
		AssertJSONPath("data.next_run_at", nil)

	req.Name, req.Schedules = "Omitted", nil
	app.As(admin).POST("/api/v1/plans", req).AssertOK().AssertJSONCount("data.schedules", 0)
}

func TestPlansAPI_SourcePasswordNeverLeavesTheAPI(t *testing.T) {
	app := testkit.New(t)
	admin := app.Create.User(factory.SuperAdmin())
	storage := app.Create.Storage()
	port := closedPort(t)
	assertNoPassword := func(t *testing.T, resp *testkit.Response) {
		t.Helper()
		assert.NotContains(t, resp.String(), sourcePassword)
		assert.NotContains(t, resp.String(), "password")
	}

	resp := app.As(admin).POST("/api/v1/plans", postgresPlan("Shop", port, sourcePassword, storage.ID)).
		AssertOK().
		AssertJSONPath("data.secret_set", true).
		AssertJSONPath("data.postgres.host", "127.0.0.1").
		AssertJSONPath("data.postgres.port", float64(port)).
		AssertJSONPath("data.postgres.ssl_mode", "disable")
	assertNoPassword(t, resp)
	id := testkit.Decode[plans.PlanDataResponse](resp).Data.ID

	sealed := storedSourceSecret(t, app, id)
	assert.NotContains(t, string(sealed), sourcePassword, "the column holds ciphertext")
	plain, err := app.Secrets.Open(sealed)
	require.NoError(t, err)
	assert.JSONEq(t, `{"password":"`+sourcePassword+`"}`, string(plain))
	app.AssertDatabaseMissing("plans", testkit.Row{"id": id, "source_config": string(plain)})

	assertNoPassword(t, app.As(admin).GET("/api/v1/plans/%d", id).AssertOK())
	assertNoPassword(t, app.As(admin).GET("/api/v1/plans").AssertOK())
	assertNoPassword(t, app.As(admin).POST("/api/v1/plans/%d/test", nil, id).AssertOK())

	t.Run("a blank password on an update keeps the stored one", func(t *testing.T) {
		resp := app.As(admin).PUT("/api/v1/plans/%d", postgresPlan("Shop renamed", port, "", storage.ID), id).
			AssertOK().
			AssertJSONPath("data.secret_set", true)
		assertNoPassword(t, resp)
		assert.Equal(t, sealed, storedSourceSecret(t, app, id))
	})

	t.Run("a new password replaces it", func(t *testing.T) {
		assertNoPassword(t, app.As(admin).PUT("/api/v1/plans/%d", postgresPlan("Shop", port, "replaced", storage.ID), id).AssertOK())
		plain, err := app.Secrets.Open(storedSourceSecret(t, app, id))
		require.NoError(t, err)
		assert.JSONEq(t, `{"password":"replaced"}`, string(plain))
	})

	t.Run("a new plan needs a password", func(t *testing.T) {
		app.As(admin).POST("/api/v1/plans", postgresPlan("No password", port, "", storage.ID)).
			AssertFieldError("postgres.password", "validation.postgres.password.required")
	})

	t.Run("changing the kind drops the password, and changing back needs one", func(t *testing.T) {
		app.As(admin).PUT("/api/v1/plans/%d", sqlitePlan("Shop", sqliteFile(t), storage.ID), id).
			AssertOK().
			AssertJSONPath("data.secret_set", false)
		app.AssertDatabaseHas("plans", testkit.Row{"id": id, "source_secret": nil})

		app.As(admin).PUT("/api/v1/plans/%d", postgresPlan("Shop", port, "", storage.ID), id).
			AssertFieldError("postgres.password", "validation.postgres.password.required")
	})
}

func TestPlansAPI_Validation(t *testing.T) {
	app := testkit.New(t)
	admin := app.Create.User(factory.Admin())
	storage := app.Create.Storage()
	path := sqliteFile(t)
	valid := func() plans.PlanRequest { return sqlitePlan("Valid", path, storage.ID) }
	post := func(req any) *testkit.Response { return app.As(admin).POST("/api/v1/plans", req) }

	post(`{}`).AssertInvalid("name", "source_kind", "storage_ids")

	t.Run("the source block must match the kind", func(t *testing.T) {
		req := valid()
		req.SQLite = nil
		post(req).AssertFieldError("sqlite", "validation.sqlite.required")

		req = valid()
		req.Files = &plans.FilesInput{Paths: []string{"/srv"}}
		post(req).AssertFieldError("files", "validation.files.invalid")

		req = valid()
		req.SourceKind = "oracle"
		post(req).AssertInvalid("source_kind")
	})

	t.Run("paths must be absolute and patterns valid", func(t *testing.T) {
		req := valid()
		req.SQLite.Path = "data/app.db"
		post(req).AssertFieldError("sqlite.path", "validation.sqlite.path.invalid")

		req = valid()
		req.SourceRequest = plans.SourceRequest{SourceKind: plans.SourceFiles, Files: &plans.FilesInput{
			Paths:    []string{"/srv/uploads", "relative"},
			Excludes: []string{"*.tmp", "[unclosed"},
		}}
		post(req).
			AssertFieldError("files.paths.1", "validation.files.paths.1.invalid").
			AssertFieldError("files.excludes.1", "validation.files.excludes.1.invalid")

		req.Files = &plans.FilesInput{Paths: []string{}}
		post(req).AssertInvalid("files.paths")
	})

	t.Run("database fields", func(t *testing.T) {
		req := valid()
		req.SourceRequest = postgresSource(70000, sourcePassword)
		req.Postgres.SSLMode = "sometimes"
		req.Postgres.Host = ""
		post(req).AssertInvalid("postgres.port", "postgres.ssl_mode", "postgres.host")

		req.SourceRequest = plans.SourceRequest{SourceKind: plans.SourceMySQL, MySQL: &plans.MySQLInput{Host: "db", User: "root"}}
		post(req).AssertInvalid("mysql.database")
	})

	t.Run("schedules", func(t *testing.T) {
		req := valid()
		req.Schedules = []plans.ScheduleInput{
			{Time: "03:00", Days: []string{}},
			{Time: "25:00", Days: []string{}},
			{Time: "04:00", Days: []string{"someday"}},
		}
		post(req).
			AssertFieldError("schedules.1.time", "validation.schedules.1.time.invalid").
			AssertFieldError("schedules.2.days", "validation.schedules.2.days.invalid")

		req.Schedules = []plans.ScheduleInput{
			{Time: "03:00", Days: []string{"mon", "tue"}},
			{Time: "03:00", Days: []string{"tue", "mon", "mon"}},
		}
		post(req).AssertFieldError("schedules.1", errs.ErrKeyPlansDuplicateSchedule)

		req.Schedules = []plans.ScheduleInput{
			{Time: "03:00", Days: []string{}},
			{Time: "03:00", Days: []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}},
		}
		post(req).AssertFieldError("schedules.1", errs.ErrKeyPlansDuplicateSchedule)
	})

	t.Run("retention", func(t *testing.T) {
		req := valid()
		req.MaxAgeDays, req.KeepWeekly, req.KeepMonthly = 3651, 521, 241
		post(req).AssertInvalid("max_age_days", "keep_weekly", "keep_monthly")

		req.MaxAgeDays, req.KeepWeekly, req.KeepMonthly = -1, 0, 0
		post(req).AssertInvalid("max_age_days")

		req.MaxAgeDays, req.KeepWeekly, req.KeepMonthly = 0, 4, 6
		post(req).
			AssertFieldError("keep_weekly", errs.ErrKeyPlansKeepNeedsMaxAge).
			AssertFieldError("keep_monthly", errs.ErrKeyPlansKeepNeedsMaxAge)

		req.MaxAgeDays, req.KeepWeekly, req.KeepMonthly = 0, 0, 0
		post(req).AssertOK().AssertJSONPath("data.max_age_days", float64(0))
	})

	t.Run("storages", func(t *testing.T) {
		req := valid()
		req.Name = "No storages"
		req.StorageIDs = []int64{}
		post(req).AssertInvalid("storage_ids")

		req.StorageIDs = []int64{storage.ID, 999999}
		post(req).AssertFieldError("storage_ids", errs.ErrKeyPlansStorageNotFound)
		app.AssertDatabaseMissing("plans", testkit.Row{"name": "No storages"})
	})

	t.Run("names are unique, ignoring case", func(t *testing.T) {
		req := valid()
		req.Name = "Shop"
		post(req).AssertOK()
		req.Name = "SHOP"
		post(req).AssertFieldError("name", errs.ErrKeyPlansNameTaken)

		other := app.Create.Plan(factory.WithPlanName("Other"), factory.WithStorages(storage))
		app.As(admin).PUT("/api/v1/plans/%d", req, other.ID).AssertFieldError("name", errs.ErrKeyPlansNameTaken)
	})
}

func TestPlansAPI_TestSource(t *testing.T) {
	app := testkit.New(t)
	admin := app.Create.User(factory.Admin())
	test := func(req plans.SourceRequest) *testkit.Response {
		return app.As(admin).POST("/api/v1/plans/test", req).AssertOK()
	}
	sqlite := func(path string) plans.SourceRequest {
		return plans.SourceRequest{SourceKind: plans.SourceSQLite, SQLite: &plans.SQLiteInput{Path: path}}
	}
	files := func(paths ...string) plans.SourceRequest {
		return plans.SourceRequest{SourceKind: plans.SourceFiles, Files: &plans.FilesInput{Paths: paths}}
	}
	path := sqliteFile(t)

	t.Run("a SQLite file with sqlite3 installed passes", func(t *testing.T) {
		test(sqlite(path)).
			AssertJSONPath("data.ok", true).
			AssertJSONMissing("data.step").
			AssertJSONPath("data.tool.name", "sqlite3").
			AssertJSONPath("data.tool.found", true).
			AssertJSONPath("data.tool.version", testkit.FakeToolVersions["sqlite3"]).
			AssertJSONPath("data.server_version", "SQLite "+sqliteLibraryVersion(t))
	})

	t.Run("without sqlite3 it fails at the tool step", func(t *testing.T) {
		app.Tools.Remove("sqlite3")
		t.Cleanup(func() { app.Tools.Set("sqlite3", testkit.FakeToolVersions["sqlite3"]) })

		resp := test(sqlite(path)).
			AssertJSONPath("data.ok", false).
			AssertJSONPath("data.step", "tool").
			AssertJSONPath("data.tool.found", false)
		assert.Contains(t, resp.String(), "sqlite3 was not found")
	})

	t.Run("a file that is not a database fails at the path step", func(t *testing.T) {
		text := filepath.Join(t.TempDir(), "notes.txt")
		require.NoError(t, os.WriteFile(text, []byte("hello"), 0o600))

		assert.Contains(t, test(sqlite(text)).AssertJSONPath("data.step", "path").String(), "is not a SQLite database")
		assert.Contains(t, test(sqlite(filepath.Join(t.TempDir(), "missing.db"))).AssertJSONPath("data.step", "path").String(), "does not exist")
		assert.Contains(t, test(sqlite(t.TempDir())).AssertJSONPath("data.step", "path").String(), "is a directory")
	})

	t.Run("files pass when every path can be read", func(t *testing.T) {
		dir := t.TempDir()
		test(files(dir, path)).AssertJSONPath("data.ok", true).AssertJSONMissing("data.tool")

		missing := filepath.Join(dir, "gone")
		resp := test(files(dir, missing, filepath.Join(dir, "also-gone"))).
			AssertJSONPath("data.ok", false).
			AssertJSONPath("data.step", "path").
			AssertJSONPath("data.error", missing+" does not exist")
		assert.NotContains(t, resp.String(), "also-gone", "the first failure is reported")
	})

	t.Run("an unreadable directory fails", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root reads everything")
		}
		locked := t.TempDir()
		require.NoError(t, os.Chmod(locked, 0o000))
		t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })

		test(files(locked)).AssertJSONPath("data.step", "path").AssertJSONPath("data.error", locked+" cannot be read: permission denied")
	})

	t.Run("a database that refuses the connection fails at the connect step, with the tool reported", func(t *testing.T) {
		port := closedPort(t)
		resp := test(postgresSource(port, sourcePassword)).
			AssertJSONPath("data.ok", false).
			AssertJSONPath("data.step", "connect").
			AssertJSONPath("data.tool.name", "pg_dump").
			AssertJSONPath("data.tool.version", testkit.FakeToolVersions["pg_dump"]).
			AssertJSONMissing("data.server_version")
		assert.Contains(t, resp.String(), strconv.Itoa(port))
		assert.NotContains(t, resp.String(), sourcePassword)

		test(plans.SourceRequest{SourceKind: plans.SourceMySQL, MySQL: &plans.MySQLInput{
			Host: "127.0.0.1", Port: port, User: "root", Database: "app", Password: sourcePassword,
		}}).
			AssertJSONPath("data.ok", false).
			AssertJSONPath("data.step", "connect").
			AssertJSONPath("data.tool.name", "mysqldump")
	})

	t.Run("an edit form with a blank password uses the stored one", func(t *testing.T) {
		port := closedPort(t)
		saved := app.Create.Plan(factory.PostgresSource(plans.PostgresConfig{Host: "127.0.0.1", Port: port, User: "u", Database: "d", SSLMode: "disable"}, sourcePassword))

		app.As(admin).POST("/api/v1/plans/test?plan_id=%d", postgresSource(port, ""), saved.ID).
			AssertOK().
			AssertJSONPath("data.step", "connect")
		app.As(admin).POST("/api/v1/plans/test", postgresSource(port, "")).
			AssertFieldError("postgres.password", "validation.postgres.password.required")
		app.As(admin).POST("/api/v1/plans/test?plan_id=%d", postgresSource(port, ""), 999999).AssertNotFound()
		app.As(admin).POST("/api/v1/plans/test?plan_id=abc", postgresSource(port, "")).AssertInvalid("plan_id")
	})

	t.Run("a saved plan", func(t *testing.T) {
		plan := app.Create.Plan(factory.SQLiteSource(path))
		app.As(admin).POST("/api/v1/plans/%d/test", nil, plan.ID).AssertOK().AssertJSONPath("data.ok", true)
		app.As(admin).POST("/api/v1/plans/%d/test", nil, 999999).AssertNotFound()
	})

	t.Run("a password sealed with another key fails at the connect step", func(t *testing.T) {
		plan := app.Create.Plan(factory.PostgresSource(plans.PostgresConfig{Host: "127.0.0.1", Port: 5432}, "x"))
		_, err := app.DB.Exec(`UPDATE plans SET source_secret = ? WHERE id = ?`, []byte("not sealed with this key"), plan.ID)
		require.NoError(t, err)

		app.As(admin).POST("/api/v1/plans/%d/test", nil, plan.ID).AssertOK().AssertJSONPath("data.step", "connect")
	})
}

// sqliteLibraryVersion is the version of the driver that wrote the test's files.
func sqliteLibraryVersion(t *testing.T) string {
	t.Helper()
	conn, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer conn.Close()
	var version string
	require.NoError(t, conn.QueryRow("SELECT sqlite_version()").Scan(&version))
	return version
}

func TestPlansAPI_AdminsOnly(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User()
	plan := app.Create.Plan()

	app.GET("/api/v1/plans").AssertUnauthorized()
	app.As(user).GET("/api/v1/plans").AssertForbidden()
	app.As(user).GET("/api/v1/plans/%d", plan.ID).AssertForbidden()
	app.As(user).POST("/api/v1/plans", `{}`).AssertForbidden()
	app.As(user).POST("/api/v1/plans/test", `{}`).AssertForbidden()
	app.As(user).DELETE("/api/v1/plans/%d", plan.ID).AssertForbidden()
	app.AssertDatabaseHas("plans", testkit.Row{"id": plan.ID})
}

func TestStoragesAPI_AStorageInUseCannotBeDeleted(t *testing.T) {
	app := testkit.New(t)
	admin := app.Create.User(factory.Admin())
	storage := app.Create.Storage()
	first := app.Create.Plan(factory.WithPlanName("Shop"), factory.WithStorages(storage))
	app.Create.Plan(factory.WithPlanName("Blog"), factory.WithStorages(storage, app.Create.Storage()))

	resp := app.As(admin).DELETE("/api/v1/storages/%d", storage.ID).AssertError(http.StatusConflict, errs.ErrKeyStoragesInUse)
	var body struct {
		Details struct {
			Plans []struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
			} `json:"plans"`
		} `json:"details"`
	}
	require.NoError(t, json.Unmarshal(resp.Body, &body))
	require.Len(t, body.Details.Plans, 2)
	assert.Equal(t, "Blog", body.Details.Plans[0].Name)
	assert.Equal(t, "Shop", body.Details.Plans[1].Name)
	assert.Equal(t, first.ID, body.Details.Plans[1].ID)
	app.AssertDatabaseHas("storages", testkit.Row{"id": storage.ID})

	for _, plan := range body.Details.Plans {
		app.As(admin).DELETE("/api/v1/plans/%d", plan.ID).AssertOK()
	}
	app.As(admin).DELETE("/api/v1/storages/%d", storage.ID).AssertOK()
}
