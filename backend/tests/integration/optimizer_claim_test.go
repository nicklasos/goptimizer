package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"app/internal/factory"
	"app/internal/folders"
	"app/internal/images"
	"app/tests/testkit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func claimFolders(t *testing.T, app *testkit.App, n int) []int64 {
	t.Helper()
	var ids []int64
	for range n {
		job, ok, err := app.Pool().Claim(app.Ctx)
		require.NoError(t, err)
		if !ok {
			break
		}
		ids = append(ids, job.Image.FolderID)
	}
	return ids
}

func TestClaim_ABigFolderDoesNotStarveASmallOne(t *testing.T) {
	app := testkit.New(t)
	big := app.Create.Folder()
	small := app.Create.Folder()
	for range 50 {
		app.Create.Image(big)
	}
	for range 5 {
		app.Create.Image(small)
	}

	claims := claimFolders(t, app, 12)

	assert.Equal(t, []int64{big.ID, small.ID, big.ID, small.ID, big.ID, small.ID, big.ID, small.ID, big.ID, small.ID, big.ID, big.ID}, claims,
		"in flight, the folders take turns until the small one is done")
}

// With one worker nothing is in flight at a claim, and the folders take turns.
func TestClaim_WithOneWorkerTheFoldersTakeTurns(t *testing.T) {
	app := testkit.New(t)
	big := app.Create.Folder()
	small := app.Create.Folder()
	for range 20 {
		app.Create.Image(big)
	}
	for range 3 {
		app.Create.Image(small)
	}

	var claims []int64
	for {
		job, ok, err := app.Pool().Claim(app.Ctx)
		require.NoError(t, err)
		if !ok {
			break
		}
		claims = append(claims, job.Image.FolderID)
		_, err = images.Record(app.Ctx, app.TxRunner, job.Image, images.Outcome{Status: images.StatusKept, Size: job.Image.Size, MtimeNs: job.Image.MtimeNs}, time.Now())
		require.NoError(t, err)
	}

	assert.Equal(t, []int64{big.ID, small.ID, big.ID, small.ID, big.ID, small.ID, big.ID}, claims[:7])
	assert.Len(t, claims, 23)
}

func TestClaim_TheFolderWithFewerImagesInFlightGoesFirst(t *testing.T) {
	app := testkit.New(t)
	busy := app.Create.Folder()
	idle := app.Create.Folder()
	for range 3 {
		app.Create.Image(busy, factory.WithImageStatus(images.StatusOptimizing))
		app.Create.Image(busy)
	}
	app.Create.Image(idle)

	assert.Equal(t, []int64{idle.ID, busy.ID}, claimFolders(t, app, 2))
}

func TestClaim_TheOldestImageOfAFolderFirst(t *testing.T) {
	app := testkit.New(t)
	folder := app.Create.Folder()
	newer := app.Create.Image(folder, factory.At("", "newer.jpg"), factory.AsOnDisk(10, 2000))
	older := app.Create.Image(folder, factory.At("", "older.png"), factory.AsOnDisk(10, 1000))

	first, _, err := app.Pool().Claim(app.Ctx)
	require.NoError(t, err)
	second, _, err := app.Pool().Claim(app.Ctx)
	require.NoError(t, err)

	assert.Equal(t, older.ID, first.Image.ID)
	assert.Equal(t, newer.ID, second.Image.ID)
}

func TestClaim_Skips(t *testing.T) {
	app := testkit.New(t)
	paused := app.Create.Folder(factory.Paused())
	app.Create.Image(paused)
	open := app.Create.Folder()
	young := app.Create.Image(open, factory.AsOnDisk(10, time.Now().Add(-time.Minute).UnixNano()))
	notDue := app.Create.Image(open, factory.Failed(1, time.Now().Add(5*time.Minute)))
	gaveUp := app.Create.Image(open, factory.Failed(3, time.Time{}))
	for _, status := range []images.Status{images.StatusOptimized, images.StatusKept, images.StatusSkipped, images.StatusGone, images.StatusOptimizing} {
		app.Create.Image(open, factory.WithImageStatus(status))
	}

	_, ok, err := app.Pool().Claim(app.Ctx)

	require.NoError(t, err)
	assert.False(t, ok, "nothing is claimable")
	app.AssertDatabaseHas("images", testkit.Row{"id": young.ID, "status": "pending"})
	app.AssertDatabaseHas("images", testkit.Row{"id": notDue.ID, "status": "failed"})
	app.AssertDatabaseHas("images", testkit.Row{"id": gaveUp.ID, "status": "failed", "attempts": 3})
}

func TestClaim_AFailedImageIsTriedAgainWhenDue(t *testing.T) {
	app := testkit.New(t)
	folder := app.Create.Folder()
	image := app.Create.Image(folder, factory.Failed(1, time.Now().Add(10*time.Minute)))
	pool := app.Pool()

	_, ok, err := pool.Claim(app.Ctx)
	require.NoError(t, err)
	require.False(t, ok)

	pool.Optimizer.Now = func() time.Time { return time.Now().Add(11 * time.Minute) }
	job, ok, err := pool.Claim(app.Ctx)

	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, image.ID, job.Image.ID)
	assert.Equal(t, int64(1), job.Image.Attempts, "the attempts so far stay")
}

