package integration

import (
	"net/http"
	"testing"
	"time"

	"app/internal/errs"
	"app/internal/factory"
	"app/internal/images"
	"app/internal/scans"
	"app/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestScansAPI_StartScan(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User()
	folder := app.Create.Folder()

	app.As(user).POST("/api/v1/folders/%d/scans", map[string]any{"kind": "quick"}, folder.ID).
		AssertStatus(http.StatusAccepted).
		AssertJSONPath("data.folder_id", folder.ID).
		AssertJSONPath("data.kind", "quick").
		AssertJSONPath("data.trigger", "manual").
		AssertJSONPath("data.status", "queued").
		AssertJSONPath("data.started_at", nil)

	app.As(user).POST("/api/v1/folders/%d/scans", map[string]any{"kind": "full"}, folder.ID).
		AssertError(http.StatusConflict, errs.ErrKeyScansInProgress)
	app.AssertDatabaseCount("scans", 1)
}

func TestScansAPI_StartScanRules(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User()
	folder := app.Create.Folder()
	paused := app.Create.Folder(factory.Paused())
	running := app.Create.Folder()
	app.Create.Scan(running, factory.WithScanStatus(scans.StatusRunning))

	app.POST("/api/v1/folders/%d/scans", map[string]any{"kind": "quick"}, folder.ID).AssertUnauthorized()
	app.As(user).POST("/api/v1/folders/%d/scans", map[string]any{"kind": "deep"}, folder.ID).AssertInvalid("kind")
	app.As(user).POST("/api/v1/folders/%d/scans", `{}`, folder.ID).AssertInvalid("kind")
	app.As(user).POST("/api/v1/folders/%d/scans", map[string]any{"kind": "quick"}, 999999).AssertError(http.StatusNotFound, errs.ErrKeyFoldersNotFound)
	app.As(user).POST("/api/v1/folders/%d/scans", map[string]any{"kind": "quick"}, paused.ID).AssertError(http.StatusConflict, errs.ErrKeyFoldersPaused)
	app.As(user).POST("/api/v1/folders/%d/scans", map[string]any{"kind": "quick"}, running.ID).AssertError(http.StatusConflict, errs.ErrKeyScansInProgress)
	app.AssertDatabaseCount("scans", 1)
}

func TestScansAPI_ListScans(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User()
	folder := app.Create.Folder()
	other := app.Create.Folder()
	start := time.Now().Add(-48 * time.Hour)
	for i := range 21 {
		app.Create.Scan(folder, factory.WithScanStatus(scans.StatusSucceeded), factory.StartedAt(start.Add(time.Duration(i)*time.Hour)))
	}
	latest := app.Create.Scan(folder, factory.WithScanStatus(scans.StatusFailed), factory.WithScanError("the folder's path cannot be read"))
	app.Create.Scan(other)

	app.As(user).GET("/api/v1/folders/%d/scans", folder.ID).
		AssertOK().
		AssertJSONCount("data", 20).
		AssertJSONPath("data.0.id", latest.ID).
		AssertJSONPath("data.0.status", "failed").
		AssertJSONPath("data.0.error", "the folder's path cannot be read").
		AssertJSONPath("data.1.status", "succeeded")

	app.GET("/api/v1/folders/%d/scans", folder.ID).AssertUnauthorized()
	app.As(user).GET("/api/v1/folders/%d/scans", 999999).AssertError(http.StatusNotFound, errs.ErrKeyFoldersNotFound)
}

func TestScansAPI_FoldersShowTheirIndex(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User()
	shop := app.Create.Folder(factory.WithFolderName("Shop"))
	blog := app.Create.Folder(factory.WithFolderName("Blog"))
	empty := app.Create.Folder(factory.WithFolderName("Empty"))
	app.Create.Image(shop)
	app.Create.Image(shop)
	app.Create.Image(shop, factory.WithImageStatus(images.StatusGone))
	app.Create.Image(blog, factory.Optimized(5000))
	app.Create.Scan(shop, factory.WithScanStatus(scans.StatusSucceeded), factory.StartedAt(time.Now().Add(-time.Hour)))
	running := app.Create.Scan(shop, factory.WithScanStatus(scans.StatusRunning))

	app.As(user).GET("/api/v1/folders").
		AssertOK().
		AssertJSONPath("data.0.name", "Blog").
		AssertJSONPath("data.0.image_counts", map[string]int{"optimized": 1}).
		AssertJSONPath("data.0.last_scan", nil).
		AssertJSONPath("data.1.name", "Empty").
		AssertJSONPath("data.1.image_counts", map[string]int{}).
		AssertJSONPath("data.2.name", "Shop").
		AssertJSONPath("data.2.image_counts", map[string]int{"pending": 2, "gone": 1}).
		AssertJSONPath("data.2.last_scan.id", running.ID).
		AssertJSONPath("data.2.last_scan.status", "running")

	app.As(user).GET("/api/v1/folders/%d", shop.ID).
		AssertOK().
		AssertJSONPath("data.image_counts", map[string]int{"pending": 2, "gone": 1}).
		AssertJSONPath("data.last_scan.id", running.ID)
	app.As(user).GET("/api/v1/folders/%d", empty.ID).
		AssertOK().
		AssertJSONPath("data.image_counts", map[string]int{}).
		AssertJSONPath("data.last_scan", nil)
}

func TestScansAPI_AManualScanFillsTheIndex(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User()
	tr := newTree(t, app)
	tr.write("a.png", "png")
	tr.write("b/c.jpg", "jpeg")
	tr.write("b/readme.md", "text")

	app.As(user).POST("/api/v1/folders/%d/scans", map[string]any{"kind": "full"}, tr.folder.ID).AssertStatus(http.StatusAccepted)
	app.ScanWorker().Drain(app.Ctx)

	app.As(user).GET("/api/v1/folders/%d", tr.folder.ID).
		AssertOK().
		AssertJSONPath("data.image_counts", map[string]int{"pending": 2}).
		AssertJSONPath("data.last_scan.status", "succeeded").
		AssertJSONPath("data.last_scan.kind", "full").
		AssertJSONPath("data.last_scan.files_seen", 2).
		AssertJSONPath("data.last_scan.files_new", 2)
}

func TestScans_TheAPIWakesTheWorker(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User()
	tr := newTree(t, app)
	tr.write("a.png", "png")
	tr.write("sub/b.png", "png")
	require.NoError(t, app.ScanWorker().Start())
	t.Cleanup(app.ScanWorker().Stop)

	id := testkit.Decode[scans.ScanDataResponse](
		app.As(user).POST("/api/v1/folders/%d/scans", map[string]any{"kind": "full"}, tr.folder.ID).AssertStatus(http.StatusAccepted),
	).Data.ID

	require.Eventually(t, func() bool {
		scan, err := app.Queries.GetLatestScanOfFolder(app.Ctx, tr.folder.ID)
		return err == nil && scan.ID == id && scan.Status == string(scans.StatusSucceeded)
	}, 3*time.Second, 20*time.Millisecond, "woken at once, not at the next 5 s poll")
	app.AssertDatabaseHas("scans", testkit.Row{"id": id, "files_new": 2, "dirs_listed": 2, "error": nil})
	app.AssertDatabaseCount("images", 2, testkit.Row{"folder_id": tr.folder.ID, "status": "pending"})
}
