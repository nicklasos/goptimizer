package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"app/internal/db"
	"app/internal/factory"
	"app/internal/folders"
	"app/internal/images"
	"app/internal/scanner"
	"app/internal/scans"
	"app/tests/testkit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tree is a folder on disk, its row and a scanner whose clock runs an hour ahead of the files, so
// no directory is within the settle time unless a test sets the clock itself.
type tree struct {
	t       *testing.T
	app     *testkit.App
	root    string
	folder  *db.Folder
	scanner *scanner.Scanner
	clock   time.Time
	// mtime is the next directory mtime touch sets: distinct, and well in the past.
	mtime time.Time
}

func newTree(t *testing.T, app *testkit.App, opts ...factory.FolderOption) *tree {
	t.Helper()
	root := realDir(t)
	tr := &tree{
		t:       t,
		app:     app,
		root:    root,
		folder:  app.Create.Folder(append([]factory.FolderOption{factory.WithFolderPath(root)}, opts...)...),
		scanner: scanner.New(app.Queries, app.TxRunner, app.Logger),
		clock:   time.Now().Add(time.Hour),
		mtime:   time.Now().Add(-24 * time.Hour),
	}
	tr.scanner.Now = func() time.Time { return tr.clock }
	return tr
}

func (tr *tree) path(rel string) string { return filepath.Join(tr.root, rel) }

func (tr *tree) write(rel, content string) {
	tr.t.Helper()
	require.NoError(tr.t, os.MkdirAll(filepath.Dir(tr.path(rel)), 0o755))
	require.NoError(tr.t, os.WriteFile(tr.path(rel), []byte(content), 0o644))
}

func (tr *tree) removeAll(rel string) {
	tr.t.Helper()
	require.NoError(tr.t, os.RemoveAll(tr.path(rel)))
}

// touch gives a directory a new mtime, as adding or removing an entry would.
func (tr *tree) touch(rel string) {
	tr.t.Helper()
	tr.mtime = tr.mtime.Add(time.Second)
	tr.setMtime(rel, tr.mtime)
}

func (tr *tree) setMtime(rel string, at time.Time) {
	tr.t.Helper()
	require.NoError(tr.t, os.Chtimes(tr.path(rel), at, at))
}

func (tr *tree) mtimeOf(rel string) time.Time {
	tr.t.Helper()
	info, err := os.Lstat(tr.path(rel))
	require.NoError(tr.t, err)
	return info.ModTime()
}

func (tr *tree) scan(kind scans.Kind) scans.Counts {
	tr.t.Helper()
	counts, err := tr.scanner.Scan(tr.app.Ctx, tr.folder.ID, kind, nil)
	require.NoError(tr.t, err)
	return counts
}

func (tr *tree) image(dir, name string) testkit.Row {
	return testkit.Row{"folder_id": tr.folder.ID, "dir": dir, "name": name}
}

func (tr *tree) imageWith(dir, name string, status images.Status) testkit.Row {
	row := tr.image(dir, name)
	row["status"] = string(status)
	return row
}

func TestScanner_IndexesImageFiles(t *testing.T) {
	app := testkit.New(t)
	tr := newTree(t, app)
	tr.write("a.png", "png")
	tr.write("d.gif", "gif")
	tr.write("notes.txt", "not an image")
	tr.write("sub/b.JPG", "jpeg")
	tr.write("sub/e.webp", "not indexed in v1")
	tr.write("sub/deeper/c.jpeg", "jpeg!")

	counts := tr.scan(scans.KindQuick)

	assert.Equal(t, scans.Counts{DirsSeen: 3, DirsListed: 3, FilesSeen: 4, FilesNew: 4}, counts, "a folder never listed is scanned in full")
	app.AssertDatabaseCount("images", 4)
	app.AssertDatabaseHas("images", testkit.Row{"folder_id": tr.folder.ID, "dir": "", "name": "a.png", "format": "png", "size": 3, "status": "pending"})
	app.AssertDatabaseHas("images", testkit.Row{"dir": "", "name": "d.gif", "format": "gif", "status": "pending"})
	app.AssertDatabaseHas("images", testkit.Row{"dir": "sub", "name": "b.JPG", "format": "jpeg", "status": "pending"})
	app.AssertDatabaseHas("images", testkit.Row{"dir": "sub/deeper", "name": "c.jpeg", "format": "jpeg", "size": 5})
	app.AssertDatabaseCount("dirs", 3)
	app.AssertDatabaseHas("dirs", testkit.Row{"folder_id": tr.folder.ID, "rel_path": "sub/deeper", "mtime_ns": tr.mtimeOf("sub/deeper").UnixNano()})
}