func TestClaim_AMissingToolLeavesItsImagesPending(t *testing.T) {
	app := testkit.New(t)
	app.Tools.Remove("pngquant")
	lossyFolder := app.Create.Folder()
	png := app.Create.Image(lossyFolder, factory.At("", "a.png"))
	jpg := app.Create.Image(lossyFolder, factory.At("", "a.jpg"))
	losslessProfile := folders.DefaultProfile()
	losslessProfile.Mode = folders.ModeLossless
	losslessFolder := app.Create.Folder(factory.WithProfile(losslessProfile))
	losslessPNG := app.Create.Image(losslessFolder, factory.At("", "b.png"))

	var claimed []int64
	for {
		job, ok, err := app.Pool().Claim(app.Ctx)
		require.NoError(t, err)
		if !ok {
			break
		}
		claimed = append(claimed, job.Image.ID)
	}

	assert.ElementsMatch(t, []int64{jpg.ID, losslessPNG.ID}, claimed, "lossy PNG needs pngquant; lossless PNG and JPEG do not")
	app.AssertDatabaseHas("images", testkit.Row{"id": png.ID, "status": "pending"})
}

// An image still optimizing at a start was being worked on when the process died: it is pending
// again with that attempt counted, and failed once it has used them all.
func TestPool_StartRecoversImagesLeftOptimizing(t *testing.T) {
	app := testkit.New(t)
	folder := app.Create.Folder(factory.Paused())
	fresh := app.Create.Image(folder, factory.WithImageStatus(images.StatusOptimizing))
	again := app.Create.Image(folder, factory.Failed(2, time.Time{}))
	_, err := app.DB.Exec(`UPDATE images SET status = 'optimizing' WHERE id = ?`, again.ID)
	require.NoError(t, err)
	pool := app.Pool()
	leftover := filepath.Join(app.Config.WorkDir, "job-123")
	unrelated := filepath.Join(app.Config.WorkDir, "notes.txt")
	require.NoError(t, os.MkdirAll(leftover, 0o700))
	require.NoError(t, os.WriteFile(unrelated, []byte("not the optimizer's"), 0o600))

	require.NoError(t, pool.Start())
	pool.Stop()

	app.AssertDatabaseHas("images", testkit.Row{"id": fresh.ID, "status": "pending", "attempts": 1})
	app.AssertDatabaseHas("images", testkit.Row{"id": again.ID, "status": "failed", "attempts": 3})
	assert.NoDirExists(t, leftover, "a job a previous run left is deleted")
	assert.FileExists(t, unrelated, "what is not a job stays")
}

func TestPool_StartRefusesAWorkDirOthersCanWrite(t *testing.T) {
	app := testkit.New(t)
	pool := app.Pool()
	require.NoError(t, osChmod(app.Config.WorkDir, 0o777))

	err := pool.Start()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "open to other users")
}

// The claim's queries must stay index lookups however many images there are: no full scan of a
// table, as SQLite's plan shows. A scan of an index is fine: the partial ones hold a few rows.
func TestClaim_QueriesUseIndexes(t *testing.T) {
	app := testkit.New(t)
	folder := app.Create.Folder()
	app.Create.Image(folder)
	now := time.Now().UTC()

	plans := map[string]string{
		"OldestClaimableImage":    `SELECT id, mtime_ns FROM images WHERE folder_id = ? AND status = 'pending' AND format = ? AND mtime_ns <= ? ORDER BY mtime_ns LIMIT 1`,
		"CountOptimizingByFolder": `SELECT folder_id, COUNT(*) AS count FROM images WHERE status = 'optimizing' GROUP BY folder_id`,
		"RequeueDueImages":        `UPDATE images SET status = 'pending', next_attempt_at = NULL, updated_at = ? WHERE status = 'failed' AND next_attempt_at IS NOT NULL AND datetime(next_attempt_at) <= datetime(?)`,
		"RecoverOptimizingImages": `UPDATE images SET status = CASE WHEN attempts + 1 >= ? THEN 'failed' ELSE 'pending' END, attempts = attempts + 1, updated_at = ? WHERE status = 'optimizing'`,
		"IsOptimizerOutput":       `SELECT EXISTS (SELECT 1 FROM optimizer_outputs WHERE sha256 = ?) AS found`,
		"ClaimImage":              `UPDATE images SET status = 'optimizing', updated_at = ? WHERE id = ? AND status = 'pending' RETURNING *`,
		"CountImagesByStatus":     `SELECT folder_id, status, COUNT(*) AS count FROM images GROUP BY folder_id, status`,
	}
	args := map[string][]any{
		"OldestClaimableImage":    {folder.ID, "png", now.UnixNano()},
		"CountOptimizingByFolder": nil,
		"RequeueDueImages":        {now, now},
		"RecoverOptimizingImages": {3, now},
		"IsOptimizerOutput":       {[]byte("x")},
		"ClaimImage":              {now, 1},
		"CountImagesByStatus":     nil,
	}
	for name, query := range plans {
		t.Run(name, func(t *testing.T) {
			rows, err := app.DB.Query("EXPLAIN QUERY PLAN "+query, args[name]...)
			require.NoError(t, err)
			defer rows.Close()
			var details []string
			for rows.Next() {
				var id, parent, notUsed int
				var detail string
				require.NoError(t, rows.Scan(&id, &parent, &notUsed, &detail))
				details = append(details, detail)
			}
			plan := strings.Join(details, "; ")
			require.NotEmpty(t, details)
			for _, d := range details {
				if strings.HasPrefix(d, "SCAN ") && d != "SCAN CONSTANT ROW" {
					assert.Contains(t, d, " USING ", "a scan of a whole table: %s", plan)
				}
			}
			t.Log(plan)
		})
	}
}
