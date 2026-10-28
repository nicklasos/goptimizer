package integration

import (
	"archive/tar"
	"compress/gzip"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"app/internal/db"
	"app/internal/errs"
	"app/internal/factory"
	"app/internal/plans"
	"app/internal/runs"
	"app/internal/storages"
	"app/tests/testkit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sqliteWithNotes is a real SQLite database in WAL mode, as an application keeps one, with rows.
func sqliteWithNotes(t *testing.T, notes ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "app.db")
	conn, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	_, err = conn.Exec("PRAGMA journal_mode=WAL; CREATE TABLE notes (id INTEGER PRIMARY KEY, body TEXT)")
	require.NoError(t, err)
	for _, note := range notes {
		_, err = conn.Exec("INSERT INTO notes (body) VALUES (?)", note)
		require.NoError(t, err)
	}
	t.Cleanup(func() { conn.Close() })
	return path
}

// runPlan starts a run of the plan through the API as the admin, lets the worker run everything
// queued, and returns the run as the API shows it then.
func runPlan(t *testing.T, app *testkit.App, admin *db.User, plan *db.Plan) runs.RunDetailResponse {
	t.Helper()
	queued := testkit.Decode[runs.RunDataResponse](
		app.As(admin).POST("/api/v1/plans/%d/run", nil, plan.ID).
			AssertStatus(http.StatusAccepted).
			AssertJSONPath("data.status", "queued").
			AssertJSONPath("data.trigger", "manual"),
	).Data
	app.Worker().Drain(app.Ctx)
	return testkit.Decode[runs.RunDetailDataResponse](app.As(admin).GET("/api/v1/runs/%d", queued.ID).AssertOK()).Data
}

func readObject(t *testing.T, app *testkit.App, storage *db.Storage, key string) []byte {
	t.Helper()
	d, err := storages.NewStorageService(app.Queries, app.Secrets).Driver(app.Ctx, storage)
	require.NoError(t, err)
	defer d.Close()
	r, err := d.Open(app.Ctx, key)
	require.NoError(t, err)
	defer r.Close()
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	return data
}

func gunzip(t *testing.T, data []byte) []byte {
	t.Helper()
	r, err := gzip.NewReader(strings.NewReader(string(data)))
	require.NoError(t, err)
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	return out
}

