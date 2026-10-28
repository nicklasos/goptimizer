package integration

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"database/sql"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"app/internal/db"
	"app/internal/errs"
	"app/internal/factory"
	"app/internal/plans"
	"app/internal/runs"
	"app/tests/testkit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// restore starts a restore of the archive through the API as the admin, lets the worker run
// everything queued, and returns the restore as the API shows it then.
func restore(t *testing.T, app *testkit.App, admin *db.User, archiveID int64, body plans.RestoreRequest) runs.RestoreDetailResponse {
	t.Helper()
	queued := testkit.Decode[runs.RestoreDetailDataResponse](
		app.As(admin).POST("/api/v1/archives/%d/restore", body, archiveID).
			AssertStatus(http.StatusAccepted).
			AssertJSONPath("data.status", "queued"),
	).Data
	app.Worker().Drain(app.Ctx)
	return testkit.Decode[runs.RestoreDetailDataResponse](app.As(admin).GET("/api/v1/restores/%d", queued.ID).AssertOK()).Data
}

func notes(t *testing.T, path string) []string {
	t.Helper()
	conn, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer conn.Close()
	rows, err := conn.Query("SELECT body FROM notes ORDER BY id")
	require.NoError(t, err)
	defer rows.Close()
	var bodies []string
	for rows.Next() {
		var body string
		require.NoError(t, rows.Scan(&body))
		bodies = append(bodies, body)
	}
	require.NoError(t, rows.Err())
	return bodies
}

func execSQLite(t *testing.T, path, statement string) {
	t.Helper()
	conn, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer conn.Close()
	_, err = conn.Exec(statement)
	require.NoError(t, err)
}

func TestRestoresAPI_SQLiteOverItsOwnSource(t *testing.T) {
	app := testkit.New(t)
	app.UseRealTool("sqlite3")
	admin := app.Create.User(factory.Admin())
	source := sqliteWithNotes(t, "first", "second")
	plan := app.Create.Plan(factory.WithPlanName("Notes"), factory.SQLiteSource(source), factory.WithStorages(app.Create.Storage(factory.LocalStorage(t.TempDir()))))
	backup := runPlan(t, app, admin, plan)
	require.Equal(t, runs.StatusSucceeded, backup.Status, backup.Log)
	archive := backup.Archives[0]

	execSQLite(t, source, "DELETE FROM notes WHERE body = 'first'; INSERT INTO notes (body) VALUES ('after the backup')")
	require.Equal(t, []string{"second", "after the backup"}, notes(t, source))

	app.As(admin).GET("/api/v1/archives/%d/restore-target", archive.ID).
		AssertOK().
		AssertJSONPath("data.source_kind", "sqlite").
		AssertJSONPath("data.sqlite.path", source)

	body := plans.RestoreRequest{SQLite: &plans.SQLiteTargetInput{Path: source}}
	app.As(admin).POST("/api/v1/archives/%d/restore", body, archive.ID).AssertFieldError("confirm", errs.ErrKeyRestoresConfirm)
	body.Confirm = source + " "
	app.As(admin).POST("/api/v1/archives/%d/restore", body, archive.ID).AssertFieldError("confirm", errs.ErrKeyRestoresConfirm)
	app.AssertDatabaseCount("restores", 0)

	body.Confirm = source
	done := restore(t, app, admin, archive.ID, body)

	require.Equal(t, runs.StatusSucceeded, done.Status, done.Log)
	assert.Equal(t, []string{"first", "second"}, notes(t, source), "the rows are back")
	kept, err := filepath.Glob(source + ".before-restore-*")
	require.NoError(t, err)
	require.NotEmpty(t, kept, "the replaced database is kept")
	var keptDB string
	for _, path := range kept {
		if !strings.HasSuffix(path, "-wal") && !strings.HasSuffix(path, "-shm") {
			keptDB = path
		}
	}
	require.NotEmpty(t, keptDB)
	assert.Equal(t, []string{"second", "after the backup"}, notes(t, keptDB), "the kept copy is the database as it was, its WAL included")
	assert.Contains(t, done.Log, "Integrity check: ok")
	assert.Contains(t, done.Log, "Kept the current database as "+keptDB)
	assert.Contains(t, done.Log, "Stop the application")
	require.NotNil(t, done.CreatedBy)
	assert.Equal(t, admin.Email, done.CreatedBy.Email)
	require.NotNil(t, done.Archive)
	assert.Equal(t, archive.ObjectKey, done.Archive.ObjectKey)
	assert.Equal(t, source, done.Target.SQLite.Path)
	assert.NotNil(t, done.DurationMs)
	t.Logf("restore log:\n%s", done.Log)
}

