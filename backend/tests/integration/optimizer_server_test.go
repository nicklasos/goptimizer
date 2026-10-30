package integration

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"app/internal/db"
	"app/internal/images"
	"app/tests/testkit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The service runs as root on a server, and the projects' files belong to the web server's user,
// often with mode 0640. The swap gives the result that owner, group and mode back; the chown comes
// first because it clears setuid and setgid bits.
func TestOptimizer_KeepsTheOwnerAndModeOfAnotherUsersFile(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("only root can give a file to another user")
	}
	const wwwData = 33
	for _, mode := range []fs.FileMode{0o640, 0o750 | fs.ModeSetuid | fs.ModeSetgid} {
		t.Run(mode.String(), func(t *testing.T) {
			app := testkit.New(t)
			picture := photo(64, 48, 1)
			useFakeTools(t, app, map[string]string{"oxipng": copyOver(t, encodePNGBest(t, picture))})
			tr := optTree(t, app, lossless())
			tr.put("uploads/a.png", encodePNG(t, picture))
			require.NoError(t, os.Chown(tr.path("uploads"), wwwData, wwwData))
			require.NoError(t, os.Chown(tr.path("uploads/a.png"), wwwData, wwwData))
			require.NoError(t, os.Chmod(tr.path("uploads/a.png"), mode))
			before := stateOf(t, tr.path("uploads/a.png"))
			require.Equal(t, uint32(wwwData), before.uid)
			require.Equal(t, mode, before.mode)

			assert.Equal(t, 1, tr.optimize())

			assertSwapped(t, tr.path("uploads/a.png"), before)
			app.AssertDatabaseHas("images", tr.imageWith("uploads", "a.png", images.StatusOptimized))
			assertNoTempFiles(t, tr.root)
		})
	}
}

// A file with a POSIX ACL is skipped: the swap could not give the ACL to the new file, and the ACL
// may be what lets the web server read it. macOS has no POSIX ACLs, so this runs on Linux.
func TestOptimizer_SkipsAFileWithAnACL(t *testing.T) {
	setfacl, err := exec.LookPath("setfacl")
	if err != nil {
		t.Skip("setfacl is not installed (apt install acl)")
	}
	app := testkit.New(t)
	picture := photo(64, 48, 2)
	useFakeTools(t, app, map[string]string{"oxipng": copyOver(t, encodePNGBest(t, picture))})
	tr := optTree(t, app, lossless())
	tr.put("a.png", encodePNG(t, picture))
	if out, err := exec.Command(setfacl, "-m", "u:65534:r", tr.path("a.png")).CombinedOutput(); err != nil {
		t.Skipf("the temp dir's filesystem takes no ACL: %s", out)
	}
	before := stateOf(t, tr.path("a.png"))

	tr.optimize()

	assertUntouched(t, tr.path("a.png"), before)
	assertNoTempFiles(t, tr.root)
	app.AssertDatabaseHas("images", tr.imageWith("", "a.png", images.StatusSkipped))
	assert.Equal(t, "access_control_list", reasonOfImage(t, app, tr, "a.png"))
}

// With the nice setting on, every tool runs 10 steps nicer than the service, and in the idle I/O
// class where ionice exists (Linux); with it off, at the service's own priority.
func TestOptimizer_ToolsRunAtLowPriorityWhenTheSettingSaysSo(t *testing.T) {
	base := niceness(t)
	if base > 9 {
		t.Skipf("the tests already run at niceness %d, where 10 more cannot show", base)
	}
	_, ionice := exec.LookPath("ionice")
	for _, low := range []bool{true, false} {
		t.Run("nice "+strconv.FormatBool(low), func(t *testing.T) {
			app := testkit.New(t)
			_, err := app.Queries.UpdateSettings(app.Ctx, db.UpdateSettingsParams{Workers: 1, ScanWorkers: 1, Nice: low})
			require.NoError(t, err)
			seen := t.TempDir()
			useFakeTools(t, app, map[string]string{"oxipng": `ps -o nice= -p $$ > "` + seen + `/nice"; ionice > "` + seen + `/ionice" 2>&1 || true`})
			tr := optTree(t, app, lossless())
			tr.put("a.png", encodePNG(t, photo(32, 32, 3)))

			tr.optimize()

			want := base
			if low {
				want = base + 10
			}
			assert.Equal(t, strconv.Itoa(want), readTrimmed(t, filepath.Join(seen, "nice")), "the tool's niceness")
			if ionice == nil {
				class := readTrimmed(t, filepath.Join(seen, "ionice"))
				assert.Equal(t, low, strings.HasPrefix(class, "idle"), "the tool's I/O class: %s", class)
			}
			app.AssertDatabaseHas("images", tr.imageWith("", "a.png", images.StatusKept))
		})
	}
}

// niceness is the niceness of a program this test process starts.
func niceness(t *testing.T) int {
	t.Helper()
	out, err := exec.Command("sh", "-c", "ps -o nice= -p $$").Output()
	require.NoError(t, err)
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	require.NoError(t, err)
	return n
}

func readTrimmed(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return strings.TrimSpace(string(data))
}