func TestRunsAPI_SQLitePlanToLocalAndS3(t *testing.T) {
	app := testkit.New(t)
	app.UseRealTool("sqlite3")
	admin := app.Create.User(factory.Admin())
	source := sqliteWithNotes(t, "first", "second", "third")
	dir := t.TempDir()
	local := app.Create.Storage(factory.WithStorageName("Disk"), factory.LocalStorage(dir))
	fake := testkit.NewFakeS3(t)
	bucket := app.Create.Storage(factory.WithStorageName("Bucket"), factory.S3Storage(
		storages.S3Config{Endpoint: fake.Endpoint, Region: "us-east-1", Bucket: fake.Bucket, Prefix: "nightly", UsePathStyle: true}, "key", "secret"))
	plan := app.Create.Plan(factory.WithPlanName("App DB"), factory.SQLiteSource(source), factory.WithStorages(local, bucket))

	run := runPlan(t, app, admin, plan)

	require.Equal(t, runs.StatusSucceeded, run.Status, run.Log)
	assert.Nil(t, run.Error)
	require.NotNil(t, run.StartedAt)
	require.NotNil(t, run.FinishedAt)
	require.NotNil(t, run.DurationMs)
	assert.GreaterOrEqual(t, *run.DurationMs, int64(0))
	require.Len(t, run.Archives, 2)
	assert.Contains(t, run.Log, "Disk: stored")
	assert.Contains(t, run.Log, "Bucket: stored")
	assert.Contains(t, run.Log, "Using ")

	archive := run.Archives[0]
	assert.Regexp(t, `^app-db/app-db-\d{8}T\d{6}Z-\d+\.db\.gz$`, archive.ObjectKey)
	assert.Equal(t, archive.ObjectKey, run.Archives[1].ObjectKey)
	assert.Equal(t, archive.SHA256, run.Archives[1].SHA256)
	require.NotNil(t, run.SizeBytes)
	assert.Equal(t, archive.SizeBytes, *run.SizeBytes)

	onDisk, err := os.ReadFile(filepath.Join(dir, archive.ObjectKey))
	require.NoError(t, err)
	assert.Equal(t, archive.SHA256, sha256Hex(string(onDisk)))
	assert.Equal(t, int64(len(onDisk)), archive.SizeBytes)
	inBucket := readObject(t, app, bucket, archive.ObjectKey)
	assert.Equal(t, onDisk, inBucket, "both storages hold the same archive")

	restored := filepath.Join(t.TempDir(), "restored.db")
	require.NoError(t, os.WriteFile(restored, gunzip(t, onDisk), 0o600))
	conn, err := sql.Open("sqlite", restored)
	require.NoError(t, err)
	defer conn.Close()
	var bodies string
	require.NoError(t, conn.QueryRow("SELECT group_concat(body, ',') FROM (SELECT body FROM notes ORDER BY id)").Scan(&bodies))
	assert.Equal(t, "first,second,third", bodies)

	app.AssertDatabaseCount("archives", 2, testkit.Row{"run_id": run.ID, "deleted_at": nil})
	app.As(admin).GET("/api/v1/plans/%d", plan.ID).
		AssertOK().
		AssertJSONPath("data.last_run.id", run.ID).
		AssertJSONPath("data.last_run.status", "succeeded").
		AssertJSONPath("data.last_run.size_bytes", archive.SizeBytes).
		AssertJSONPath("data.last_success_at", *run.FinishedAt)
	app.As(admin).GET("/api/v1/plans/%d/runs", plan.ID).
		AssertOK().
		AssertJSONCount("data", 1).
		AssertJSONPath("data.0.status", "succeeded").
		AssertJSONMissing("data.0.log")
	app.As(admin).GET("/api/v1/plans/%d/archives?storage_id=%d", plan.ID, bucket.ID).
		AssertOK().
		AssertJSONCount("data", 1).
		AssertJSONPath("data.0.storage_name", "Bucket").
		AssertJSONPath("data.0.storage_kind", "s3")
	assert.Empty(t, app.Mail.Sent())
}

// untar lists an archive: each entry's name, and the contents of its files.
func untar(t *testing.T, data []byte) map[string]string {
	t.Helper()
	r := tar.NewReader(strings.NewReader(string(gunzip(t, data))))
	entries := map[string]string{}
	for {
		hdr, err := r.Next()
		if errors.Is(err, io.EOF) {
			return entries
		}
		require.NoError(t, err)
		body, err := io.ReadAll(r)
		require.NoError(t, err)
		entries[hdr.Name] = string(body)
	}
}

func TestRunsAPI_FilesPlanWithExcludes(t *testing.T) {
	app := testkit.New(t)
	admin := app.Create.User(factory.Admin())
	root := t.TempDir()
	for path, body := range map[string]string{
		"site/index.html":          "<h1>hello</h1>",
		"site/uploads/photo.jpg":   "jpeg bytes",
		"site/debug.log":           "noise",
		"site/cache/page.html":     "cached",
		"site/uploads/.keep":       "",
		"site/uploads/old/old.log": "old noise",
	} {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(root, path), []byte(body), 0o644))
	}
	require.NoError(t, os.Symlink("index.html", filepath.Join(root, "site/home.html")))
	dir := t.TempDir()
	storage := app.Create.Storage(factory.LocalStorage(dir))
	site := filepath.Join(root, "site")
	plan := app.Create.Plan(factory.WithStorages(storage), factory.FilesSourceExcluding([]string{site}, []string{"*.log", filepath.Join(site, "cache")}))

	run := runPlan(t, app, admin, plan)

	require.Equal(t, runs.StatusSucceeded, run.Status, run.Log)
	require.Len(t, run.Archives, 1)
	assert.True(t, strings.HasSuffix(run.Archives[0].ObjectKey, ".tar.gz"))
	data, err := os.ReadFile(filepath.Join(dir, run.Archives[0].ObjectKey))
	require.NoError(t, err)
	entries := untar(t, data)

	prefix := strings.TrimPrefix(filepath.ToSlash(site), "/") + "/"
	assert.Equal(t, "<h1>hello</h1>", entries[prefix+"index.html"], "names are absolute paths without the leading slash")
	assert.Equal(t, "jpeg bytes", entries[prefix+"uploads/photo.jpg"])
	assert.Contains(t, entries, prefix+"uploads/.keep")
	assert.Contains(t, entries, prefix+"uploads/old/")
	assert.Contains(t, entries, prefix+"home.html")
	for name := range entries {
		assert.False(t, strings.HasSuffix(name, ".log"), "%s matches *.log", name)
		assert.NotContains(t, name, "cache", "the cache folder is excluded")
	}
	assert.Contains(t, run.Log, "skipped 3 excluded")
}

