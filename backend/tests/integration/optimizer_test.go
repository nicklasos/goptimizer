package integration

import (
	"bytes"
	"crypto/sha256"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"app/internal/folders"
	"app/internal/images"
	"app/internal/scans"
	"app/tests/testkit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOptimizer_RealToolsShrinkEachFormat(t *testing.T) {
	cases := []struct {
		name    string
		file    string
		profile folders.Profile
		tools   []string
		content func(t *testing.T) []byte
	}{
		{"png lossy", "a.png", lossy(), []string{"pngquant", "oxipng"}, func(t *testing.T) []byte { return encodePNG(t, flat(160, 120)) }},
		{"png lossless", "a.png", lossless(), []string{"oxipng"}, func(t *testing.T) []byte { return encodePNG(t, photo(160, 120, 2)) }},
		{"png lossless with optipng", "a.png", lossless(), []string{"optipng"}, func(t *testing.T) []byte { return encodePNG(t, photo(160, 120, 3)) }},
		{"jpeg lossy", "a.jpg", lossy(), []string{"jpegoptim"}, func(t *testing.T) []byte { return encodeJPEG(t, photo(320, 240, 4), 95) }},
		{"jpeg lossless", "a.jpeg", lossless(), []string{"jpegoptim"}, func(t *testing.T) []byte { return encodeJPEG(t, photo(320, 240, 5), 90) }},
		{"gif lossy", "a.gif", lossy(), []string{"gifsicle"}, func(t *testing.T) []byte { return animation(t, 8, 7, 0) }},
		{"gif lossless", "a.gif", lossless(), []string{"gifsicle"}, func(t *testing.T) []byte { return animation(t, 8, 7, 0) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := testkit.New(t)
			useFakeTools(t, app, nil)
			app.Tools.Remove("oxipng")
			useRealTools(t, app, tc.tools...)
			tr := optTree(t, app, tc.profile)
			tr.put("sub/"+tc.file, tc.content(t))
			before := stateOf(t, tr.path("sub/"+tc.file))

			assert.Equal(t, 1, tr.optimize())

			after := assertSwapped(t, tr.path("sub/"+tc.file), before)
			assertDecodesLike(t, before.data, after.data)
			sum := sha256.Sum256(after.data)
			row := tr.imageWith("sub", tc.file, images.StatusOptimized)
			row["size"], row["original_size"], row["optimized_size"] = len(after.data), len(before.data), len(after.data)
			row["mtime_ns"], row["mode"], row["output_sha256"], row["attempts"] = uploadedAt.UnixNano(), string(tc.profile.Mode), sum[:], 0
			app.AssertDatabaseHas("images", row)
			app.AssertDatabaseHas("daily_stats", testkit.Row{
				"folder_id": tr.folder.ID, "optimized": 1, "bytes_before": len(before.data), "bytes_after": len(after.data),
			})
			assertNoTempFiles(t, tr.root)
			assertWorkDirEmpty(t, app)

			assert.Zero(t, tr.scan(scans.KindFull).FilesChanged, "the next scan sees the row as the file is")
		})
	}
}

// assertDecodesLike: the result decodes in the original's format with its dimensions.
func assertDecodesLike(t *testing.T, original, result []byte) {
	t.Helper()
	a, formatA, err := image.DecodeConfig(bytes.NewReader(original))
	require.NoError(t, err)
	b, formatB, err := image.DecodeConfig(bytes.NewReader(result))
	require.NoError(t, err)
	assert.Equal(t, formatA, formatB)
	assert.Equal(t, a.Width, b.Width)
	assert.Equal(t, a.Height, b.Height)
	_, _, err = image.Decode(bytes.NewReader(result))
	require.NoError(t, err)
}

