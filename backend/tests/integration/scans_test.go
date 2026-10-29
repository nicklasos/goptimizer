package integration

import (
	"fmt"
	"os"
	"testing"
	"time"

	"app/internal/db"
	"app/internal/factory"
	"app/internal/scanner"
	"app/internal/scans"
	"app/tests/testkit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScans_StartFailsScansLeftRunning(t *testing.T) {
	app := testkit.New(t)
	interrupted := app.Create.Scan(app.Create.Folder(), factory.WithScanStatus(scans.StatusRunning))
	finished := app.Create.Scan(app.Create.Folder(), factory.WithScanStatus(scans.StatusSucceeded))

	require.NoError(t, app.ScanWorker().Start())
	t.Cleanup(app.ScanWorker().Stop)

	app.AssertDatabaseHas("scans", testkit.Row{"id": interrupted.ID, "status": "failed", "error": scans.ErrInterruptedRestart})
	app.AssertDatabaseHas("scans", testkit.Row{"id": finished.ID, "status": "succeeded", "error": nil})
}

func TestScans_StopFailsTheRunningScan(t *testing.T) {
	app := testkit.New(t)
	tr := newTree(t, app)
	for i := range 2000 {
		require.NoError(t, os.MkdirAll(tr.path(fmt.Sprintf("d%02d/e%03d", i/100, i%100)), 0o755))
	}
	worker := app.ScanWorker()
	require.NoError(t, worker.Start())

	scan, err := scans.NewScanService(app.Queries, worker.Wake).Start(app.Ctx, tr.folder.ID, scans.KindFull)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		scan, err := app.Queries.GetLatestScanOfFolder(app.Ctx, tr.folder.ID)
		return err == nil && scan.Status == string(scans.StatusRunning)
	}, 3*time.Second, 2*time.Millisecond)
	stopped := time.Now()
	worker.Stop()

	assert.Less(t, time.Since(stopped), 2*time.Second)
	app.AssertDatabaseHas("scans", testkit.Row{"id": scan.ID, "status": "failed", "error": scans.ErrInterruptedStop})
}

func TestScans_APausedFolderFailsItsQueuedScan(t *testing.T) {
	app := testkit.New(t)
	tr := newTree(t, app, factory.Paused())
	tr.write("a.png", "png")
	scan := app.Create.Scan(tr.folder, factory.WithScanKind(scans.KindFull))

	app.ScanWorker().Drain(app.Ctx)

	app.AssertDatabaseHas("scans", testkit.Row{"id": scan.ID, "status": "failed", "error": scanner.ErrFolderPaused.Error()})
	app.AssertDatabaseCount("images", 0)
}

func TestScans_EnqueueDue(t *testing.T) {
	app := testkit.New(t)
	now := time.Now()
	ago := func(d time.Duration) factory.ScanOption { return factory.StartedAt(now.Add(-d)) }
	succeeded := factory.WithScanStatus(scans.StatusSucceeded)
	failed := factory.WithScanStatus(scans.StatusFailed)
	full := factory.WithScanKind(scans.KindFull)

	newFolder := app.Create.Folder()
	paused := app.Create.Folder(factory.Paused())
	fresh := app.Create.Folder()
	app.Create.Scan(fresh, full, succeeded, ago(10*time.Minute))
	quickDue := app.Create.Folder()
	app.Create.Scan(quickDue, full, succeeded, ago(2*time.Hour))
	fullDue := app.Create.Folder()
	app.Create.Scan(fullDue, full, succeeded, ago(25*time.Hour))
	app.Create.Scan(fullDue, succeeded, ago(2*time.Hour))
	broken := app.Create.Folder()
	app.Create.Scan(broken, full, failed, ago(10*time.Minute))
	busy := app.Create.Folder()
	app.Create.Scan(busy, full, factory.WithScanStatus(scans.StatusRunning), ago(time.Hour))
	service := scans.NewScanService(app.Queries, nil)

	queued, err := service.EnqueueDue(app.Ctx, now)

	require.NoError(t, err)
	assert.Equal(t, 3, queued)
	queuedKinds := map[int64]string{}
	for _, folder := range []*db.Folder{newFolder, paused, fresh, quickDue, fullDue, broken, busy} {
		scan, err := app.Queries.GetLatestScanOfFolder(app.Ctx, folder.ID)
		if err == nil && scan.Status == string(scans.StatusQueued) {
			queuedKinds[folder.ID] = scan.Kind
			app.AssertDatabaseHas("scans", testkit.Row{"id": scan.ID, "trigger": "schedule"})
		}
	}
	assert.Equal(t, map[int64]string{newFolder.ID: "full", quickDue.ID: "quick", fullDue.ID: "full"}, queuedKinds)

	again, err := service.EnqueueDue(app.Ctx, now.Add(time.Minute))
	require.NoError(t, err)
	assert.Zero(t, again, "a folder with a scan queued or running gets no second one")
}