func TestScanner_ChangedFileGoesBackToPendingWithItsHistory(t *testing.T) {
	app := testkit.New(t)
	tr := newTree(t, app)
	tr.write("a.png", "small")
	info, err := os.Stat(tr.path("a.png"))
	require.NoError(t, err)
	app.Create.Image(tr.folder, factory.At("", "a.png"), factory.AsOnDisk(info.Size(), info.ModTime().UnixNano()), factory.Optimized(9000))

	assert.Zero(t, tr.scan(scans.KindFull).FilesChanged, "the row matches the file")
	app.AssertDatabaseHas("images", tr.imageWith("", "a.png", images.StatusOptimized))

	tr.write("a.png", "rewritten and larger")
	counts := tr.scan(scans.KindFull)

	assert.Equal(t, int64(1), counts.FilesChanged)
	row := tr.imageWith("", "a.png", images.StatusPending)
	row["size"], row["original_size"] = 20, 9000
	app.AssertDatabaseHas("images", row)
}

// The reason a file was left alone was about its old content.
func TestScanner_ChangedFileLosesItsReason(t *testing.T) {
	app := testkit.New(t)
	tr := newTree(t, app)
	tr.write("a.png", "small")
	info, err := os.Stat(tr.path("a.png"))
	require.NoError(t, err)
	app.Create.Image(tr.folder, factory.At("", "a.png"), factory.AsOnDisk(info.Size(), info.ModTime().UnixNano()),
		factory.WithImageStatus(images.StatusKept), factory.WithReason("no_saving: 5 bytes to 5"))

	tr.write("a.png", "rewritten and larger")
	tr.scan(scans.KindFull)

	row := tr.imageWith("", "a.png", images.StatusPending)
	row["reason"] = nil
	app.AssertDatabaseHas("images", row)
}

func TestScanner_DeletedFileIsGoneAndComesBackPending(t *testing.T) {
	app := testkit.New(t)
	tr := newTree(t, app)
	tr.write("a.png", "png")
	tr.write("b.png", "png")
	tr.touch("")
	tr.scan(scans.KindFull)

	tr.removeAll("a.png")
	tr.touch("")
	counts := tr.scan(scans.KindQuick)

	assert.Equal(t, int64(1), counts.FilesGone)
	app.AssertDatabaseHas("images", tr.imageWith("", "a.png", images.StatusGone))
	app.AssertDatabaseHas("images", tr.imageWith("", "b.png", images.StatusPending))

	tr.write("a.png", "back")
	tr.touch("")
	counts = tr.scan(scans.KindQuick)

	assert.Equal(t, int64(1), counts.FilesNew)
	app.AssertDatabaseCount("images", 2)
	app.AssertDatabaseHas("images", testkit.Row{"dir": "", "name": "a.png", "status": "pending", "size": 4})
}

func TestScanner_QuickScanFindsNewNestedDirectories(t *testing.T) {
	app := testkit.New(t)
	tr := newTree(t, app)
	tr.write("a.png", "png")
	tr.touch("")
	tr.scan(scans.KindFull)

	tr.write("new/y.png", "png")
	tr.write("new/deeper/x.png", "png")
	tr.touch("")
	counts := tr.scan(scans.KindQuick)

	assert.Equal(t, int64(3), counts.DirsListed, "the root, new and new/deeper")
	assert.Equal(t, int64(2), counts.FilesNew)
	app.AssertDatabaseHas("images", tr.imageWith("new", "y.png", images.StatusPending))
	app.AssertDatabaseHas("images", tr.imageWith("new/deeper", "x.png", images.StatusPending))
	app.AssertDatabaseHas("dirs", testkit.Row{"rel_path": "new/deeper", "mtime_ns": tr.mtimeOf("new/deeper").UnixNano()})
}