// Every refusal leaves the original exactly as it was, and nothing behind: no temp file next to
// it, nothing in the work dir.
func TestOptimizer_RefusedResultsLeaveTheOriginal(t *testing.T) {
	pngFile := func(t *testing.T) []byte { return encodePNG(t, photo(96, 64, 7)) }
	jpegFile := func(t *testing.T) []byte {
		return withJPEGSegments(encodeJPEG(t, photo(96, 64, 8), 95), exifSegment(6), iccSegment("display profile"))
	}
	gifFile := func(t *testing.T) []byte { return animation(t, 6, 8, 0) }

	cases := []struct {
		name    string
		file    string
		profile folders.Profile
		content func(t *testing.T) []byte
		tools   func(t *testing.T, original []byte) map[string]string
		status  images.Status
		reason  string
	}{
		{"larger output", "a.png", lossless(), pngFile, func(t *testing.T, o []byte) map[string]string {
			return map[string]string{"oxipng": `printf 'padding padding' >> "$f"`}
		}, images.StatusKept, "no_saving"},
		{"another format", "a.png", lossless(), pngFile, func(t *testing.T, o []byte) map[string]string {
			return map[string]string{"oxipng": copyOver(t, animation(t, 1, 0, 0))}
		}, images.StatusKept, "format_changed"},
		{"other dimensions", "a.png", lossless(), pngFile, func(t *testing.T, o []byte) map[string]string {
			return map[string]string{"oxipng": copyOver(t, encodePNGBest(t, photo(96, 63, 7)))}
		}, images.StatusKept, "dimensions_changed"},
		{"fewer GIF frames", "a.gif", lossless(), gifFile, func(t *testing.T, o []byte) map[string]string {
			return map[string]string{"gifsicle": copyOver(t, animation(t, 3, 8, 0))}
		}, images.StatusKept, "frames_changed"},
		{"one changed pixel in lossless", "a.png", lossless(), pngFile, func(t *testing.T, o []byte) map[string]string {
			m := photo(96, 64, 7)
			m.Pix[4*(10*96+10)] ^= 1
			return map[string]string{"oxipng": copyOver(t, encodePNGBest(t, m))}
		}, images.StatusKept, "pixels_changed"},
		{"heavy banding in lossy", "a.png", lossy(), func(t *testing.T) []byte { return encodePNG(t, smooth(256, 128)) }, func(t *testing.T, o []byte) map[string]string {
			m := smooth(256, 128)
			for i := range m.Pix {
				if i%4 != 3 {
					m.Pix[i] &^= 31
				}
			}
			return map[string]string{"pngquant": copyOver(t, encodePNGBest(t, m))}
		}, images.StatusKept, "ssim_below_threshold"},
		{"dropped ICC profile", "a.jpg", lossy(), jpegFile, func(t *testing.T, o []byte) map[string]string {
			return map[string]string{"jpegoptim": copyOver(t, withJPEGSegments(encodeJPEG(t, photo(96, 64, 8), 80), exifSegment(6)))}
		}, images.StatusKept, "metadata_changed"},
		{"dropped orientation", "a.jpg", lossy(), jpegFile, func(t *testing.T, o []byte) map[string]string {
			return map[string]string{"jpegoptim": copyOver(t, withJPEGSegments(encodeJPEG(t, photo(96, 64, 8), 80), iccSegment("display profile")))}
		}, images.StatusKept, "metadata_changed"},
		{"tool exits 1", "a.png", lossless(), pngFile, func(t *testing.T, o []byte) map[string]string {
			return map[string]string{"oxipng": `echo "oxipng: out of memory" >&2; exit 1`}
		}, images.StatusFailed, "tool_failed"},
		{"tool writes garbage to its input", "a.png", lossless(), pngFile, func(t *testing.T, o []byte) map[string]string {
			return map[string]string{"oxipng": `for path; do printf 'garbage' > "$path" 2>/dev/null; done`}
		}, images.StatusKept, "format_changed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := testkit.New(t)
			tr := optTree(t, app, tc.profile)
			tr.put(tc.file, tc.content(t))
			before := stateOf(t, tr.path(tc.file))
			useFakeTools(t, app, tc.tools(t, before.data))

			assert.Equal(t, 1, tr.optimize())

			assertUntouched(t, tr.path(tc.file), before)
			assertNoTempFiles(t, tr.root)
			assertWorkDirEmpty(t, app)
			row := tr.imageWith("", tc.file, tc.status)
			app.AssertDatabaseHas("images", row)
			assert.True(t, strings.HasPrefix(reasonOf(t, app, tr), tc.reason), "reason %q", reasonOf(t, app, tr))
			app.AssertDatabaseCount("daily_stats", 0)
		})
	}
}

func reasonOf(t *testing.T, app *testkit.App, tr *tree) string {
	t.Helper()
	var reason string
	require.NoError(t, app.DB.QueryRow(`SELECT COALESCE(reason, '') FROM images WHERE folder_id = ?`, tr.folder.ID).Scan(&reason))
	return reason
}

