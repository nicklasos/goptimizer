package integration

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"app/internal/factory"
	"app/internal/folders"
	"app/internal/scans"
	"app/internal/tools"
	"app/tests/testkit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// uploadedAt is the mtime the tests give their image files: well past min_age_minutes, with
// nanoseconds, so a swap that rounded the time would show.
var uploadedAt = time.Date(2025, 3, 14, 15, 9, 26, 535897932, time.UTC)

// optTree is a folder on disk that the optimizer works on, with the given profile.
func optTree(t *testing.T, app *testkit.App, profile folders.Profile) *tree {
	t.Helper()
	return newTree(t, app, factory.WithProfile(profile))
}

func lossy() folders.Profile { return folders.DefaultProfile() }

func lossless() folders.Profile {
	p := folders.DefaultProfile()
	p.Mode = folders.ModeLossless
	return p
}

// put writes an image file as an upload would leave it: mode 0640, old mtime and atime.
func (tr *tree) put(rel string, data []byte) {
	tr.t.Helper()
	path := tr.path(rel)
	require.NoError(tr.t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(tr.t, os.WriteFile(path, data, 0o640))
	require.NoError(tr.t, os.Chmod(path, 0o640))
	require.NoError(tr.t, os.Chtimes(path, uploadedAt.Add(time.Hour), uploadedAt))
}

// optimize indexes the folder and optimizes everything claimable, as the scheduler would.
func (tr *tree) optimize() int {
	tr.t.Helper()
	tr.scan(scans.KindFull)
	return tr.app.Pool().Drain(tr.app.Ctx)
}

// fileState is everything a swap must keep, and the bytes.
type fileState struct {
	data       []byte
	mode       fs.FileMode
	uid, gid   uint32
	mtime      time.Time
	atime      time.Time
	inode      uint64
	isSymlink  bool
	linkTarget string
}

func stateOf(t *testing.T, path string) fileState {
	t.Helper()
	info, err := os.Lstat(path)
	require.NoError(t, err)
	var st unix.Stat_t
	require.NoError(t, unix.Lstat(path, &st))
	s := fileState{
		mode: info.Mode(), uid: st.Uid, gid: st.Gid, mtime: info.ModTime(),
		atime: time.Unix(st.Atim.Unix()), inode: uint64(st.Ino),
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		s.isSymlink = true
		s.linkTarget, err = os.Readlink(path)
		require.NoError(t, err)
		return s
	}
	s.data, err = os.ReadFile(path)
	require.NoError(t, err)
	return s
}

// assertUntouched: the same bytes, owner, mode and times, and the same inode, so not even a
// rename of identical bytes happened.
func assertUntouched(t *testing.T, path string, before fileState) {
	t.Helper()
	after := stateOf(t, path)
	assert.True(t, bytes.Equal(before.data, after.data), "the file's bytes are unchanged")
	assert.Equal(t, before.mode, after.mode, "mode")
	assert.Equal(t, before.uid, after.uid, "owner")
	assert.Equal(t, before.gid, after.gid, "group")
	assert.True(t, before.mtime.Equal(after.mtime), "mtime %v, was %v", after.mtime, before.mtime)
	assert.Equal(t, before.inode, after.inode, "inode")
	assert.Equal(t, before.isSymlink, after.isSymlink, "symlink")
	assert.Equal(t, before.linkTarget, after.linkTarget, "symlink target")
}

// assertSwapped: new, smaller bytes in a new inode, with the original's owner, mode and times.
func assertSwapped(t *testing.T, path string, before fileState) fileState {
	t.Helper()
	after := stateOf(t, path)
	assert.Less(t, len(after.data), len(before.data), "the file is smaller")
	assert.NotEqual(t, before.inode, after.inode, "the file was replaced by a rename")
	assert.Equal(t, before.mode, after.mode, "mode")
	assert.Equal(t, before.uid, after.uid, "owner")
	assert.Equal(t, before.gid, after.gid, "group")
	assert.True(t, before.mtime.Equal(after.mtime), "mtime %v, was %v", after.mtime, before.mtime)
	assert.True(t, before.atime.Equal(after.atime), "atime %v, was %v", after.atime, before.atime)
	return after
}

func assertNoTempFiles(t *testing.T, root string) {
	t.Helper()
	require.NoError(t, filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		require.NoError(t, err)
		assert.False(t, strings.HasPrefix(d.Name(), ".goptimizer-"), "temp file left: %s", path)
		return nil
	}))
}

func assertWorkDirEmpty(t *testing.T, app *testkit.App) {
	t.Helper()
	entries, err := os.ReadDir(app.Config.WorkDir)
	require.NoError(t, err)
	assert.Empty(t, entries, "the work dir is empty")
}

// useRealTools points app.Tools at the programs on PATH, and skips the test when one is missing.
func useRealTools(t *testing.T, app *testkit.App, names ...string) {
	t.Helper()
	for _, name := range names {
		path, err := exec.LookPath(name)
		if err != nil {
			t.Skipf("%s is not installed (brew install %s, apt install %s)", name, name, name)
		}
		app.Tools.Install(name, path, "real")
	}
	app.ToolReport().Refresh(app.Ctx)
}

// useFakeTools installs a shell script for every image program: the given body, or one that does
// nothing. A script gets the program's arguments; the work file is always the last one, $f.
func useFakeTools(t *testing.T, app *testkit.App, bodies map[string]string) {
	t.Helper()
	dir := t.TempDir()
	for _, program := range tools.Programs {
		body, ok := bodies[program.Name]
		if !ok {
			body = "exit 0"
		}
		path := filepath.Join(dir, program.Name)
		script := "#!/bin/sh\nfor f; do :; done\n" + body + "\n"
		require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
		app.Tools.Install(program.Name, path, "fake")
	}
	app.ToolReport().Refresh(app.Ctx)
}