func TestScanner_RemovedDirectoryTakesEverythingUnderIt(t *testing.T) {
	app := testkit.New(t)
	tr := newTree(t, app)
	tr.write("a/b/c.png", "png")
	tr.write("a/d.png", "png")
	tr.write("a/x/y/z.png", "png")
	tr.write("a b/e.png", "a sibling that sorts inside a's range")
	tr.write("keep.png", "png")
	for _, dir := range []string{"", "a", "a/b", "a/x", "a/x/y", "a b"} {
		tr.touch(dir)
	}
	tr.scan(scans.KindFull)

	mtimeOfA := tr.mtimeOf("a")
	tr.removeAll("a/x")
	tr.setMtime("a", mtimeOfA)
	counts := tr.scan(scans.KindQuick)

	assert.Equal(t, int64(1), counts.FilesGone, "a is unchanged, so a/x is found missing by its own lstat")
	app.AssertDatabaseHas("images", tr.imageWith("a/x/y", "z.png", images.StatusGone))
	app.AssertDatabaseMissing("dirs", testkit.Row{"rel_path": "a/x/y"})

	tr.removeAll("a")
	tr.touch("")
	counts = tr.scan(scans.KindQuick)

	assert.Equal(t, int64(2), counts.FilesGone, "the root's listing no longer has a")
	app.AssertDatabaseHas("images", tr.imageWith("a/b", "c.png", images.StatusGone))
	app.AssertDatabaseHas("images", tr.imageWith("a", "d.png", images.StatusGone))
	app.AssertDatabaseHas("images", tr.imageWith("a b", "e.png", images.StatusPending))
	app.AssertDatabaseHas("images", tr.imageWith("", "keep.png", images.StatusPending))
	app.AssertDatabaseCount("dirs", 2, testkit.Row{"folder_id": tr.folder.ID})
	app.AssertDatabaseHas("dirs", testkit.Row{"rel_path": "a b"})
}

func TestScanner_Excludes(t *testing.T) {
	app := testkit.New(t)
	profile := folders.DefaultProfile()
	profile.Excludes = []string{"cache/**", "**/*.gif"}
	tr := newTree(t, app, factory.WithProfile(profile))
	tr.write("cache/x.png", "png")
	tr.write("cache/sub/y.png", "png")
	tr.write("a.gif", "gif")
	tr.write("deep/z.gif", "gif")
	tr.write("deep/w.png", "png")
	tr.write("b.png", "png")

	counts := tr.scan(scans.KindFull)

	assert.Equal(t, int64(2), counts.FilesNew)
	app.AssertDatabaseCount("images", 2)
	app.AssertDatabaseHas("images", tr.image("", "b.png"))
	app.AssertDatabaseHas("images", tr.image("deep", "w.png"))
	app.AssertDatabaseCount("dirs", 2)
	app.AssertDatabaseMissing("dirs", testkit.Row{"rel_path": "cache"})
}

func TestScanner_SymlinksAreNeitherFollowedNorIndexed(t *testing.T) {
	app := testkit.New(t)
	tr := newTree(t, app)
	outside := realDir(t)
	require.NoError(t, os.WriteFile(filepath.Join(outside, "outside.png"), []byte("png"), 0o644))
	tr.write("b.png", "png")
	require.NoError(t, os.Symlink(outside, tr.path("linked-dir")))
	require.NoError(t, os.Symlink(tr.path("b.png"), tr.path("linked.png")))

	tr.scan(scans.KindFull)

	app.AssertDatabaseCount("images", 1)
	app.AssertDatabaseHas("images", tr.image("", "b.png"))
	app.AssertDatabaseCount("dirs", 1)
}