func TestOptimizer_AToolThatHangsIsKilledAtTheTimeout(t *testing.T) {
	app := testkit.New(t)
	tr := optTree(t, app, lossless())
	tr.put("a.png", encodePNG(t, photo(64, 64, 9)))
	before := stateOf(t, tr.path("a.png"))
	pidFile := filepath.Join(t.TempDir(), "pid")
	useFakeTools(t, app, map[string]string{"oxipng": `sleep 60 & echo $! > "` + pidFile + `"; wait`})
	app.Pool().Optimizer.ToolTimeout = 2 * time.Second

	started := time.Now()
	assert.Equal(t, 1, tr.optimize())

	assert.Less(t, time.Since(started), 10*time.Second, "the timeout stopped the tool")
	assertUntouched(t, tr.path("a.png"), before)
	assertNoTempFiles(t, tr.root)
	assertWorkDirEmpty(t, app)
	row := tr.imageWith("", "a.png", images.StatusFailed)
	row["attempts"] = 1
	app.AssertDatabaseHas("images", row)
	assert.Contains(t, reasonOf(t, app, tr), "oxipng ran longer than 2s")
	pid, err := os.ReadFile(pidFile)
	require.NoError(t, err)
	assert.Error(t, exec.Command("kill", "-0", strings.TrimSpace(string(pid))).Run(), "the process the tool started was killed with it")
}

// Files the optimizer must not touch, whatever the tools would make of them. The tools here would
// shrink anything, so only the guards stand between them and the file.
func TestOptimizer_FilesItMustNotTouch(t *testing.T) {
	shrink := map[string]string{"oxipng": `head -c 100 "$f" > "$f.cut" && mv "$f.cut" "$f"`}
	apng := withPNGChunk(encodePNG(t, photo(32, 32, 1)), "acTL", []byte{0, 0, 0, 2, 0, 0, 0, 0})
	bomb := withPNGChunk(encodePNG(t, photo(8, 8, 1)), "", nil)
	bomb[16], bomb[17], bomb[18], bomb[19] = 0, 0, 0x27, 0x10
	bomb[20], bomb[21], bomb[22], bomb[23] = 0, 0, 0x27, 0x10
	bomb = fixIHDRCRC(bomb)

	cases := []struct {
		name   string
		put    func(t *testing.T, tr *tree)
		after  func(t *testing.T, tr *tree)
		status images.Status
		reason string
	}{
		{"a symlink", func(t *testing.T, tr *tree) {
			tr.put("target.png", encodePNG(t, photo(32, 32, 2)))
			tr.put("a.png", encodePNG(t, photo(32, 32, 3)))
		}, func(t *testing.T, tr *tree) {
			require.NoError(t, os.Remove(tr.path("a.png")))
			require.NoError(t, os.Symlink(tr.path("target.png"), tr.path("a.png")))
		}, images.StatusSkipped, "symlink"},
		{"a hard-linked file", func(t *testing.T, tr *tree) {
			tr.put("a.png", encodePNG(t, photo(32, 32, 4)))
			require.NoError(t, os.Link(tr.path("a.png"), filepath.Join(t.TempDir(), "elsewhere.png")))
		}, nil, images.StatusSkipped, "hard_link"},
		{"an animated PNG", func(t *testing.T, tr *tree) { tr.put("a.png", apng) }, nil, images.StatusSkipped, "apng"},
		{"a JPEG named .png", func(t *testing.T, tr *tree) { tr.put("a.png", encodeJPEG(t, photo(32, 32, 5), 90)) }, nil, images.StatusSkipped, "wrong_content"},
		{"a file over the pixel limit", func(t *testing.T, tr *tree) { tr.put("a.png", bomb) }, nil, images.StatusSkipped, "too_many_pixels"},
		{"a file rewritten since the scan, younger than min_age_minutes", func(t *testing.T, tr *tree) {
			tr.put("a.png", encodePNG(t, photo(32, 32, 6)))
		}, func(t *testing.T, tr *tree) {
			require.NoError(t, os.WriteFile(tr.path("a.png"), encodePNG(t, photo(32, 32, 7)), 0o640))
		}, images.StatusPending, "too_new"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := testkit.New(t)
			useFakeTools(t, app, shrink)
			tr := optTree(t, app, lossless())
			tc.put(t, tr)
			tr.scan(scans.KindFull)
			if tc.after != nil {
				tc.after(t, tr)
			}
			before := stateOf(t, tr.path("a.png"))

			tr.app.Pool().Drain(app.Ctx)

			assertUntouched(t, tr.path("a.png"), before)
			assertNoTempFiles(t, tr.root)
			app.AssertDatabaseHas("images", tr.imageWith("", "a.png", tc.status))
			assert.Equal(t, tc.reason, reasonOfImage(t, app, tr, "a.png"))
		})
	}
}