func TestRunsAPI_OneStorageFails(t *testing.T) {
	app := testkit.New(t)
	app.UseRealTool("sqlite3")
	admin := app.Create.User(factory.Admin(), factory.WithEmail("admin@example.com"))
	app.Create.User(factory.SuperAdmin(), factory.WithEmail("root@example.com"))
	app.Create.User(factory.Admin(), factory.Unverified(), factory.WithEmail("new-admin@example.com"))
	app.Create.User(factory.WithEmail("user@example.com"))
	working := app.Create.Storage(factory.WithStorageName("Working"), factory.LocalStorage(t.TempDir()))
	goneDir := t.TempDir()
	gone := app.Create.Storage(factory.WithStorageName("Gone"), factory.LocalStorage(goneDir))
	require.NoError(t, os.RemoveAll(goneDir))
	plan := app.Create.Plan(factory.WithPlanName("Shop"), factory.SQLiteSource(sqliteWithNotes(t, "x")), factory.WithStorages(working, gone))

	run := runPlan(t, app, admin, plan)

	require.Equal(t, runs.StatusFailed, run.Status, run.Log)
	require.NotNil(t, run.Error)
	assert.Contains(t, *run.Error, "1 of 2 storages failed: Gone:")
	require.Len(t, run.Archives, 1, "the storage that worked keeps its archive")
	assert.Equal(t, "Working", run.Archives[0].StorageName)
	assert.Contains(t, run.Log, "Gone: upload failed")
	assert.NotContains(t, run.Log, "retention", "retention waits for a run that stored everywhere")

	sent := app.Mail.Sent()
	var to []string
	for _, msg := range sent {
		to = append(to, msg.To...)
		assert.Equal(t, "Backup failed: Shop", msg.Subject)
		assert.Contains(t, msg.Text, *run.Error)
		assert.Contains(t, msg.Text, "Gone: upload failed", "quotes the end of the log")
		assert.Contains(t, msg.Text, fmt.Sprintf("http://localhost:5173/plans/%d\n", plan.ID), "links to the plan's page")
		assert.Contains(t, msg.HTML, "Open the plan")
	}
	slices.Sort(to)
	assert.Equal(t, []string{"admin@example.com", "root@example.com"}, to, "every admin with a verified email, each in their own message")
}

func TestRunsAPI_DumpFailure(t *testing.T) {
	app := testkit.New(t)
	admin := app.Create.User(factory.Admin())
	dir := t.TempDir()
	plan := app.Create.Plan(factory.SQLiteSource(filepath.Join(t.TempDir(), "missing.db")), factory.WithStorages(app.Create.Storage(factory.LocalStorage(dir))))

	run := runPlan(t, app, admin, plan)

	require.Equal(t, runs.StatusFailed, run.Status)
	assert.Contains(t, *run.Error, "the dump failed:")
	assert.Contains(t, *run.Error, "missing.db does not exist")
	assert.Empty(t, run.Archives)
	assert.Nil(t, run.SizeBytes)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, entry := range entries {
		inner, _ := os.ReadDir(filepath.Join(dir, entry.Name()))
		assert.Empty(t, inner, "no partial archive is left behind")
	}
	assert.Len(t, app.Mail.Sent(), 1)
}

