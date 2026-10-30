package integration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"app/internal/errs"
	"app/internal/factory"
	"app/internal/folders"
	"app/tests/testkit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// realDir is a new directory as the API stores it: t.TempDir() on macOS sits behind the /var symlink.
func realDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	return dir
}

func mkdir(t *testing.T, path string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(path, 0o755))
	return path
}

func folderBody(name, path string) map[string]any {
	return map[string]any{"name": name, "path": path}
}

func TestFoldersAPI_CRUD(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User()
	dir := realDir(t)

	resp := app.As(user).POST("/api/v1/folders", folderBody("  Shop  ", dir)).
		AssertOK().
		AssertJSONPath("data.name", "Shop").
		AssertJSONPath("data.kind", "local").
		AssertJSONPath("data.path", dir).
		AssertJSONPath("data.enabled", true).
		AssertJSONPath("data.scan_interval_minutes", 60).
		AssertJSONPath("data.full_scan_interval_hours", 24).
		AssertJSONPath("data.profile", folders.DefaultProfile())
	created := testkit.Decode[folders.FolderDataResponse](resp).Data

	app.As(user).GET("/api/v1/folders/%d", created.ID).AssertOK().AssertJSONPath("data.name", "Shop")

	other := mkdir(t, filepath.Join(realDir(t), "blog"))
	profile := folders.DefaultProfile()
	profile.Mode = folders.ModeLossless
	profile.Excludes = []string{"cache/**"}
	app.As(user).PUT("/api/v1/folders/%d", map[string]any{
		"name": "Blog", "path": other, "scan_interval_minutes": 15, "full_scan_interval_hours": 48, "profile": profile,
	}, created.ID).
		AssertOK().
		AssertJSONPath("data.name", "Blog").
		AssertJSONPath("data.path", other).
		AssertJSONPath("data.scan_interval_minutes", 15).
		AssertJSONPath("data.full_scan_interval_hours", 48).
		AssertJSONPath("data.profile.mode", "lossless").
		AssertJSONPath("data.profile.excludes", []string{"cache/**"})

	app.As(user).GET("/api/v1/folders").AssertOK().AssertJSONCount("data", 1).AssertJSONPath("data.0.name", "Blog")

	app.As(user).DELETE("/api/v1/folders/%d", created.ID).AssertOK()
	app.AssertDatabaseCount("folders", 0)
	app.As(user).GET("/api/v1/folders/%d", created.ID).AssertError(404, errs.ErrKeyFoldersNotFound)
	app.As(user).DELETE("/api/v1/folders/%d", created.ID).AssertError(404, errs.ErrKeyFoldersNotFound)
	app.As(user).PUT("/api/v1/folders/%d", folderBody("Gone", dir), created.ID).AssertError(404, errs.ErrKeyFoldersNotFound)
	assert.NotZero(t, created.ID)
}

func TestFoldersAPI_ListIsSortedByName(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User()
	app.Create.Folder(factory.WithFolderName("beta"))
	app.Create.Folder(factory.WithFolderName("Alpha"))

	app.As(user).GET("/api/v1/folders").
		AssertOK().
		AssertJSONPath("data.0.name", "Alpha").
		AssertJSONPath("data.1.name", "beta")
}

func TestFoldersAPI_PartialProfileTakesDefaults(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User()

	app.As(user).POST("/api/v1/folders", map[string]any{
		"name": "Photos", "path": realDir(t), "profile": map[string]any{"jpeg_max_quality": 70},
	}).
		AssertOK().
		AssertJSONPath("data.profile.jpeg_max_quality", 70).
		AssertJSONPath("data.profile.mode", "lossy").
		AssertJSONPath("data.profile.png_quality_max", 80)
}