func TestOptimizer_AFileYoungerThanMinAgeIsNotClaimed(t *testing.T) {
	app := testkit.New(t)
	useFakeTools(t, app, map[string]string{"oxipng": `head -c 100 "$f" > "$f.cut" && mv "$f.cut" "$f"`})
	tr := optTree(t, app, lossless())
	require.NoError(t, os.WriteFile(tr.path("new.png"), encodePNG(t, photo(32, 32, 1)), 0o644))
	before := stateOf(t, tr.path("new.png"))

	assert.Zero(t, tr.optimize())

	assertUntouched(t, tr.path("new.png"), before)
	app.AssertDatabaseHas("images", tr.imageWith("", "new.png", images.StatusPending))
}

// A fake tool that rewrites the original while it runs stands in for an upload that lands
// mid-optimization: the new upload stays, and the image is pending again for it.
func TestOptimizer_AFileChangedWhileTheToolRunsKeepsTheNewContent(t *testing.T) {
	app := testkit.New(t)
	tr := optTree(t, app, lossless())
	tr.put("a.png", encodePNG(t, photo(64, 48, 1)))
	smaller := encodePNGBest(t, photo(64, 48, 1))
	upload := encodePNG(t, photo(40, 40, 2))
	uploadPath := filepath.Join(t.TempDir(), "upload")
	require.NoError(t, os.WriteFile(uploadPath, upload, 0o644))
	useFakeTools(t, app, map[string]string{
		"oxipng": `cp "` + uploadPath + `" "` + tr.path("a.png") + `"; ` + copyOver(t, smaller),
	})

	assert.Equal(t, 1, tr.optimize())

	data, err := os.ReadFile(tr.path("a.png"))
	require.NoError(t, err)
	assert.Equal(t, upload, data, "the upload that came in during the run is on disk")
	row := tr.imageWith("", "a.png", images.StatusPending)
	row["size"], row["reason"] = len(upload), "changed_while_optimizing"
	app.AssertDatabaseHas("images", row)
	assertNoTempFiles(t, tr.root)
	app.AssertDatabaseCount("daily_stats", 0)
}

func reasonOfImage(t *testing.T, app *testkit.App, tr *tree, name string, dir ...string) string {
	t.Helper()
	var reason string
	d := ""
	if len(dir) > 0 {
		d = dir[0]
	}
	require.NoError(t, app.DB.QueryRow(`SELECT COALESCE(reason, '') FROM images WHERE folder_id = ? AND dir = ? AND name = ?`, tr.folder.ID, d, name).Scan(&reason))
	return reason
}

func TestOptimizer_NeverLossyTwice(t *testing.T) {
	app := testkit.New(t)
	useRealTools(t, app, "pngquant", "oxipng")
	tr := optTree(t, app, lossy())
	tr.put("a.png", encodePNG(t, flat(160, 120)))
	require.Equal(t, 1, tr.optimize())
	optimized := stateOf(t, tr.path("a.png"))
	app.AssertDatabaseHas("images", tr.imageWith("", "a.png", images.StatusOptimized))

	t.Run("touched, it is the optimizer's own output and stays as it is", func(t *testing.T) {
		touched := uploadedAt.Add(time.Minute)
		require.NoError(t, os.Chtimes(tr.path("a.png"), touched, touched))
		before := stateOf(t, tr.path("a.png"))

		assert.Equal(t, 1, tr.optimize())

		assertUntouched(t, tr.path("a.png"), before)
		row := tr.imageWith("", "a.png", images.StatusOptimized)
		row["mtime_ns"], row["size"] = touched.UnixNano(), len(optimized.data)
		app.AssertDatabaseHas("images", row)
		app.AssertDatabaseHas("daily_stats", testkit.Row{"folder_id": tr.folder.ID, "optimized": 1})
	})

	t.Run("copied elsewhere in the folder, it is still the optimizer's output", func(t *testing.T) {
		tr.put("copy/a.png", optimized.data)
		before := stateOf(t, tr.path("copy/a.png"))

		assert.Equal(t, 1, tr.optimize())

		assertUntouched(t, tr.path("copy/a.png"), before)
		app.AssertDatabaseHas("images", tr.imageWith("copy", "a.png", images.StatusKept))
		assert.Equal(t, "already_optimized", reasonOfImage(t, app, tr, "a.png", "copy"))
	})

	t.Run("a new upload at the same path is a new original", func(t *testing.T) {
		tr.put("a.png", encodePNG(t, flat(200, 100)))
		before := stateOf(t, tr.path("a.png"))

		assert.Equal(t, 1, tr.optimize())

		after := assertSwapped(t, tr.path("a.png"), before)
		row := tr.imageWith("", "a.png", images.StatusOptimized)
		row["original_size"], row["size"] = len(before.data), len(after.data)
		app.AssertDatabaseHas("images", row)
	})
}

