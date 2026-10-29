package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"app/internal/db"
	"app/internal/errs"
	"app/internal/factory"
	"app/internal/images"
	"app/tests/testkit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// walk follows next_cursor from the first page to the last and returns every image's path, in order.
func walk(t *testing.T, app *testkit.App, user *db.User, folderID int64, query string) []string {
	t.Helper()
	var paths []string
	cursor := ""
	for range 100 {
		target := fmt.Sprintf("/api/v1/folders/%d/images?%s", folderID, query)
		if cursor != "" {
			target += "&cursor=" + url.QueryEscape(cursor)
		}
		page := testkit.Decode[images.ImagesPageResponse](app.As(user).GET(target).AssertOK()).Data
		for _, item := range page.Items {
			paths = append(paths, item.Path)
		}
		if page.NextCursor == nil {
			return paths
		}
		cursor = *page.NextCursor
	}
	t.Fatal("the pages never ended")
	return nil
}

func TestImagesAPI_ListsTheLatestChangeFirstOnePageAtATime(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User()
	folder := app.Create.Folder()
	other := app.Create.Folder()
	base := time.Now().Add(-time.Hour)
	tie := base.Add(30 * time.Minute)
	app.Create.Image(folder, factory.At("", "oldest.png"), factory.UpdatedAt(base))
	app.Create.Image(folder, factory.At("shop/2024", "a.jpg"), factory.UpdatedAt(tie), factory.Failed(3, time.Time{}))
	app.Create.Image(folder, factory.At("shop/2024", "b.jpg"), factory.UpdatedAt(tie))
	app.Create.Image(folder, factory.At("shop/2024", "c.jpg"), factory.UpdatedAt(tie), factory.Failed(1, time.Now().Add(time.Hour)))
	app.Create.Image(folder, factory.At("blog", "newest.gif"), factory.UpdatedAt(base.Add(50*time.Minute)), factory.Optimized(5000))
	app.Create.Image(other, factory.At("", "elsewhere.png"))

	assert.Equal(t, []string{"blog/newest.gif", "shop/2024/c.jpg", "shop/2024/b.jpg", "shop/2024/a.jpg", "oldest.png"},
		walk(t, app, user, folder.ID, "limit=2"))
	assert.Equal(t, []string{"shop/2024/c.jpg", "shop/2024/a.jpg"}, walk(t, app, user, folder.ID, "status=failed&limit=1"))
	assert.Empty(t, walk(t, app, user, folder.ID, "status=kept"))

	app.As(user).GET("/api/v1/folders/%d/images?limit=2", folder.ID).
		AssertOK().
		AssertJSONCount("data.items", 2).
		AssertJSONPath("data.order", "updated").
		AssertJSONPath("data.items.0.status", "optimized").
		AssertJSONPath("data.items.0.format", "gif").
		AssertJSONPath("data.items.0.original_size", 5000).
		AssertJSONPath("data.items.1.status", "failed").
		AssertJSONPath("data.items.1.attempts", 1).
		AssertJSONPath("data.items.1.original_size", nil)
	app.As(user).GET("/api/v1/folders/%d/images", folder.ID).
		AssertOK().
		AssertJSONCount("data.items", 5).
		AssertJSONPath("data.next_cursor", nil)
}

func TestImagesAPI_ListsOptimizedImagesByBytesSaved(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User()
	folder := app.Create.Folder()
	app.Create.Image(folder, factory.At("", "small.png"), factory.AsOnDisk(900, 1), factory.Optimized(1000))
	app.Create.Image(folder, factory.At("", "big.png"), factory.AsOnDisk(1000, 1), factory.Optimized(9000))
	app.Create.Image(folder, factory.At("", "tie-a.png"), factory.AsOnDisk(500, 1), factory.Optimized(1500))
	app.Create.Image(folder, factory.At("", "tie-b.png"), factory.AsOnDisk(2000, 1), factory.Optimized(3000))
	app.Create.Image(folder, factory.At("", "kept.png"), factory.WithImageStatus(images.StatusKept))

	assert.Equal(t, []string{"big.png", "tie-b.png", "tie-a.png", "small.png"},
		walk(t, app, user, folder.ID, "status=optimized&sort=saved&limit=1"))
	app.As(user).GET("/api/v1/folders/%d/images?status=optimized&sort=saved", folder.ID).AssertOK().AssertJSONPath("data.order", "saved")
	app.As(user).GET("/api/v1/folders/%d/images?sort=saved", folder.ID).AssertInvalid("sort")
	app.As(user).GET("/api/v1/folders/%d/images?status=kept&sort=saved", folder.ID).AssertInvalid("sort")
}