func TestFoldersAPI_Validation(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User()
	dir := realDir(t)

	app.As(user).POST("/api/v1/folders", `{}`).AssertInvalid("name", "path")
	app.As(user).POST("/api/v1/folders", folderBody("   ", dir)).AssertFieldError("name", "validation.name.required")
	app.As(user).POST("/api/v1/folders", map[string]any{"name": "A", "path": dir, "kind": "ftp"}).AssertFieldError("kind", "validation.kind.oneof")
	app.As(user).POST("/api/v1/folders", map[string]any{"name": "A", "path": dir, "scan_interval_minutes": 4}).AssertInvalid("scan_interval_minutes")
	app.As(user).POST("/api/v1/folders", map[string]any{"name": "A", "path": dir, "scan_interval_minutes": 10081}).AssertInvalid("scan_interval_minutes")
	app.As(user).POST("/api/v1/folders", map[string]any{"name": "A", "path": dir, "full_scan_interval_hours": 0}).AssertInvalid("full_scan_interval_hours")
	app.As(user).POST("/api/v1/folders", map[string]any{"name": "A", "path": dir, "full_scan_interval_hours": 721}).AssertInvalid("full_scan_interval_hours")
	app.As(user).POST("/api/v1/folders", map[string]any{"name": "A", "path": dir, "profile": map[string]any{"png_quality_min": 90, "png_quality_max": 80}}).
		AssertFieldError("profile.png_quality_min", errs.ErrKeyFoldersPNGQualityRange)
	app.As(user).POST("/api/v1/folders", map[string]any{"name": "A", "path": dir, "profile": map[string]any{"excludes": []string{"[oops"}}}).
		AssertFieldError("profile.excludes", errs.ErrKeyFoldersExcludeInvalid)
	app.AssertDatabaseCount("folders", 0)

	app.Create.Folder(factory.WithFolderName("Shop"))
	app.As(user).POST("/api/v1/folders", folderBody("SHOP", dir)).AssertFieldError("name", errs.ErrKeyFoldersNameTaken)
}

func TestFoldersAPI_PathRules(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User()
	root := realDir(t)
	file := filepath.Join(root, "photo.jpg")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o644))

	refused := map[string]struct{ path, key string }{
		"relative":                 {"uploads/images", errs.ErrKeyFoldersPathNotAbsolute},
		"missing":                  {filepath.Join(root, "nope"), errs.ErrKeyFoldersPathNotFound},
		"a file":                   {file, errs.ErrKeyFoldersPathNotDirectory},
		"the root":                 {"/", errs.ErrKeyFoldersPathSystem},
		"a system root":            {"/etc", errs.ErrKeyFoldersPathSystem},
		"inside a system root":     {"/usr/share", errs.ErrKeyFoldersPathSystem},
		"lib anything":             {"/lib64/whatever", errs.ErrKeyFoldersPathSystem},
		"inside /var/lib":          {"/var/lib/mysql", errs.ErrKeyFoldersPathSystem},
		"a link into a system dir": {symlink(t, "/usr", filepath.Join(root, "usr-link")), errs.ErrKeyFoldersPathSystem},
	}
	for name, c := range refused {
		t.Run(name, func(t *testing.T) {
			app.As(user).POST("/api/v1/folders", folderBody("Refused", c.path)).AssertFieldError("path", c.key)
		})
	}

	t.Run("an unreadable directory", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root reads every directory")
		}
		locked := mkdir(t, filepath.Join(root, "locked"))
		require.NoError(t, os.Chmod(locked, 0o000))
		t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

		app.As(user).POST("/api/v1/folders", folderBody("Locked", locked)).AssertFieldError("path", errs.ErrKeyFoldersPathUnreadable)
	})

	t.Run("a symlink is stored as the directory it points to", func(t *testing.T) {
		target := mkdir(t, filepath.Join(realDir(t), "site", "public"))
		link := symlink(t, target, filepath.Join(realDir(t), "current"))

		app.As(user).POST("/api/v1/folders", folderBody("Linked", link+"/./")).AssertOK().AssertJSONPath("data.path", target)
	})

	app.AssertDatabaseCount("folders", 1)
}

func symlink(t *testing.T, target, link string) string {
	t.Helper()
	require.NoError(t, os.Symlink(target, link))
	return link
}