func TestRestoresAPI_SQLiteToAnotherPath(t *testing.T) {
	app := testkit.New(t)
	app.UseRealTool("sqlite3")
	admin := app.Create.User(factory.Admin())
	source := sqliteWithNotes(t, "kept")
	plan := app.Create.Plan(factory.SQLiteSource(source), factory.WithStorages(app.Create.Storage(factory.LocalStorage(t.TempDir()))))
	backup := runPlan(t, app, admin, plan)
	copyPath := filepath.Join(t.TempDir(), "copy.db")

	done := restore(t, app, admin, backup.Archives[0].ID, plans.RestoreRequest{SQLite: &plans.SQLiteTargetInput{Path: copyPath}})

	require.Equal(t, runs.StatusSucceeded, done.Status, done.Log)
	assert.Equal(t, []string{"kept"}, notes(t, copyPath))
	kept, _ := filepath.Glob(copyPath + ".before-restore-*")
	assert.Empty(t, kept, "nothing was there to keep")
	leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(copyPath), ".copy.db.restore-*"))
	assert.Empty(t, leftovers)

	t.Run("a directory that does not exist fails", func(t *testing.T) {
		failed := restore(t, app, admin, backup.Archives[0].ID, plans.RestoreRequest{SQLite: &plans.SQLiteTargetInput{Path: "/nonexistent-gogobackup/x.db"}})
		require.Equal(t, runs.StatusFailed, failed.Status)
		assert.Contains(t, *failed.Error, "the directory /nonexistent-gogobackup does not exist")
		require.NotEmpty(t, app.Mail.Sent())
		last := app.Mail.Sent()[len(app.Mail.Sent())-1]
		assert.Equal(t, "Restore failed: "+plan.Name, last.Subject)
		assert.Contains(t, last.Text, "The restore of "+plan.Name+" failed.")
		assert.Contains(t, last.Text, fmt.Sprintf("/plans/%d?tab=restores&restore=%d", plan.ID, failed.ID))
	})
}