func TestImagesAPI_SearchesThePathAnywhereIgnoringCase(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User()
	folder := app.Create.Folder()
	app.Create.Image(folder, factory.At("uploads/2024", "Hero-Banner.PNG"))
	app.Create.Image(folder, factory.At("uploads/2025", "hero.jpg"), factory.WithImageStatus(images.StatusKept))
	app.Create.Image(folder, factory.At("", "hero.gif"))
	app.Create.Image(folder, factory.At("heroes", "team.png"))
	app.Create.Image(folder, factory.At("uploads", "logo.png"))

	assert.Equal(t, []string{"hero.gif", "heroes/team.png", "uploads/2024/Hero-Banner.PNG", "uploads/2025/hero.jpg"},
		walk(t, app, user, folder.ID, "q=HERO&limit=3"))
	assert.Equal(t, []string{"uploads/2025/hero.jpg"}, walk(t, app, user, folder.ID, "q=hero&status=kept"))
	assert.Equal(t, []string{"uploads/2024/Hero-Banner.PNG"}, walk(t, app, user, folder.ID, "q=2024%2Fhero"))
	assert.Empty(t, walk(t, app, user, folder.ID, "q=%25"))
	app.As(user).GET("/api/v1/folders/%d/images?q=hero&sort=updated", folder.ID).AssertOK().AssertJSONPath("data.order", "path")
}

func TestImagesAPI_ReadsTheReasonBack(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User()
	folder := app.Create.Folder()
	base := time.Now().Add(-time.Hour)
	app.Create.Image(folder, factory.UpdatedAt(base.Add(3*time.Minute)), factory.WithImageStatus(images.StatusKept),
		factory.WithReason("ssim_below_threshold: 0.7100, below 0.8500, at 128,64; lossless: no_saving: 1000 bytes to 990"))
	app.Create.Image(folder, factory.UpdatedAt(base.Add(2*time.Minute)), factory.WithImageStatus(images.StatusSkipped), factory.WithReason("apng"))
	app.Create.Image(folder, factory.UpdatedAt(base.Add(time.Minute)), factory.Failed(3, time.Time{}), factory.WithReason("a reason from another version"))
	app.Create.Image(folder, factory.UpdatedAt(base))

	app.As(user).GET("/api/v1/folders/%d/images", folder.ID).
		AssertOK().
		AssertJSONPath("data.items.0.reason.code", "ssim_below_threshold").
		AssertJSONPath("data.items.0.reason.detail", "0.7100, below 0.8500, at 128,64").
		AssertJSONPath("data.items.0.lossless_reason.code", "no_saving").
		AssertJSONPath("data.items.0.lossless_reason.detail", "1000 bytes to 990").
		AssertJSONPath("data.items.1.reason.code", "apng").
		AssertJSONPath("data.items.1.reason.detail", nil).
		AssertJSONPath("data.items.1.lossless_reason", nil).
		AssertJSONPath("data.items.2.reason.code", nil).
		AssertJSONPath("data.items.2.reason.detail", "a reason from another version").
		AssertJSONPath("data.items.2.next_attempt_at", nil).
		AssertJSONPath("data.items.3.reason", nil)
}

func TestImagesAPI_ListRules(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User()
	folder := app.Create.Folder()

	app.GET("/api/v1/folders/%d/images", folder.ID).AssertUnauthorized()
	app.As(user).GET("/api/v1/folders/%d/images", 999999).AssertError(http.StatusNotFound, errs.ErrKeyFoldersNotFound)
	app.As(user).GET("/api/v1/folders/%d/images?status=lost", folder.ID).AssertInvalid("status")
	app.As(user).GET("/api/v1/folders/%d/images?sort=size", folder.ID).AssertInvalid("sort")
	app.As(user).GET("/api/v1/folders/%d/images?limit=101", folder.ID).AssertInvalid("limit")
	app.As(user).GET("/api/v1/folders/%d/images?cursor=not-a-cursor", folder.ID).AssertInvalid("cursor")
}

func TestImagesAPI_RetryImage(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User()
	folder := app.Create.Folder()
	failed := app.Create.Image(folder, factory.Failed(3, time.Time{}), factory.WithReason("tool_failed: pngquant: signal: killed"))
	waiting := app.Create.Image(folder, factory.Failed(1, time.Now().Add(time.Hour)))
	kept := app.Create.Image(folder, factory.WithImageStatus(images.StatusKept), factory.WithReason("no_saving: 1000 bytes to 990"))

	app.As(user).POST("/api/v1/images/%d/retry", nil, failed.ID).
		AssertOK().
		AssertJSONPath("data.id", failed.ID).
		AssertJSONPath("data.status", "pending").
		AssertJSONPath("data.attempts", 0).
		AssertJSONPath("data.next_attempt_at", nil)
	app.AssertDatabaseHas("images", testkit.Row{"id": failed.ID, "status": "pending", "attempts": 0, "next_attempt_at": nil})
	app.As(user).POST("/api/v1/images/%d/retry", nil, waiting.ID).AssertOK()
	app.AssertDatabaseHas("images", testkit.Row{"id": waiting.ID, "status": "pending", "attempts": 0, "next_attempt_at": nil})

	app.As(user).POST("/api/v1/images/%d/retry", nil, failed.ID).AssertError(http.StatusConflict, errs.ErrKeyImagesNotFailed)
	app.As(user).POST("/api/v1/images/%d/retry", nil, kept.ID).AssertError(http.StatusConflict, errs.ErrKeyImagesNotFailed)
	app.AssertDatabaseHas("images", testkit.Row{"id": kept.ID, "status": "kept"})
	app.As(user).POST("/api/v1/images/%d/retry", nil, 999999).AssertError(http.StatusNotFound, errs.ErrKeyImagesNotFound)
	app.POST("/api/v1/images/%d/retry", nil, failed.ID).AssertUnauthorized()
}

