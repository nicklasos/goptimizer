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