func TestRestoresAPI_Files(t *testing.T) {
	app := testkit.New(t)
	admin := app.Create.User(factory.Admin())
	src := filepath.Join(t.TempDir(), "uploads")
	mtime := time.Date(2025, 5, 4, 3, 2, 1, 0, time.UTC)
	require.NoError(t, os.MkdirAll(filepath.Join(src, "2026", "private"), 0o755))
	files := map[string]struct {
		content string
		mode    fs.FileMode
	}{
		"readme.txt":          {"hello", 0o644},
		"2026/photo.jpg":      {"\x00\xff jpeg", 0o600},
		"2026/private/run.sh": {"#!/bin/sh\necho hi\n", 0o750},
		"2026/private/empty":  {"", 0o640},
	}
	for name, f := range files {
		path := filepath.Join(src, name)
		require.NoError(t, os.WriteFile(path, []byte(f.content), f.mode))
		require.NoError(t, os.Chmod(path, f.mode))
		require.NoError(t, os.Chtimes(path, mtime, mtime))
	}
	require.NoError(t, os.Chmod(filepath.Join(src, "2026", "private"), 0o700))
	require.NoError(t, os.Symlink("../readme.txt", filepath.Join(src, "2026", "readme-link")))
	require.NoError(t, os.Symlink("/etc/hosts", filepath.Join(src, "hosts-link")))

	dir := t.TempDir()
	plan := app.Create.Plan(factory.WithPlanName("Uploads"), factory.FilesSource(src), factory.WithStorages(app.Create.Storage(factory.LocalStorage(dir))))
	backup := runPlan(t, app, admin, plan)
	require.Equal(t, runs.StatusSucceeded, backup.Status, backup.Log)
	archive := backup.Archives[0]

	defaults := testkit.Decode[runs.RestoreTargetDataResponse](app.As(admin).GET("/api/v1/archives/%d/restore-target", archive.ID).AssertOK()).Data
	assert.Equal(t, filepath.Join(app.Config.WorkDir, "restores"), filepath.Dir(defaults.Files.Directory), "a new directory in the work directory")
	assert.True(t, strings.HasPrefix(filepath.Base(defaults.Files.Directory), "uploads-"))

	target := filepath.Join(t.TempDir(), "restored")
	done := restore(t, app, admin, archive.ID, plans.RestoreRequest{Files: &plans.FilesTargetInput{Directory: target}})

	require.Equal(t, runs.StatusSucceeded, done.Status, done.Log)
	restored := filepath.Join(target, strings.TrimPrefix(src, "/"))
	for name, f := range files {
		path := filepath.Join(restored, name)
		content, err := os.ReadFile(path)
		require.NoError(t, err, name)
		assert.Equal(t, f.content, string(content), name)
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, f.mode, info.Mode().Perm(), name)
		assert.True(t, info.ModTime().Equal(mtime), "%s keeps its mtime, got %s", name, info.ModTime())
	}
	info, err := os.Stat(filepath.Join(restored, "2026", "private"))
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o700), info.Mode().Perm(), "directories keep their mode")
	link, err := os.Readlink(filepath.Join(restored, "2026", "readme-link"))
	require.NoError(t, err)
	assert.Equal(t, "../readme.txt", link)
	_, err = os.Lstat(filepath.Join(restored, "hosts-link"))
	assert.ErrorIs(t, err, fs.ErrNotExist, "a link out of the target is skipped")
	assert.Contains(t, done.Log, "hosts-link: skipped, a link to /etc/hosts")
	assert.Contains(t, done.Log, "Extracted 4 files")

	t.Run("to the original locations, confirmed", func(t *testing.T) {
		require.NoError(t, os.WriteFile(filepath.Join(src, "readme.txt"), []byte("changed"), 0o644))
		require.NoError(t, os.Remove(filepath.Join(src, "2026", "photo.jpg")))
		body := plans.RestoreRequest{Files: &plans.FilesTargetInput{OriginalLocations: true}}
		app.As(admin).POST("/api/v1/archives/%d/restore", body, archive.ID).AssertFieldError("confirm", errs.ErrKeyRestoresConfirm)
		body.Confirm = plans.OriginalLocations

		done := restore(t, app, admin, archive.ID, body)

		require.Equal(t, runs.StatusSucceeded, done.Status, done.Log)
		content, err := os.ReadFile(filepath.Join(src, "readme.txt"))
		require.NoError(t, err)
		assert.Equal(t, "hello", string(content))
		content, err = os.ReadFile(filepath.Join(src, "2026", "photo.jpg"))
		require.NoError(t, err)
		assert.Equal(t, "\x00\xff jpeg", string(content))
		link, err := os.Readlink(filepath.Join(src, "hosts-link"))
		require.NoError(t, err, "a link outside the plan's paths is skipped, so the one there stays")
		assert.Equal(t, "/etc/hosts", link)
	})

	t.Run("a corrupted archive fails without touching the target", func(t *testing.T) {
		object := filepath.Join(dir, archive.ObjectKey)
		data, err := os.ReadFile(object)
		require.NoError(t, err)
		data[len(data)/2] ^= 0xff
		require.NoError(t, os.WriteFile(object, data, 0o644))
		untouched := filepath.Join(t.TempDir(), "never")

		failed := restore(t, app, admin, archive.ID, plans.RestoreRequest{Files: &plans.FilesTargetInput{Directory: untouched}})

		require.Equal(t, runs.StatusFailed, failed.Status, failed.Log)
		assert.Contains(t, *failed.Error, "the archive does not match the one stored")
		_, err = os.Stat(untouched)
		assert.ErrorIs(t, err, fs.ErrNotExist)
		entries, err := os.ReadDir(app.Config.WorkDir)
		require.NoError(t, err)
		assert.Empty(t, entries, "the work directory of the restore is removed")
	})
}