func TestRunsAPI_RunNowRules(t *testing.T) {
	app := testkit.New(t)
	admin := app.Create.User(factory.Admin())
	user := app.Create.User()
	plan := app.Create.Plan()

	app.POST("/api/v1/plans/%d/run", nil, plan.ID).AssertUnauthorized()
	app.As(user).POST("/api/v1/plans/%d/run", nil, plan.ID).AssertForbidden()
	app.As(user).GET("/api/v1/plans/%d/runs", plan.ID).AssertForbidden()
	app.As(user).GET("/api/v1/plans/%d/archives", plan.ID).AssertForbidden()
	app.As(admin).POST("/api/v1/plans/999999/run", nil).AssertError(http.StatusNotFound, errs.ErrKeyPlansNotFound)
	app.As(admin).GET("/api/v1/plans/999999/runs").AssertError(http.StatusNotFound, errs.ErrKeyPlansNotFound)
	app.As(admin).GET("/api/v1/runs/999999").AssertError(http.StatusNotFound, errs.ErrKeyRunsNotFound)
	app.As(admin).GET("/api/v1/plans/%d/archives?storage_id=abc", plan.ID).AssertInvalid("storage_id")

	queued := app.Create.Run(plan)
	app.As(user).GET("/api/v1/runs/%d", queued.ID).AssertForbidden()
	app.As(admin).POST("/api/v1/plans/%d/run", nil, plan.ID).AssertError(http.StatusConflict, errs.ErrKeyPlansRunInProgress)

	ok, err := runs.Claim(app.Ctx, app.Queries, queued.ID, time.Now())
	require.NoError(t, err)
	require.True(t, ok)
	app.As(admin).POST("/api/v1/plans/%d/run", nil, plan.ID).AssertError(http.StatusConflict, errs.ErrKeyPlansRunInProgress)

	_, err = runs.Finish(app.Ctx, app.Queries, queued.ID, runs.Outcome{}, time.Now())
	require.NoError(t, err)
	app.As(admin).POST("/api/v1/plans/%d/run", nil, plan.ID).AssertStatus(http.StatusAccepted)

	disabled := app.Create.Plan(factory.Disabled())
	app.As(admin).POST("/api/v1/plans/%d/run", nil, disabled.ID).AssertStatus(http.StatusAccepted)
	app.As(admin).GET("/api/v1/plans/%d", disabled.ID).AssertOK().AssertJSONPath("data.last_run.status", "queued").AssertJSONPath("data.last_success_at", nil)
}

func TestRuns_StateTransitions(t *testing.T) {
	app := testkit.New(t)
	plan := app.Create.Plan()
	run := app.Create.Run(plan)
	other := app.Create.Run(plan)
	now := time.Now()

	first, err := runs.Claim(app.Ctx, app.Queries, run.ID, now)
	require.NoError(t, err)
	second, err := runs.Claim(app.Ctx, app.Queries, run.ID, now)
	require.NoError(t, err)
	assert.True(t, first)
	assert.False(t, second, "a run is claimed once")

	sameplan, err := runs.Claim(app.Ctx, app.Queries, other.ID, now)
	require.NoError(t, err)
	assert.False(t, sameplan, "a plan never runs twice at once")

	finishedQueued, err := runs.Finish(app.Ctx, app.Queries, other.ID, runs.Outcome{}, now)
	require.NoError(t, err)
	assert.False(t, finishedQueued, "a queued run cannot finish")

	done, err := runs.Finish(app.Ctx, app.Queries, run.ID, runs.Outcome{Err: errors.New("boom"), Log: "the log"}, now)
	require.NoError(t, err)
	again, err := runs.Finish(app.Ctx, app.Queries, run.ID, runs.Outcome{}, now)
	require.NoError(t, err)
	assert.True(t, done)
	assert.False(t, again, "a finished run stays finished")
	app.AssertDatabaseHas("runs", testkit.Row{"id": run.ID, "status": "failed", "error": "boom", "log": "the log"})

	nowFree, err := runs.Claim(app.Ctx, app.Queries, other.ID, now)
	require.NoError(t, err)
	assert.True(t, nowFree, "the next run of the plan starts once the first ended")
}