// copyOver is a fake tool body that replaces the work file with the bytes given.
func copyOver(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "output")
	require.NoError(t, os.WriteFile(path, data, 0o644))
	return `cp "` + path + `" "$f"`
}

// photo is a deterministic picture with smooth gradients and fine noise, the kind of content the
// lossy tools shrink a lot without visible damage.
func photo(w, h int, seed uint32) *image.NRGBA {
	m := image.NewNRGBA(image.Rect(0, 0, w, h))
	state := seed | 1
	for y := range h {
		for x := range w {
			state ^= state << 13
			state ^= state >> 17
			state ^= state << 5
			noise := float64(state%9) - 4
			fx, fy := float64(x)/float64(w), float64(y)/float64(h)
			r := 120 + 90*math.Sin(fx*5+fy*2) + noise
			g := 110 + 80*math.Cos(fy*4-fx) + noise
			b := 100 + 70*math.Sin(fx*3*fy+1) + noise
			m.SetNRGBA(x, y, color.NRGBA{clamp(r), clamp(g), clamp(b), 255})
		}
	}
	return m
}

func clamp(v float64) uint8 { return uint8(max(0, min(255, math.Round(v)))) }

// flat is a picture of a few flat colors, as UI art is: quantizing it to a palette loses nothing.
func flat(w, h int) *image.NRGBA {
	m := image.NewNRGBA(image.Rect(0, 0, w, h))
	colors := []color.NRGBA{{240, 240, 245, 255}, {30, 120, 220, 255}, {250, 250, 250, 255}, {220, 60, 60, 255}, {40, 40, 40, 255}}
	for y := range h {
		for x := range w {
			m.SetNRGBA(x, y, colors[(x/20+y/15)%len(colors)])
		}
	}
	return m
}

func encodePNG(t *testing.T, m image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, (&png.Encoder{CompressionLevel: png.NoCompression}).Encode(&buf, m))
	return buf.Bytes()
}

func encodeJPEG(t *testing.T, m image.Image, quality int) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, m, &jpeg.Options{Quality: quality}))
	return buf.Bytes()
}

// animation is a GIF of n frames of a moving square, each shown for delay hundredths of a second.
func animation(t *testing.T, n, delay, loop int) []byte {
	t.Helper()
	palette := []color.Color{color.RGBA{0, 0, 0, 255}, color.RGBA{255, 255, 255, 255}, color.RGBA{200, 30, 30, 255}, color.RGBA{30, 30, 200, 255}}
	g := &gif.GIF{LoopCount: loop}
	for i := range n {
		frame := image.NewPaletted(image.Rect(0, 0, 64, 48), palette)
		draw.Draw(frame, frame.Rect, &image.Uniform{palette[1]}, image.Point{}, draw.Src)
		draw.Draw(frame, image.Rect(4*i, 10, 4*i+16, 26), &image.Uniform{palette[2+i%2]}, image.Point{}, draw.Src)
		g.Image = append(g.Image, frame)
		g.Delay = append(g.Delay, delay)
	}
	var buf bytes.Buffer
	require.NoError(t, gif.EncodeAll(&buf, g))
	return buf.Bytes()
}

func encodePNGBest(t *testing.T, m image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&buf, m))
	return buf.Bytes()
}

// smooth is a wide gradient, where too few levels show as bands.
func smooth(w, h int) *image.NRGBA {
	m := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			m.SetNRGBA(x, y, color.NRGBA{uint8(x * 255 / w), uint8(80 + y/2), 160, 255})
		}
	}
	return m
}

// withJPEGSegments puts marker segments right after SOI.
func withJPEGSegments(data []byte, segments ...[]byte) []byte {
	out := append([]byte{}, data[:2]...)
	for _, s := range segments {
		out = append(out, s...)
	}
	return append(out, data[2:]...)
}

func jpegSegment(marker byte, body []byte) []byte {
	return append([]byte{0xFF, marker, byte((len(body) + 2) >> 8), byte(len(body) + 2)}, body...)
}

// exifSegment is an APP1 EXIF block with only an orientation tag.
func exifSegment(orientation byte) []byte {
	tiff := []byte("MM\x00\x2a\x00\x00\x00\x08\x00\x01\x01\x12\x00\x03\x00\x00\x00\x01\x00")
	tiff = append(tiff, orientation, 0, 0, 0, 0, 0, 0)
	return jpegSegment(0xE1, append([]byte("Exif\x00\x00"), tiff...))
}

func iccSegment(profile string) []byte {
	return jpegSegment(0xE2, append([]byte("ICC_PROFILE\x00\x01\x01"), profile...))
}

// withPNGChunk puts a chunk right after IHDR; an empty kind adds nothing.
func withPNGChunk(data []byte, kind string, body []byte) []byte {
	out := append([]byte{}, data[:33]...)
	if kind != "" {
		c := binary.BigEndian.AppendUint32(nil, uint32(len(body)))
		c = append(c, kind...)
		c = append(c, body...)
		out = append(out, binary.BigEndian.AppendUint32(c, crc32.ChecksumIEEE(c[4:]))...)
	}
	return append(out, data[33:]...)
}

// fixIHDRCRC recomputes the CRC of an IHDR edited in place.
func fixIHDRCRC(data []byte) []byte {
	binary.BigEndian.PutUint32(data[29:], crc32.ChecksumIEEE(data[12:29]))
	return data
}

func osChmod(path string, mode os.FileMode) error { return os.Chmod(path, mode) }