// A scan that read a row before the optimizer swapped and recorded its file, and writes after,
// must not send the image back to pending. The scan's progress callback runs between its read
// and its flush, so the optimizer runs exactly there.
func TestOptimizer_AStaleScanWriteDoesNotUndoASwap(t *testing.T) {
	app := testkit.New(t)
	useRealTools(t, app, "oxipng")
	tr := optTree(t, app, lossless())
	tr.put("a.png", encodePNG(t, photo(64, 48, 1)))
	tr.scan(scans.KindFull)
	tr.put("a.png", encodePNG(t, photo(64, 40, 2)))
	tr.touch("")

	optimized := false
	_, err := tr.scanner.Scan(app.Ctx, tr.folder.ID, scans.KindFull, func(scans.Counts) {
		if !optimized {
			optimized = app.Pool().Drain(app.Ctx) == 1
		}
	})
	require.NoError(t, err)
	require.True(t, optimized, "the optimizer ran inside the scan, after the scan read the changed file")

	after := stateOf(t, tr.path("a.png"))
	row := tr.imageWith("", "a.png", images.StatusOptimized)
	row["size"] = len(after.data)
	app.AssertDatabaseHas("images", row)
	assert.Zero(t, tr.scan(scans.KindFull).FilesChanged, "the row and the file agree")
}

func TestOptimizer_TheScannerLeavesARowBeingOptimizedAlone(t *testing.T) {
	app := testkit.New(t)
	tr := optTree(t, app, lossless())
	tr.put("a.png", encodePNG(t, photo(16, 16, 1)))
	tr.put("gone/b.png", encodePNG(t, photo(16, 16, 2)))
	tr.scan(scans.KindFull)
	for range 2 {
		_, ok, err := app.Pool().Claim(app.Ctx)
		require.NoError(t, err)
		require.True(t, ok)
	}
	tr.put("a.png", encodePNG(t, photo(16, 16, 3)))
	tr.removeAll("gone")
	tr.touch("")

	counts := tr.scan(scans.KindFull)

	assert.Zero(t, counts.FilesChanged+counts.FilesGone, "neither change is the scanner's to record")
	app.AssertDatabaseHas("images", tr.imageWith("", "a.png", images.StatusOptimizing))
	app.AssertDatabaseHas("images", tr.imageWith("gone", "b.png", images.StatusOptimizing))
}

// A lossy result refused for its SSIM does not leave the image unoptimized when the lossless
// pipeline can still shrink it with every pixel kept.
func TestOptimizer_ARefusedLossyResultFallsBackToLossless(t *testing.T) {
	app := testkit.New(t)
	original := encodePNG(t, smooth(256, 128))
	banded := smooth(256, 128)
	for i := range banded.Pix {
		if i%4 != 3 {
			banded.Pix[i] &^= 31
		}
	}
	useFakeTools(t, app, map[string]string{"pngquant": copyOver(t, encodePNGBest(t, banded))})
	useRealTools(t, app, "oxipng")
	tr := optTree(t, app, lossy())
	tr.put("a.png", original)
	before := stateOf(t, tr.path("a.png"))

	assert.Equal(t, 1, tr.optimize())

	after := assertSwapped(t, tr.path("a.png"), before)
	a, err := png.Decode(bytes.NewReader(before.data))
	require.NoError(t, err)
	b, err := png.Decode(bytes.NewReader(after.data))
	require.NoError(t, err)
	assert.Equal(t, image.NewNRGBA(a.Bounds()).Bounds(), b.Bounds())
	for y := range 128 {
		for x := range 256 {
			require.Equal(t, color.NRGBAModel.Convert(a.At(x, y)), color.NRGBAModel.Convert(b.At(x, y)), "pixel %d,%d", x, y)
		}
	}
	row := tr.imageWith("", "a.png", images.StatusOptimized)
	row["mode"] = "lossless"
	app.AssertDatabaseHas("images", row)
}