type tarEntry struct {
	name, link, content string
	typeflag            byte
}

// craftedArchive stores a tar.gz of entries as an archive of a files plan, as an attacker who
// could write to the storage would, with a matching checksum.
func craftedArchive(t *testing.T, app *testkit.App, entries ...tarEntry) (*db.Archive, *db.Plan) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		hdr := &tar.Header{Name: e.name, Linkname: e.link, Typeflag: e.typeflag, Mode: 0o644, Size: int64(len(e.content)), ModTime: time.Now()}
		if e.typeflag != tar.TypeReg {
			hdr.Size = 0
		}
		if e.typeflag == tar.TypeDir {
			hdr.Mode = 0o755
		}
		require.NoError(t, tw.WriteHeader(hdr))
		if e.typeflag == tar.TypeReg {
			_, err := tw.Write([]byte(e.content))
			require.NoError(t, err)
		}
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())

	dir := t.TempDir()
	storage := app.Create.Storage(factory.LocalStorage(dir))
	plan := app.Create.Plan(factory.FilesSource(t.TempDir()), factory.WithStorages(storage))
	key := "crafted/crafted.tar.gz"
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "crafted"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, key), buf.Bytes(), 0o644))
	archive := app.Create.Archive(app.Create.Run(plan, factory.WithRunStatus(runs.StatusSucceeded)), storage,
		factory.ArchiveKey(key), factory.ArchiveContent(buf.Bytes()))
	return archive, plan
}

func TestRestoresAPI_FilesRefuseEntriesThatLeadOut(t *testing.T) {
	app := testkit.New(t)
	admin := app.Create.User(factory.Admin())
	outside := t.TempDir()
	escape, err := filepath.Rel("/", outside)
	require.NoError(t, err)

	cases := []struct {
		name    string
		entries []tarEntry
		// prepare runs on the target directory before the restore.
		prepare func(target string)
		want    string
	}{
		{name: "a name with ..", entries: []tarEntry{{name: "a/../../../../../../" + escape + "/evil.txt", content: "x", typeflag: tar.TypeReg}}, want: "climbs out"},
		{name: "an absolute name", entries: []tarEntry{{name: "/" + escape + "/evil.txt", content: "x", typeflag: tar.TypeReg}}, want: "climbs out"},
		{name: "a link already in the target that leads out", prepare: func(target string) {
			require.NoError(t, os.MkdirAll(target, 0o755))
			require.NoError(t, os.Symlink(outside, filepath.Join(target, "srv")))
		}, entries: []tarEntry{{name: "srv/evil.txt", content: "x", typeflag: tar.TypeReg}}, want: "srv/evil.txt"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			archive, _ := craftedArchive(t, app, tc.entries...)
			target := filepath.Join(t.TempDir(), "target")
			if tc.prepare != nil {
				tc.prepare(target)
			}

			failed := restore(t, app, admin, archive.ID, plans.RestoreRequest{Files: &plans.FilesTargetInput{Directory: target}})

			require.Equal(t, runs.StatusFailed, failed.Status, failed.Log)
			assert.Contains(t, *failed.Error, tc.want)
			_, err := os.Stat(filepath.Join(outside, "evil.txt"))
			assert.ErrorIs(t, err, fs.ErrNotExist, "nothing is written outside the target")
		})
	}

	t.Run("a link that leads out, then a file through it", func(t *testing.T) {
		archive, _ := craftedArchive(t, app,
			tarEntry{name: "link", link: outside, typeflag: tar.TypeSymlink},
			tarEntry{name: "up", link: "../../../../../../../../" + escape, typeflag: tar.TypeSymlink},
			tarEntry{name: "link/evil.txt", content: "x", typeflag: tar.TypeReg},
			tarEntry{name: "up/evil.txt", content: "x", typeflag: tar.TypeReg},
			tarEntry{name: "inside", link: "link/evil.txt", typeflag: tar.TypeSymlink},
		)
		target := filepath.Join(t.TempDir(), "target")

		done := restore(t, app, admin, archive.ID, plans.RestoreRequest{Files: &plans.FilesTargetInput{Directory: target}})

		require.Equal(t, runs.StatusSucceeded, done.Status, done.Log)
		_, err := os.Stat(filepath.Join(outside, "evil.txt"))
		assert.ErrorIs(t, err, fs.ErrNotExist, "nothing is written outside the target")
		info, err := os.Lstat(filepath.Join(target, "link"))
		require.NoError(t, err)
		assert.True(t, info.IsDir(), "the link was skipped, so the file went into a plain directory")
		content, err := os.ReadFile(filepath.Join(target, "inside"))
		require.NoError(t, err, "a link inside the target is made")
		assert.Equal(t, "x", string(content))
		assert.Contains(t, done.Log, "link: skipped")
		assert.Contains(t, done.Log, "up: skipped")
	})

	t.Run("to the original locations, only the plan's paths", func(t *testing.T) {
		archive, _ := craftedArchive(t, app, tarEntry{name: escape + "/evil.txt", content: "x", typeflag: tar.TypeReg})

		failed := restore(t, app, admin, archive.ID, plans.RestoreRequest{Files: &plans.FilesTargetInput{OriginalLocations: true}, Confirm: plans.OriginalLocations})

		require.Equal(t, runs.StatusFailed, failed.Status, failed.Log)
		assert.Contains(t, *failed.Error, "is outside the plan's paths")
		_, err := os.Stat(filepath.Join(outside, "evil.txt"))
		assert.ErrorIs(t, err, fs.ErrNotExist)
	})
}

