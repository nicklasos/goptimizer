package integration

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"app/internal/factory"
	"app/internal/plans"
	"app/internal/runs"
	"app/internal/secretbox"
	"app/internal/storages"
	"app/tests/testkit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDashboardAPI(t *testing.T) {
	app := testkit.New(t)
	admin := app.Create.User(factory.Admin())
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	longAgo := today.AddDate(0, 0, -60)
	// A slot two hours ago every day: one that ran is ok, one that did not is overdue.
	slot := plans.Schedule{Time: plans.TimeOfDay(now.Add(-2*time.Hour).Hour() * 60), Days: []plans.Weekday{}}
	storage := app.Create.Storage(factory.WithStorageName("Disk"))

	healthy := app.Create.Plan(factory.WithPlanName("B healthy"), factory.WithStorages(storage), factory.WithSchedules(slot), factory.CreatedAt(longAgo))
	app.Create.Run(healthy, factory.WithRunStatus(runs.StatusSucceeded), factory.StartedAt(today.AddDate(0, 0, -29).Add(time.Hour)))
	app.Create.Run(healthy, factory.WithRunStatus(runs.StatusSucceeded), factory.StartedAt(today.AddDate(0, 0, -30).Add(time.Hour)))
	app.Create.Run(healthy, factory.WithRunStatus(runs.StatusFailed), factory.StartedAt(today.AddDate(0, 0, -2).Add(time.Hour)))
	app.Create.Run(healthy, factory.WithRunStatus(runs.StatusSucceeded), factory.StartedAt(today.AddDate(0, 0, -2).Add(2*time.Hour)))
	app.Create.Run(healthy, factory.WithRunStatus(runs.StatusSucceeded), factory.StartedAt(now.Add(-90*time.Minute)), factory.RunSize(4096))

	failed := app.Create.Plan(factory.WithPlanName("A failed"), factory.WithSchedules(), factory.CreatedAt(longAgo))
	app.Create.Run(failed, factory.WithRunStatus(runs.StatusFailed), factory.StartedAt(now.Add(-time.Hour)))

	overdue := app.Create.Plan(factory.WithPlanName("C overdue"), factory.WithSchedules(slot), factory.CreatedAt(longAgo))
	app.Create.Run(overdue, factory.WithRunStatus(runs.StatusSucceeded), factory.StartedAt(now.Add(-50*time.Hour)))

	running := app.Create.Plan(factory.WithPlanName("D running"), factory.CreatedAt(longAgo))
	app.Create.Run(running, factory.WithRunStatus(runs.StatusRunning))

	app.Create.Plan(factory.WithPlanName("E paused"), factory.Disabled(), factory.CreatedAt(longAgo))
	app.Create.Plan(factory.WithPlanName("F never"), factory.WithSchedules())

	restoring := app.Create.Plan(factory.WithPlanName("G restoring"), factory.WithStorages(storage), factory.CreatedAt(longAgo))
	backup := app.Create.Run(restoring, factory.WithRunStatus(runs.StatusSucceeded), factory.StartedAt(now.Add(-time.Hour)))
	app.Create.Restore(app.Create.Archive(backup, storage), factory.WithRestoreStatus(runs.StatusRunning))

	resp := app.As(admin).GET("/api/v1/dashboard").
		AssertOK().
		AssertJSONCount("data.plans", 7).
		AssertJSONPath("data.summary", map[string]any{"ok": 1, "failed": 1, "overdue": 1, "restoring": 1, "running": 1, "paused": 1, "never": 1})
	dashboard := testkit.Decode[plans.DashboardDataResponse](resp).Data

	health := map[string]plans.Health{}
	for _, plan := range dashboard.Plans {
		health[plan.Name] = plan.Health
		require.Len(t, plan.Days, plans.DashboardDays, plan.Name)
		assert.Equal(t, today.AddDate(0, 0, -29).Format(time.DateOnly), plan.Days[0].Date, "oldest first")
		assert.Equal(t, today.Format(time.DateOnly), plan.Days[29].Date, "today last")
	}
	assert.Equal(t, map[string]plans.Health{
		"A failed": plans.HealthFailed, "B healthy": plans.HealthOK, "C overdue": plans.HealthOverdue,
		"D running": plans.HealthRunning, "E paused": plans.HealthPaused, "F never": plans.HealthNever,
		"G restoring": plans.HealthRestoring,
	}, health)
	assert.Equal(t, "A failed", dashboard.Plans[0].Name, "by name")

	b := dashboard.Plans[1]
	assert.Equal(t, plans.DayResponse{Date: today.AddDate(0, 0, -29).Format(time.DateOnly), Succeeded: 1}, b.Days[0], "the run 30 days ago is outside the window")
	assert.Equal(t, plans.DayResponse{Date: today.AddDate(0, 0, -2).Format(time.DateOnly), Succeeded: 1, Failed: 1}, b.Days[27])
	var counted int64
	for _, day := range b.Days {
		counted += day.Succeeded + day.Failed
	}
	assert.Equal(t, int64(4), counted)
	require.NotNil(t, b.LastSize)
	assert.Equal(t, int64(4096), *b.LastSize)
	assert.Equal(t, "b-healthy/", b.Folder)
	require.Len(t, b.Storages, 1)
	assert.Equal(t, "Disk", b.Storages[0].Name)

	app.As(admin).GET("/api/v1/plans/%d", overdue.ID).AssertOK().AssertJSONPath("data.health", "overdue")
	app.As(admin).GET("/api/v1/plans").AssertOK().AssertJSONPath("data.0.health", "failed")

	app.GET("/api/v1/dashboard").AssertUnauthorized()
	app.As(app.Create.User()).GET("/api/v1/dashboard").AssertForbidden()
}