func TestFoldersAPI_Overlap(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User()
	base := realDir(t)
	parent := mkdir(t, filepath.Join(base, "a"))
	child := mkdir(t, filepath.Join(parent, "b"))
	sibling := mkdir(t, filepath.Join(base, "ab"))

	existing := testkit.Decode[folders.FolderDataResponse](app.As(user).POST("/api/v1/folders", folderBody("Parent", parent)).AssertOK()).Data

	app.As(user).POST("/api/v1/folders", folderBody("Same", parent)).AssertFieldError("path", errs.ErrKeyFoldersPathOverlaps)
	app.As(user).POST("/api/v1/folders", folderBody("Inside", child)).AssertFieldError("path", errs.ErrKeyFoldersPathOverlaps)
	app.As(user).POST("/api/v1/folders", folderBody("Around", base)).AssertFieldError("path", errs.ErrKeyFoldersPathOverlaps)
	app.As(user).POST("/api/v1/folders", folderBody("Sibling", sibling)).AssertOK()

	app.As(user).PUT("/api/v1/folders/%d", folderBody("Parent", child), existing.ID).AssertOK().AssertJSONPath("data.path", child)
	app.As(user).PUT("/api/v1/folders/%d", folderBody("Parent", sibling), existing.ID).AssertFieldError("path", errs.ErrKeyFoldersPathOverlaps)
}

func TestFoldersAPI_SetEnabled(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User()
	folder := app.Create.Folder()

	app.As(user).PUT("/api/v1/folders/%d/enabled", map[string]any{"enabled": false}, folder.ID).AssertOK().AssertJSONPath("data.enabled", false)
	app.AssertDatabaseHas("folders", testkit.Row{"id": folder.ID, "enabled": false})
	app.As(user).PUT("/api/v1/folders/%d/enabled", map[string]any{"enabled": true}, folder.ID).AssertOK().AssertJSONPath("data.enabled", true)

	app.As(user).PUT("/api/v1/folders/%d/enabled", `{}`, folder.ID).AssertInvalid("enabled")
	app.As(user).PUT("/api/v1/folders/%d/enabled", map[string]any{"enabled": true}, folder.ID+1).AssertError(404, errs.ErrKeyFoldersNotFound)
}

func TestFoldersAPI_EverySignedInUser(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User(factory.Unverified())
	folder := app.Create.Folder()

	app.GET("/api/v1/folders").AssertUnauthorized()
	app.As(user).GET("/api/v1/folders").AssertOK().AssertJSONCount("data", 1)
	app.As(user).POST("/api/v1/folders", folderBody("Mine", realDir(t))).AssertOK()
	app.As(user).PUT("/api/v1/folders/%d/enabled", map[string]any{"enabled": false}, folder.ID).AssertOK()
	app.As(user).DELETE("/api/v1/folders/%d", folder.ID).AssertOK()
	app.AssertDatabaseCount("folders", 1)
}

func TestFoldersAPI_ShowWhatOptimizingSaved(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User()
	shop := app.Create.Folder(factory.WithFolderName("A shop"))
	app.Create.Image(shop, factory.Optimized(5000), factory.AsOnDisk(3000, 1))
	app.Create.Image(shop, factory.Failed(3, time.Time{}))
	app.Create.DailyStat(shop, time.Now().AddDate(0, 0, -1), 3, 9000, 5000)
	app.Create.DailyStat(shop, time.Now(), 1, 5000, 3000)
	app.Create.Folder(factory.WithFolderName("B blog"))

	app.As(user).GET("/api/v1/folders").
		AssertOK().
		AssertJSONPath("data.0.bytes_saved", 6000).
		AssertJSONPath("data.0.bytes_before", 14000).
		AssertJSONPath("data.0.image_counts", map[string]int{"optimized": 1, "failed": 1}).
		AssertJSONPath("data.0.missing_tools", []string{}).
		AssertJSONPath("data.1.bytes_saved", 0).
		AssertJSONPath("data.1.bytes_before", 0)
	app.As(user).GET("/api/v1/folders/%d", shop.ID).
		AssertOK().
		AssertJSONPath("data.bytes_saved", 6000).
		AssertJSONPath("data.bytes_before", 14000)
}

func TestFoldersAPI_NameTheToolsAFolderWaitsFor(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User()
	app.Tools.Remove("pngquant")
	app.Tools.Remove("oxipng")
	lossless := folders.DefaultProfile()
	lossless.Mode = folders.ModeLossless
	app.Create.Folder(factory.WithFolderName("A lossy"))
	app.Create.Folder(factory.WithFolderName("B lossless"), factory.WithProfile(lossless))

	app.As(user).GET("/api/v1/folders").
		AssertOK().
		AssertJSONPath("data.0.missing_tools", []string{"pngquant"}).
		AssertJSONPath("data.1.missing_tools", []string{})
}