func TestRestoresAPI_Requests(t *testing.T) {
	app := testkit.New(t)
	admin := app.Create.User(factory.Admin())
	user := app.Create.User()
	storage := app.Create.Storage(factory.LocalStorage(t.TempDir()))
	plan := app.Create.Plan(factory.SQLiteSource("/srv/app/app.db"), factory.WithStorages(storage))
	run := app.Create.Run(plan, factory.WithRunStatus(runs.StatusSucceeded))
	archive := app.Create.Archive(run, storage, factory.ArchiveKey("p/p.db.gz"))
	body := plans.RestoreRequest{SQLite: &plans.SQLiteTargetInput{Path: "/srv/app/copy.db"}}

	app.POST("/api/v1/archives/%d/restore", body, archive.ID).AssertUnauthorized()
	app.As(user).POST("/api/v1/archives/%d/restore", body, archive.ID).AssertForbidden()
	app.As(admin).POST("/api/v1/archives/%d/restore", `{}`, archive.ID).AssertInvalid("sqlite")
	app.As(admin).POST("/api/v1/archives/%d/restore", plans.RestoreRequest{Files: &plans.FilesTargetInput{Directory: "/tmp/x"}}, archive.ID).AssertInvalid("sqlite", "files")

	queued := app.As(admin).POST("/api/v1/archives/%d/restore", body, archive.ID).
		AssertStatus(http.StatusAccepted).
		AssertJSONPath("data.status", "queued").
		AssertJSONPath("data.target.sqlite.path", "/srv/app/copy.db").
		AssertJSONPath("data.archive.object_key", "p/p.db.gz").
		AssertJSONPath("data.created_by.email", admin.Email).
		AssertJSONPath("data.secret_set", false)
	id := testkit.Decode[runs.RestoreDetailDataResponse](queued).Data.ID

	app.As(admin).POST("/api/v1/archives/%d/restore", body, archive.ID).AssertError(http.StatusConflict, errs.ErrKeyRestoresInProgress)
	app.As(admin).GET("/api/v1/plans/%d", plan.ID).AssertOK().AssertJSONPath("data.health", "restoring")
	app.As(admin).POST("/api/v1/plans/%d/run", nil, plan.ID).AssertStatus(http.StatusAccepted)

	app.As(admin).GET("/api/v1/plans/%d/restores", plan.ID).
		AssertOK().
		AssertJSONCount("data", 1).
		AssertJSONPath("data.0.id", id).
		AssertJSONPath("pagination.total", 1)
	app.As(admin).GET("/api/v1/restores/%d", id).AssertOK().AssertJSONPath("data.log", "")
	app.As(admin).GET("/api/v1/restores/%d", id+100).AssertError(http.StatusNotFound, errs.ErrKeyRestoresNotFound)
	app.As(admin).GET("/api/v1/plans/%d/restores", plan.ID+100).AssertNotFound()

	deleted := app.Create.Archive(run, storage, factory.ArchiveDeleted(), factory.ArchiveKey("p/old.db.gz"))
	app.As(admin).POST("/api/v1/archives/%d/restore", body, deleted.ID).AssertError(http.StatusNotFound, errs.ErrKeyArchivesNotFound)
	app.As(admin).GET("/api/v1/archives/%d/restore-target", deleted.ID).AssertError(http.StatusNotFound, errs.ErrKeyArchivesNotFound)
	app.As(admin).POST("/api/v1/archives/%d/restore", body, 9999).AssertError(http.StatusNotFound, errs.ErrKeyArchivesNotFound)

	other := app.Create.Archive(run, storage, factory.ArchiveKey("p/p.tar.gz"))
	app.As(admin).POST("/api/v1/archives/%d/restore", body, other.ID).AssertError(http.StatusConflict, errs.ErrKeyRestoresKindChanged)
}