func TestRuns_StartFailsRunsLeftRunning(t *testing.T) {
	app := testkit.New(t)
	plan := app.Create.Plan()
	interrupted := app.Create.Run(plan, factory.WithRunStatus(runs.StatusRunning))
	finished := app.Create.Run(app.Create.Plan(), factory.WithRunStatus(runs.StatusSucceeded))

	require.NoError(t, app.Worker().Start())
	t.Cleanup(app.Worker().Stop)

	app.AssertDatabaseHas("runs", testkit.Row{"id": interrupted.ID, "status": "failed", "error": runs.ErrInterruptedRestart})
	app.AssertDatabaseHas("runs", testkit.Row{"id": finished.ID, "status": "succeeded", "error": nil})
}

// fakeTool writes a shell script that stands in for a dump program.
func fakeTool(t *testing.T, name, script string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755))
	return path
}

func TestRuns_StopKillsTheDump(t *testing.T) {
	app := testkit.New(t)
	admin := app.Create.User(factory.Admin())
	started := filepath.Join(t.TempDir(), "started")
	app.Tools.Install("sqlite3", fakeTool(t, "sqlite3", "echo copying >&2\ntouch '"+started+"'\nexec sleep 60\n"), "3.45.1")
	plan := app.Create.Plan(factory.SQLiteSource(sqliteWithNotes(t)), factory.WithStorages(app.Create.Storage(factory.LocalStorage(t.TempDir()))))
	worker := app.Worker()
	require.NoError(t, worker.Start())

	id := testkit.Decode[runs.RunDataResponse](app.As(admin).POST("/api/v1/plans/%d/run", nil, plan.ID).AssertStatus(http.StatusAccepted)).Data.ID
	require.Eventually(t, func() bool {
		_, err := os.Stat(started)
		return err == nil
	}, 10*time.Second, 20*time.Millisecond, "the API wakes the worker, which starts the dump")

	stopped := time.Now()
	worker.Stop()

	assert.Less(t, time.Since(stopped), 5*time.Second, "the dump program is killed, not waited for")
	run, err := app.Queries.GetRunByID(app.Ctx, id)
	require.NoError(t, err)
	assert.Equal(t, string(runs.StatusFailed), run.Status)
	assert.Equal(t, runs.ErrInterruptedStop, run.Error.String)
	assert.Contains(t, run.Log, "sqlite3: copying", "what the tool printed is in the log")
	assert.Len(t, app.Mail.Sent(), 1)
}

func TestRuns_MySQLDefaultsFile(t *testing.T) {
	app := testkit.New(t)
	admin := app.Create.User(factory.Admin())
	app.Tools.Install("mysqldump", fakeTool(t, "mysqldump", `defaults="${1#--defaults-extra-file=}"
echo "file: $defaults"
echo "mode: $(stat -c %a "$defaults" 2>/dev/null || stat -f %Lp "$defaults")"
cat "$defaults"
shift
echo "args: $*"
echo "mysqldump: [Warning] a note on stderr" >&2
`), "mysqldump  Ver 8.0.36")
	dir := t.TempDir()
	plan := app.Create.Plan(
		factory.MySQLSource(plans.MySQLConfig{Host: "db.internal", Port: 3307, User: "shop", Database: "shop_prod"}, `pa#ss"word`),
		factory.WithStorages(app.Create.Storage(factory.LocalStorage(dir))),
	)

	run := runPlan(t, app, admin, plan)

	require.Equal(t, runs.StatusSucceeded, run.Status, run.Log)
	assert.True(t, strings.HasSuffix(run.Archives[0].ObjectKey, ".sql.gz"))
	data, err := os.ReadFile(filepath.Join(dir, run.Archives[0].ObjectKey))
	require.NoError(t, err)
	out := string(gunzip(t, data))

	assert.Contains(t, out, "mode: 600")
	assert.Contains(t, out, "[client]\nprotocol=TCP\nhost=\"db.internal\"\nport=\"3307\"\nuser=\"shop\"\npassword='pa#ss\"word'\n")
	assert.Contains(t, out, "args: --single-transaction --routines --triggers --events --hex-blob --no-tablespaces shop_prod")
	assert.NotContains(t, run.Log, "pa#ss", "the password is not in the log")
	assert.Contains(t, run.Log, " mysqldump: [Warning] a note on stderr\n")
	assert.NotContains(t, run.Log, "mysqldump: mysqldump:", "a tool that names itself is not named twice")

	defaults := strings.TrimPrefix(strings.SplitN(out, "\n", 2)[0], "file: ")
	_, err = os.Stat(defaults)
	assert.True(t, os.IsNotExist(err), "the defaults file %s is removed after the dump", defaults)
}