func TestDashboardAPI_NoPlans(t *testing.T) {
	app := testkit.New(t)
	app.As(app.Create.User(factory.Admin())).GET("/api/v1/dashboard").
		AssertOK().
		AssertJSONCount("data.plans", 0).
		AssertJSONPath("data.summary.ok", 0)
}

func TestPlansAPI_PauseAndResume(t *testing.T) {
	app := testkit.New(t)
	admin := app.Create.User(factory.Admin())
	savedAt := time.Now().UTC().Add(-48 * time.Hour).Truncate(time.Second)
	plan := app.Create.Plan(factory.CreatedAt(savedAt))

	app.As(admin).PUT("/api/v1/plans/%d/enabled", map[string]any{"enabled": true}, plan.ID).
		AssertOK().
		AssertJSONPath("data.enabled", true).
		AssertJSONPath("data.updated_at", internalTime(savedAt))

	app.As(admin).PUT("/api/v1/plans/%d/enabled", map[string]any{"enabled": false}, plan.ID).
		AssertOK().
		AssertJSONPath("data.enabled", false).
		AssertJSONPath("data.health", "paused").
		AssertJSONPath("data.next_run_at", nil)
	app.AssertDatabaseHas("plans", testkit.Row{"id": plan.ID, "enabled": false})

	resumed := testkit.Decode[plans.PlanDataResponse](app.As(admin).PUT("/api/v1/plans/%d/enabled", map[string]any{"enabled": true}, plan.ID).AssertOK()).Data
	assert.True(t, resumed.Enabled)
	assert.NotEqual(t, internalTime(savedAt), resumed.UpdatedAt, "resuming is a save, so missed slots are not run")
	assert.Equal(t, plans.HealthNever, resumed.Health)

	app.As(admin).PUT("/api/v1/plans/%d/enabled", map[string]any{}, plan.ID).AssertInvalid("enabled")
	app.As(admin).PUT("/api/v1/plans/%d/enabled", map[string]any{"enabled": true}, plan.ID+100).AssertNotFound()
	app.PUT("/api/v1/plans/%d/enabled", map[string]any{"enabled": true}, plan.ID).AssertUnauthorized()
	app.As(app.Create.User()).PUT("/api/v1/plans/%d/enabled", map[string]any{"enabled": true}, plan.ID).AssertForbidden()
}

func internalTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func TestArchivesAPI_Download(t *testing.T) {
	app := testkit.New(t)
	admin := app.Create.User(factory.Admin())
	dir := t.TempDir()
	storage := app.Create.Storage(factory.LocalStorage(dir))
	plan := app.Create.Plan(factory.WithStorages(storage))
	run := app.Create.Run(plan, factory.WithRunStatus(runs.StatusSucceeded))
	content := []byte("archive bytes")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "shop"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "shop", "shop-20261017T030000Z-1.db.gz"), content, 0o600))
	archive := app.Create.Archive(run, storage, factory.ArchiveKey("shop/shop-20261017T030000Z-1.db.gz"), factory.ArchiveContent(content))

	resp := app.As(admin).GET("/api/v1/archives/%d/download", archive.ID).AssertOK()
	assert.Equal(t, content, resp.Body)
	assert.Equal(t, `attachment; filename=shop-20261017T030000Z-1.db.gz`, resp.Header.Get("Content-Disposition"))
	assert.Equal(t, "13", resp.Header.Get("Content-Length"))

	link := testkit.Decode[runs.DownloadLinkDataResponse](app.As(admin).POST("/api/v1/archives/%d/download-link", nil, archive.ID).AssertOK()).Data
	require.NotEmpty(t, link.Token)
	expires, err := time.Parse(time.RFC3339, link.ExpiresAt)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().Add(runs.DownloadLinkTTL), expires, 2*time.Second)
	byLink := app.GET("/api/v1/downloads/%s", link.Token).AssertOK()
	assert.Equal(t, content, byLink.Body, "a link needs no session")

	app.GET("/api/v1/downloads/%s", link.Token[:len(link.Token)-2]+"xx").AssertError(http.StatusForbidden, "archives.invalid_link")
	app.GET("/api/v1/downloads/not-a-token").AssertError(http.StatusForbidden, "archives.invalid_link")

	app.GET("/api/v1/archives/%d/download", archive.ID).AssertUnauthorized()
	app.As(app.Create.User()).GET("/api/v1/archives/%d/download", archive.ID).AssertForbidden()
	app.As(app.Create.User()).POST("/api/v1/archives/%d/download-link", nil, archive.ID).AssertForbidden()
	app.As(admin).GET("/api/v1/archives/%d/download", archive.ID+100).AssertError(http.StatusNotFound, "archives.not_found")

	missing := app.Create.Archive(run, storage, factory.ArchiveKey("shop/gone.db.gz"))
	app.As(admin).GET("/api/v1/archives/%d/download", missing.ID).AssertError(http.StatusBadGateway, "storages.unreachable")

	pruned := app.Create.Archive(run, storage, factory.ArchiveKey("shop/shop-20261017T030000Z-1.db.gz"), factory.ArchiveDeleted())
	app.As(admin).GET("/api/v1/archives/%d/download", pruned.ID).AssertError(http.StatusNotFound, "archives.not_found")
	app.As(admin).POST("/api/v1/archives/%d/download-link", nil, pruned.ID).AssertError(http.StatusNotFound, "archives.not_found")
}

func TestArchivesAPI_DownloadFromS3(t *testing.T) {
	app := testkit.New(t)
	admin := app.Create.User(factory.Admin())
	fake := testkit.NewFakeS3(t)
	bucket := app.Create.Storage(factory.S3Storage(
		storages.S3Config{Endpoint: fake.Endpoint, Region: "us-east-1", Bucket: fake.Bucket, Prefix: "nightly", UsePathStyle: true}, "key", "secret"))
	plan := app.Create.Plan(factory.WithStorages(bucket))
	run := app.Create.Run(plan, factory.WithRunStatus(runs.StatusSucceeded))
	content := []byte("in the bucket")
	d, err := storages.NewStorageService(app.Queries, app.Secrets).Driver(app.Ctx, bucket)
	require.NoError(t, err)
	t.Cleanup(func() { d.Close() })
	require.NoError(t, d.Put(app.Ctx, "shop/a.tar.gz", bytes.NewReader(content)))
	archive := app.Create.Archive(run, bucket, factory.ArchiveKey("shop/a.tar.gz"), factory.ArchiveContent(content))

	resp := app.As(admin).GET("/api/v1/archives/%d/download", archive.ID).AssertOK()
	assert.Equal(t, content, resp.Body)

	down := app.Create.Storage(factory.S3Storage(
		storages.S3Config{Endpoint: fmt.Sprintf("http://127.0.0.1:%d", closedPort(t)), Region: "us-east-1", Bucket: "b", UsePathStyle: true}, "key", "secret"))
	unreachable := app.Create.Archive(run, down, factory.ArchiveKey("shop/a.tar.gz"))
	app.As(admin).GET("/api/v1/archives/%d/download", unreachable.ID).AssertError(http.StatusBadGateway, "storages.unreachable")
}

func TestArchiveService_LinksExpire(t *testing.T) {
	app := testkit.New(t)
	storage := app.Create.Storage()
	run := app.Create.Run(app.Create.Plan(factory.WithStorages(storage)), factory.WithRunStatus(runs.StatusSucceeded))
	archive := app.Create.Archive(run, storage)
	service := runs.NewArchiveService(app.Queries, storages.NewStorageService(app.Queries, app.Secrets), app.Secrets)
	now := time.Now()

	token, expires, err := service.Link(app.Ctx, archive.ID, now)
	require.NoError(t, err)

	_, err = service.OpenLink(app.Ctx, token, expires)
	assert.ErrorIs(t, err, runs.ErrInvalidLink, "dead at expires_at")
	_, err = service.OpenLink(app.Ctx, token, now.Add(2*runs.DownloadLinkTTL))
	assert.ErrorIs(t, err, runs.ErrInvalidLink)
	other, err := secretbox.New(bytes.Repeat([]byte("k"), secretbox.KeySize))
	require.NoError(t, err)
	_, err = runs.NewArchiveService(app.Queries, storages.NewStorageService(app.Queries, other), other).OpenLink(app.Ctx, token, now)
	assert.ErrorIs(t, err, runs.ErrInvalidLink, "a link from another key is refused")
}