func TestScanner_DirectoryReplacedBySymlinkIsGone(t *testing.T) {
	app := testkit.New(t)
	tr := newTree(t, app)
	tr.write("sub/a.png", "png")
	tr.touch("sub")
	tr.scan(scans.KindFull)

	outside := realDir(t)
	require.NoError(t, os.WriteFile(filepath.Join(outside, "a.png"), []byte("png"), 0o644))
	tr.removeAll("sub")
	require.NoError(t, os.Symlink(outside, tr.path("sub")))
	counts := tr.scan(scans.KindQuick)

	assert.Equal(t, int64(1), counts.FilesGone)
	app.AssertDatabaseHas("images", tr.imageWith("sub", "a.png", images.StatusGone))
}

func TestScanner_DeletesOnlyOldTempFiles(t *testing.T) {
	app := testkit.New(t)
	tr := newTree(t, app)
	tr.write(".goptimizer-1234.tmp", "half-written result")
	tr.write("photo.png", "png")

	tr.clock = time.Now().Add(30 * time.Minute)
	tr.scan(scans.KindFull)
	assert.FileExists(t, tr.path(".goptimizer-1234.tmp"), "a temp file younger than an hour may still be in use")

	tr.clock = time.Now().Add(2 * time.Hour)
	tr.scan(scans.KindFull)
	assert.NoFileExists(t, tr.path(".goptimizer-1234.tmp"))
	assert.FileExists(t, tr.path("photo.png"))
	app.AssertDatabaseCount("images", 1)
}

func TestScanner_ASecondScanWritesNothing(t *testing.T) {
	app := testkit.New(t)
	tr := newTree(t, app)
	tr.write("a.png", "png")
	tr.write("sub/b.gif", "gif")
	tr.write("sub/c.txt", "text")
	tr.scan(scans.KindFull)
	app.AssertDatabaseCount("images", 2, testkit.Row{"updated_at": tr.clock.UTC()})

	tr.clock = tr.clock.Add(time.Minute)
	full := tr.scan(scans.KindFull)
	quick := tr.scan(scans.KindQuick)

	assert.Equal(t, scans.Counts{DirsSeen: 2, DirsListed: 2, FilesSeen: 2}, full)
	assert.Equal(t, scans.Counts{DirsSeen: 2}, quick, "nothing changed, so nothing is listed")
	app.AssertDatabaseCount("images", 0, testkit.Row{"updated_at": tr.clock.UTC()})
}

func TestScanner_InPlaceRewriteIsFoundByTheFullScanOnly(t *testing.T) {
	app := testkit.New(t)
	tr := newTree(t, app)
	tr.write("a.png", "png")
	tr.touch("")
	tr.scan(scans.KindFull)

	rootMtime := tr.mtimeOf("")
	tr.write("a.png", "a longer png")
	tr.setMtime("", rootMtime)

	assert.Zero(t, tr.scan(scans.KindQuick).FilesChanged, "the directory's mtime did not move")
	app.AssertDatabaseHas("images", testkit.Row{"name": "a.png", "size": 3})

	assert.Equal(t, int64(1), tr.scan(scans.KindFull).FilesChanged)
	app.AssertDatabaseHas("images", testkit.Row{"name": "a.png", "size": 12, "status": "pending"})
}

func TestScanner_DirectoryChangedWithinTheSettleTimeIsListedAgain(t *testing.T) {
	for _, tc := range []struct {
		name      string
		listedAt  time.Duration
		findsLate bool
	}{
		{"listed 1s after its mtime", time.Second, true},
		{"listed 10s after its mtime", 10 * time.Second, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := testkit.New(t)
			tr := newTree(t, app)
			mtime := time.Now().Add(-time.Hour).Truncate(time.Second)
			tr.write("a.png", "png")
			tr.setMtime("", mtime)
			tr.clock = mtime.Add(tc.listedAt)
			tr.scan(scans.KindFull)

			// A file created in the same mtime tick as the listing leaves the mtime as it was.
			tr.write("late.png", "png")
			tr.setMtime("", mtime)
			tr.clock = mtime.Add(time.Minute)
			tr.scan(scans.KindQuick)

			if tc.findsLate {
				app.AssertDatabaseHas("images", tr.image("", "late.png"))
			} else {
				app.AssertDatabaseMissing("images", tr.image("", "late.png"))
			}
		})
	}
}