func liveArchiveIDs(t *testing.T, app *testkit.App, plan *db.Plan, storage *db.Storage) []int64 {
	t.Helper()
	rows, err := app.Queries.ListLiveArchivesOfPlanStorage(app.Ctx, db.ListLiveArchivesOfPlanStorageParams{PlanID: plan.ID, StorageID: storage.ID})
	require.NoError(t, err)
	ids := make([]int64, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	return ids
}

func TestRuns_RetentionAfterARun(t *testing.T) {
	app := testkit.New(t)
	app.UseRealTool("sqlite3")
	admin := app.Create.User(factory.Admin())
	dir := t.TempDir()
	storage := app.Create.Storage(factory.WithStorageName("Disk"), factory.LocalStorage(dir))
	plan := app.Create.Plan(factory.SQLiteSource(sqliteWithNotes(t, "x")), factory.WithStorages(storage), factory.WithRetention(7, 0, 0))
	earlier := app.Create.Run(plan, factory.WithRunStatus(runs.StatusSucceeded))
	daysAgo := func(n int) time.Time { return time.Now().AddDate(0, 0, -n) }
	write := func(key string) string {
		path := filepath.Join(dir, key)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte("archive"), 0o644))
		return path
	}

	expired := app.Create.Archive(earlier, storage, factory.ArchiveKey("old/expired.db.gz"), factory.ArchiveCreatedAt(daysAgo(30)))
	expiredPath := write(expired.ObjectKey)
	app.Create.Archive(earlier, storage, factory.ArchiveKey("old/vanished.db.gz"), factory.ArchiveCreatedAt(daysAgo(20)))
	// A directory with a file in it cannot be deleted as an object: a storage that refuses.
	stuck := app.Create.Archive(earlier, storage, factory.ArchiveKey("old/stuck.db.gz"), factory.ArchiveCreatedAt(daysAgo(15)))
	stuckPath := write(stuck.ObjectKey + "/inside")
	recent := app.Create.Archive(earlier, storage, factory.ArchiveKey("old/recent.db.gz"), factory.ArchiveCreatedAt(daysAgo(2)))
	recentPath := write(recent.ObjectKey)

	run := runPlan(t, app, admin, plan)

	require.Equal(t, runs.StatusSucceeded, run.Status, run.Log)
	assert.NoFileExists(t, expiredPath)
	assert.FileExists(t, recentPath)
	assert.FileExists(t, stuckPath)
	assert.FileExists(t, filepath.Join(dir, run.Archives[0].ObjectKey), "the new archive is kept")
	assert.ElementsMatch(t, []int64{stuck.ID, recent.ID, run.Archives[0].ID}, liveArchiveIDs(t, app, plan, storage),
		"an object already gone counts as deleted; one that cannot be deleted stays listed")
	assert.Contains(t, run.Log, "could not delete old/stuck.db.gz, the next run tries again")
	assert.Contains(t, run.Log, "Disk: retention kept 3 archive(s), deleted 2")

	require.NoError(t, os.RemoveAll(filepath.Dir(stuckPath)))
	again := runPlan(t, app, admin, plan)

	require.Equal(t, runs.StatusSucceeded, again.Status, again.Log)
	assert.ElementsMatch(t, []int64{recent.ID, run.Archives[0].ID, again.Archives[0].ID}, liveArchiveIDs(t, app, plan, storage),
		"the next run's prune retries what the last one could not delete")
}