func TestImagesAPI_RetryEveryFailedImageOfAFolder(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User()
	folder := app.Create.Folder()
	other := app.Create.Folder()
	for range 3 {
		app.Create.Image(folder, factory.Failed(3, time.Time{}))
	}
	app.Create.Image(folder, factory.WithImageStatus(images.StatusKept))
	elsewhere := app.Create.Image(other, factory.Failed(3, time.Time{}))

	app.As(user).POST("/api/v1/folders/%d/images/retry-failed", nil, folder.ID).AssertOK().AssertJSONPath("data.retried", 3)

	app.AssertDatabaseCount("images", 3, testkit.Row{"folder_id": folder.ID, "status": "pending", "attempts": 0})
	app.AssertDatabaseHas("images", testkit.Row{"folder_id": folder.ID, "status": "kept"})
	app.AssertDatabaseHas("images", testkit.Row{"id": elsewhere.ID, "status": "failed", "attempts": 3})
	app.As(user).POST("/api/v1/folders/%d/images/retry-failed", nil, folder.ID).AssertOK().AssertJSONPath("data.retried", 0)
	app.As(user).POST("/api/v1/folders/%d/images/retry-failed", nil, 999999).AssertError(http.StatusNotFound, errs.ErrKeyFoldersNotFound)
	app.POST("/api/v1/folders/%d/images/retry-failed", nil, folder.ID).AssertUnauthorized()
}

func TestImagesAPI_RetriedImagesAreOptimizedAgain(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User()
	useFakeTools(t, app, map[string]string{"pngquant": "exit 1"})
	tr := optTree(t, app, lossy())
	tr.put("a.png", encodePNG(t, smooth(64, 64)))
	for range images.MaxAttempts {
		tr.optimize()
		app.AssertDatabaseHas("images", testkit.Row{"name": "a.png", "status": "failed"})
		_, err := images.RequeueDue(app.Ctx, app.Queries, time.Now().Add(2*time.Hour))
		require.NoError(t, err)
	}
	app.AssertDatabaseHas("images", testkit.Row{"name": "a.png", "status": "failed", "attempts": images.MaxAttempts, "next_attempt_at": nil})

	app.As(user).POST("/api/v1/folders/%d/images/retry-failed", nil, tr.folder.ID).AssertOK().AssertJSONPath("data.retried", 1)
	useFakeTools(t, app, map[string]string{"pngquant": copyOver(t, encodePNGBest(t, smooth(64, 64)))})

	assert.Equal(t, 1, tr.optimize())
	app.AssertDatabaseHas("images", testkit.Row{"name": "a.png", "status": "optimized"})
}

func TestImagesAPI_DailySavings(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User()
	shop := app.Create.Folder()
	blog := app.Create.Folder()
	today := time.Now().UTC()
	app.Create.DailyStat(shop, today, 2, 3000, 1000)
	app.Create.DailyStat(blog, today, 1, 500, 400)
	app.Create.DailyStat(shop, today.AddDate(0, 0, -29), 4, 9000, 5000)
	app.Create.DailyStat(shop, today.AddDate(0, 0, -30), 9, 99000, 1)

	app.As(user).GET("/api/v1/daily-savings").
		AssertOK().
		AssertJSONCount("data", images.ChartDays).
		AssertJSONPath("data.0.day", images.Day(today.AddDate(0, 0, -29))).
		AssertJSONPath("data.0.optimized", 4).
		AssertJSONPath("data.0.bytes_saved", 4000).
		AssertJSONPath("data.1.optimized", 0).
		AssertJSONPath("data.1.bytes_saved", 0).
		AssertJSONPath("data.29.day", images.Day(today)).
		AssertJSONPath("data.29.optimized", 3).
		AssertJSONPath("data.29.bytes_saved", 2100)
	app.As(user).GET("/api/v1/daily-savings?folder_id=%d", blog.ID).
		AssertOK().
		AssertJSONCount("data", images.ChartDays).
		AssertJSONPath("data.0.bytes_saved", 0).
		AssertJSONPath("data.29.optimized", 1).
		AssertJSONPath("data.29.bytes_saved", 100)

	app.GET("/api/v1/daily-savings").AssertUnauthorized()
	app.As(user).GET("/api/v1/daily-savings?folder_id=999999").AssertError(http.StatusNotFound, errs.ErrKeyFoldersNotFound)
	app.As(user).GET("/api/v1/daily-savings?folder_id=-1").AssertInvalid("folder_id")
}