func TestScanner_StopsForAPausedFolder(t *testing.T) {
	app := testkit.New(t)
	tr := newTree(t, app, factory.Paused())
	tr.write("a.png", "png")

	_, err := tr.scanner.Scan(app.Ctx, tr.folder.ID, scans.KindFull, nil)

	assert.ErrorIs(t, err, scanner.ErrFolderPaused)
	app.AssertDatabaseCount("images", 0)
}

func TestScanner_FailsWhenTheFolderPathIsGone(t *testing.T) {
	app := testkit.New(t)
	tr := newTree(t, app)
	tr.write("a.png", "png")
	tr.scan(scans.KindFull)
	tr.removeAll("")

	_, err := tr.scanner.Scan(app.Ctx, tr.folder.ID, scans.KindQuick, nil)

	assert.ErrorContains(t, err, "the folder's path cannot be read")
	app.AssertDatabaseHas("images", tr.imageWith("", "a.png", images.StatusPending))
}

// A scan stopped after any directory, or before its last commit, leaves rows the next quick scan
// completes. The tree has more files than one batch, so some stops land after a commit.
func TestScanner_AScanStoppedAnywhereConverges(t *testing.T) {
	const dirs, perDir = 30, 50
	app := testkit.New(t)
	root := realDir(t)
	for d := range dirs {
		require.NoError(t, os.Mkdir(filepath.Join(root, fmt.Sprintf("d%02d", d)), 0o755))
		for f := range perDir {
			require.NoError(t, os.WriteFile(filepath.Join(root, fmt.Sprintf("d%02d/f%02d.png", d, f)), nil, 0o644))
		}
	}
	scan := scanner.New(app.Queries, app.TxRunner, app.Logger)
	scan.Now = func() time.Time { return time.Now().Add(time.Hour) }

	for _, stopAfter := range []int64{1, 12, 21, 25, dirs + 1} {
		t.Run(fmt.Sprintf("stopped after listing %d directories", stopAfter), func(t *testing.T) {
			folder := app.Create.Folder(factory.WithFolderPath(root))
			t.Cleanup(func() { _, _ = app.Queries.DeleteFolder(app.Ctx, folder.ID) })
			ctx, cancel := context.WithCancel(app.Ctx)
			defer cancel()
			_, err := scan.Scan(ctx, folder.ID, scans.KindFull, func(c scans.Counts) {
				if c.DirsListed >= stopAfter {
					cancel()
				}
			})
			require.Error(t, err)

			_, err = scan.Scan(app.Ctx, folder.ID, scans.KindQuick, nil)

			require.NoError(t, err)
			app.AssertDatabaseCount("images", dirs*perDir, testkit.Row{"folder_id": folder.ID, "status": "pending"})
			app.AssertDatabaseCount("dirs", dirs+1, testkit.Row{"folder_id": folder.ID})
			app.AssertDatabaseCount("dirs", 0, testkit.Row{"folder_id": folder.ID, "mtime_ns": 0})
		})
	}
}

func TestScans_DeletingAFolderDeletesItsIndex(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User()
	tr := newTree(t, app)
	tr.write("a.png", "png")
	tr.scan(scans.KindFull)
	app.Create.Scan(tr.folder, factory.WithScanStatus(scans.StatusSucceeded))

	app.As(user).DELETE("/api/v1/folders/%d", tr.folder.ID).AssertOK()

	for _, table := range []string{"scans", "dirs", "images"} {
		app.AssertDatabaseCount(table, 0)
	}
	assert.FileExists(t, filepath.Join(tr.root, "a.png"))
}