// The worker never runs a restore and a run of one plan at once: whichever was queued first
// runs, the other waits for it.
func TestRestores_WaitForTheRunOfTheirPlan(t *testing.T) {
	app := testkit.New(t)
	app.UseRealTool("sqlite3")
	admin := app.Create.User(factory.Admin())
	source := sqliteWithNotes(t, "one")
	plan := app.Create.Plan(factory.SQLiteSource(source), factory.WithStorages(app.Create.Storage(factory.LocalStorage(t.TempDir()))))
	backup := runPlan(t, app, admin, plan)
	copyPath := filepath.Join(t.TempDir(), "copy.db")

	running := app.Create.Run(plan, factory.WithRunStatus(runs.StatusRunning))
	queued := testkit.Decode[runs.RestoreDetailDataResponse](app.As(admin).POST("/api/v1/archives/%d/restore",
		plans.RestoreRequest{SQLite: &plans.SQLiteTargetInput{Path: copyPath}}, backup.Archives[0].ID).AssertStatus(http.StatusAccepted)).Data

	app.Worker().Drain(app.Ctx)
	app.As(admin).GET("/api/v1/restores/%d", queued.ID).AssertOK().AssertJSONPath("data.status", "queued")
	_, err := os.Stat(copyPath)
	assert.ErrorIs(t, err, fs.ErrNotExist)

	_, err = runs.Finish(app.Ctx, app.Queries, running.ID, runs.Outcome{}, time.Now())
	require.NoError(t, err)
	app.Worker().Drain(app.Ctx)
	app.As(admin).GET("/api/v1/restores/%d", queued.ID).AssertOK().AssertJSONPath("data.status", "succeeded")
	assert.Equal(t, []string{"one"}, notes(t, copyPath))
}

func TestRestores_StartFailsRestoresLeftRunning(t *testing.T) {
	app := testkit.New(t)
	storage := app.Create.Storage()
	plan := app.Create.Plan(factory.WithStorages(storage))
	left := app.Create.Restore(app.Create.Archive(app.Create.Run(plan, factory.WithRunStatus(runs.StatusSucceeded)), storage), factory.WithRestoreStatus(runs.StatusRunning))

	worker := app.Worker()
	require.NoError(t, worker.Start())
	worker.Stop()

	app.AssertDatabaseHas("restores", testkit.Row{"id": left.ID, "status": "failed", "error": runs.ErrInterruptedRestart})
}
