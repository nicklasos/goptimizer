package integration

import (
	"errors"
	"testing"
	"time"

	"app/internal/factory"
	"app/internal/scans"
	"app/tests/testkit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScans_StateTransitions(t *testing.T) {
	app := testkit.New(t)
	older := app.Create.Scan(app.Create.Folder(), factory.StartedAt(time.Now().Add(-time.Minute)))
	newer := app.Create.Scan(app.Create.Folder())
	now := time.Now()

	finishedQueued, err := scans.Finish(app.Ctx, app.Queries, older.ID, scans.Counts{}, nil, now)
	require.NoError(t, err)
	assert.False(t, finishedQueued, "a queued scan cannot finish")

	first, ok, err := scans.ClaimNext(app.Ctx, app.Queries, now)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, older.ID, first.ID, "the oldest queued scan is claimed first")
	assert.Equal(t, string(scans.StatusRunning), first.Status)

	done, err := scans.Finish(app.Ctx, app.Queries, first.ID, scans.Counts{FilesNew: 3}, errors.New("boom"), now)
	require.NoError(t, err)
	again, err := scans.Finish(app.Ctx, app.Queries, first.ID, scans.Counts{}, nil, now)
	require.NoError(t, err)
	assert.True(t, done)
	assert.False(t, again, "a finished scan stays finished")
	app.AssertDatabaseHas("scans", testkit.Row{"id": first.ID, "status": "failed", "error": "boom", "files_new": 3})

	second, ok, err := scans.ClaimNext(app.Ctx, app.Queries, now)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, newer.ID, second.ID)

	_, ok, err = scans.ClaimNext(app.Ctx, app.Queries, now)
	require.NoError(t, err)
	assert.False(t, ok, "a running scan is not claimed again")
}

func TestScans_FinishKeepsTheLatestScansOfTheFolder(t *testing.T) {
	app := testkit.New(t)
	folder := app.Create.Folder()
	other := app.Create.Folder()
	start := time.Now().Add(-300 * time.Hour)
	succeeded := factory.WithScanStatus(scans.StatusSucceeded)
	var created []int64
	for i := range scans.Keep + 5 {
		created = append(created, app.Create.Scan(folder, succeeded, factory.StartedAt(start.Add(time.Duration(i)*time.Hour))).ID)
	}
	for range 3 {
		app.Create.Scan(other, succeeded, factory.StartedAt(start))
	}
	running := app.Create.Scan(folder, factory.WithScanStatus(scans.StatusRunning))

	done, err := scans.Finish(app.Ctx, app.Queries, running.ID, scans.Counts{}, nil, time.Now())

	require.NoError(t, err)
	assert.True(t, done)
	app.AssertDatabaseCount("scans", scans.Keep, testkit.Row{"folder_id": folder.ID})
	app.AssertDatabaseHas("scans", testkit.Row{"id": running.ID})
	for _, id := range created[:6] {
		app.AssertDatabaseMissing("scans", testkit.Row{"id": id})
	}
	app.AssertDatabaseHas("scans", testkit.Row{"id": created[6]})
	app.AssertDatabaseCount("scans", 3, testkit.Row{"folder_id": other.ID})
}

// With quick scans every 5 minutes and a full one every 30 days, the newest 100 scans can all be
// quick. Deleting the last successful full scan would make the folder due for a full scan again.
func TestScans_FinishKeepsTheLastSuccessfulFullScan(t *testing.T) {
	app := testkit.New(t)
	folder := app.Create.Folder(factory.WithScanIntervals(5, 720))
	start := time.Now().Add(-10 * time.Hour)
	full := app.Create.Scan(folder, factory.WithScanKind(scans.KindFull), factory.WithScanStatus(scans.StatusSucceeded), factory.StartedAt(start))
	failedFull := app.Create.Scan(folder, factory.WithScanKind(scans.KindFull), factory.WithScanStatus(scans.StatusFailed), factory.StartedAt(start.Add(time.Minute)))
	for i := range scans.Keep + 5 {
		app.Create.Scan(folder, factory.WithScanStatus(scans.StatusSucceeded), factory.StartedAt(start.Add(time.Duration(i+2)*5*time.Minute)))
	}
	running := app.Create.Scan(folder, factory.WithScanStatus(scans.StatusRunning))

	_, err := scans.Finish(app.Ctx, app.Queries, running.ID, scans.Counts{}, nil, time.Now())

	require.NoError(t, err)
	app.AssertDatabaseCount("scans", scans.Keep+1, testkit.Row{"folder_id": folder.ID})
	app.AssertDatabaseHas("scans", testkit.Row{"id": full.ID})
	app.AssertDatabaseMissing("scans", testkit.Row{"id": failedFull.ID})
	enqueued, err := scans.NewScanService(app.Queries, nil).EnqueueDue(app.Ctx, time.Now().Add(10*time.Minute))
	require.NoError(t, err)
	assert.Equal(t, 1, enqueued)
	app.AssertDatabaseHas("scans", testkit.Row{"folder_id": folder.ID, "status": "queued", "kind": "quick"})
}

func TestScans_FinishDeletesNothingWhileTheFolderHasFewScans(t *testing.T) {
	app := testkit.New(t)
	folder := app.Create.Folder()
	for range 3 {
		app.Create.Scan(folder, factory.WithScanStatus(scans.StatusFailed))
	}
	running := app.Create.Scan(folder, factory.WithScanStatus(scans.StatusRunning))

	_, err := scans.Finish(app.Ctx, app.Queries, running.ID, scans.Counts{}, nil, time.Now())

	require.NoError(t, err)
	app.AssertDatabaseCount("scans", 4, testkit.Row{"folder_id": folder.ID})
}