func TestRuns_EnqueueDue(t *testing.T) {
	app := testkit.New(t)
	created := utcTime(t, "2026-10-01T00:00:00Z")
	plan := app.Create.Plan(factory.CreatedAt(created))
	disabled := app.Create.Plan(factory.CreatedAt(created), factory.Disabled())
	manualOnly := app.Create.Plan(factory.CreatedAt(created), factory.WithSchedules())
	brandNew := app.Create.Plan(factory.CreatedAt(utcTime(t, "2026-10-15T04:00:00Z")))
	service := runs.NewRunService(app.Queries, app.TxRunner, nil)
	enqueueAt := func(at string) int {
		t.Helper()
		n, err := service.EnqueueDue(app.Ctx, utcTime(t, at))
		require.NoError(t, err)
		return n
	}
	lastSlot := func() time.Time {
		t.Helper()
		slot, err := app.Queries.LastScheduledSlot(app.Ctx, plan.ID)
		require.NoError(t, err)
		return slot.Time.UTC()
	}

	assert.Equal(t, 1, enqueueAt("2026-10-14T03:00:30Z"), "03:00 every day, half a minute later")
	assert.Equal(t, utcTime(t, "2026-10-14T03:00:00Z"), lastSlot())
	app.AssertDatabaseHas("runs", testkit.Row{"plan_id": plan.ID, "trigger": "schedule", "status": "queued"})
	assert.Equal(t, 0, enqueueAt("2026-10-14T03:00:30Z"), "the same tick again")
	assert.Equal(t, 0, enqueueAt("2026-10-14T03:01:30Z"), "the next tick")

	assert.Equal(t, 1, enqueueAt("2026-10-15T05:00:00Z"), "the service was down at 03:00: the missed slot runs once")
	assert.Equal(t, utcTime(t, "2026-10-15T03:00:00Z"), lastSlot())
	assert.Equal(t, 0, enqueueAt("2026-10-15T05:01:00Z"))

	assert.Equal(t, 2, enqueueAt("2026-10-20T03:30:00Z"), "after days of downtime the latest slot runs once, here and for the plan made on the 15th")
	app.AssertDatabaseCount("runs", 3, testkit.Row{"plan_id": plan.ID})
	app.AssertDatabaseCount("runs", 0, testkit.Row{"plan_id": disabled.ID})
	app.AssertDatabaseCount("runs", 0, testkit.Row{"plan_id": manualOnly.ID})
	app.AssertDatabaseCount("runs", 1, testkit.Row{"plan_id": brandNew.ID})

	n, err := app.Queries.EnqueueScheduledRun(app.Ctx, db.EnqueueScheduledRunParams{
		PlanID:       plan.ID,
		ScheduledFor: sql.NullTime{Time: utcTime(t, "2026-10-20T03:00:00Z"), Valid: true},
	})
	require.NoError(t, err)
	assert.Zero(t, n, "another process enqueuing the same slot adds nothing")
}

func TestRuns_SavingAPlanDoesNotRunAPassedSlot(t *testing.T) {
	app := testkit.New(t)
	now := time.Now().UTC()
	plan := app.Create.Plan(factory.CreatedAt(now.AddDate(0, 0, -30)), factory.Disabled())
	service := runs.NewRunService(app.Queries, app.TxRunner, nil)

	_, err := app.DB.ExecContext(app.Ctx, `UPDATE plans SET enabled = TRUE, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, plan.ID)
	require.NoError(t, err)

	n, err := service.EnqueueDue(app.Ctx, now.Add(time.Second))
	require.NoError(t, err)
	assert.Zero(t, n, "today's 03:00 passed before the plan was re-enabled")
	app.AssertDatabaseCount("runs", 0, testkit.Row{"plan_id": plan.ID})
}

func utcTime(t *testing.T, s string) time.Time {
	t.Helper()
	at, err := time.Parse(time.RFC3339, s)
	require.NoError(t, err)
	return at.UTC()
}
