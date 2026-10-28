package integration

import (
	"os"
	"path/filepath"
	"testing"

	"app/internal/factory"
	"app/internal/plans"
	"app/internal/selfbackup"
	"app/internal/storages"
	"app/tests/testkit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func selfBackupOptions(t *testing.T) selfbackup.Options {
	return selfbackup.Options{
		Database:    sqliteFile(t),
		Dir:         filepath.Join(t.TempDir(), "backups"),
		Time:        "03:15",
		MaxAgeDays:  30,
		KeepWeekly:  8,
		KeepMonthly: 12,
	}
}

func ensureSelfBackup(t *testing.T, app *testkit.App, opts selfbackup.Options) *selfbackup.Result {
	t.Helper()
	result, err := selfbackup.Ensure(app.Ctx, app.Queries,
		storages.NewStorageService(app.Queries, app.Secrets),
		plans.NewPlanService(app.Queries, app.TxRunner, app.Secrets, app.Tools),
		opts)
	require.NoError(t, err)
	return result
}

func TestSelfBackup_SecondRunChangesNothing(t *testing.T) {
	app := testkit.New(t)
	admin := app.Create.User(factory.Admin())
	opts := selfBackupOptions(t)

	first := ensureSelfBackup(t, app, opts)
	assert.True(t, first.StorageCreated)
	assert.True(t, first.PlanCreated)
	info, err := os.Stat(opts.Dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	app.As(admin).GET("/api/v1/plans/%d", first.Plan.ID).
		AssertOK().
		AssertJSONPath("data.name", selfbackup.PlanName).
		AssertJSONPath("data.sqlite.path", opts.Database).
		AssertJSONPath("data.schedules.0.time", "03:15").
		AssertJSONCount("data.schedules.0.days", 0).
		AssertJSONPath("data.max_age_days", float64(30)).
		AssertJSONPath("data.keep_weekly", float64(8)).
		AssertJSONPath("data.keep_monthly", float64(12)).
		AssertJSONPath("data.enabled", true).
		AssertJSONPath("data.storages.0.name", selfbackup.StorageName).
		AssertJSONPath("data.storages.0.kind", "local")
	app.As(admin).GET("/api/v1/storages/%d", first.Storage.ID).
		AssertOK().
		AssertJSONPath("data.local.path", opts.Dir)

	second := ensureSelfBackup(t, app, opts)
	assert.False(t, second.StorageCreated)
	assert.False(t, second.PlanCreated)
	assert.Equal(t, first.Plan.ID, second.Plan.ID)
	app.AssertDatabaseCount("storages", 1)
	app.AssertDatabaseCount("plans", 1)
	app.AssertDatabaseCount("plan_storages", 1)
}

func TestSelfBackup_LeavesAPlanTheOwnerEditedAlone(t *testing.T) {
	app := testkit.New(t)
	admin := app.Create.User(factory.Admin())
	opts := selfBackupOptions(t)
	created := ensureSelfBackup(t, app, opts)
	offsite := app.Create.Storage(factory.WithStorageName("Offsite"))

	edit := sqlitePlan("Our own database", opts.Database, created.Storage.ID, offsite.ID)
	edit.Schedules = []plans.ScheduleInput{{Time: "05:45", Days: []string{"sun"}}}
	edit.MaxAgeDays, edit.KeepWeekly, edit.KeepMonthly = 7, 0, 0
	app.As(admin).PUT("/api/v1/plans/%d", edit, created.Plan.ID).AssertOK()

	again := ensureSelfBackup(t, app, opts)
	assert.False(t, again.PlanCreated)
	assert.Equal(t, created.Plan.ID, again.Plan.ID)
	app.AssertDatabaseCount("plans", 1)
	app.AssertDatabaseCount("storages", 2)
	app.As(admin).GET("/api/v1/plans/%d", created.Plan.ID).
		AssertOK().
		AssertJSONPath("data.name", "Our own database").
		AssertJSONPath("data.schedules.0.time", "05:45").
		AssertJSONPath("data.schedules.0.days.0", "sun").
		AssertJSONPath("data.max_age_days", float64(7)).
		AssertJSONPath("data.keep_weekly", float64(0)).
		AssertJSONCount("data.storages", 2)
}

func TestSelfBackup_ReusesTheStorageOfARunThatStoppedHalfway(t *testing.T) {
	app := testkit.New(t)
	opts := selfBackupOptions(t)
	storage := app.Create.Storage(factory.WithStorageName("Left behind"), factory.LocalStorage(opts.Dir))

	result := ensureSelfBackup(t, app, opts)
	assert.False(t, result.StorageCreated)
	assert.True(t, result.PlanCreated)
	assert.Equal(t, storage.ID, result.Storage.ID)
	app.AssertDatabaseCount("storages", 1)
	app.AssertDatabaseHas("plan_storages", testkit.Row{"plan_id": result.Plan.ID, "storage_id": storage.ID})
}

func TestSelfBackup_RejectsABadTimeBeforeCreatingAnything(t *testing.T) {
	app := testkit.New(t)
	opts := selfBackupOptions(t)
	opts.Time = "25:00"

	_, err := selfbackup.Ensure(app.Ctx, app.Queries,
		storages.NewStorageService(app.Queries, app.Secrets),
		plans.NewPlanService(app.Queries, app.TxRunner, app.Secrets, app.Tools),
		opts)
	require.Error(t, err)
	app.AssertDatabaseCount("storages", 0)
	app.AssertDatabaseCount("plans", 0)
}
