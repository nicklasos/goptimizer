package plans

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"app/internal/runlog"
)

// ExtractLayout is where the entries of a files archive go. Their names are the absolute paths
// that were archived, without the leading slash. An entry lands in one region: a path it must be
// at or under, and the directory opened as an os.Root to write it through, so that neither the
// name nor a symbolic link on the way can lead out.
type ExtractLayout struct {
	// base is what a name is joined to: the target directory, or / for the original locations.
	base    string
	regions []extractRegion
}

type extractRegion struct {
	allow string
	root  string
}

// IntoDirectory extracts every entry under dir, at dir/<archived path>.
func IntoDirectory(dir string) ExtractLayout {
	dir = filepath.Clean(dir)
	return ExtractLayout{base: dir, regions: []extractRegion{{allow: dir, root: dir}}}
}

// ToOriginalLocations extracts every entry back to the path it was archived from, which must be
// one of the plan's paths or under one.
func ToOriginalLocations(paths []string) ExtractLayout {
	layout := ExtractLayout{base: string(filepath.Separator)}
	for _, path := range paths {
		path = filepath.Clean(path)
		layout.regions = append(layout.regions, extractRegion{allow: path, root: filepath.Dir(path)})
	}
	return layout
}

var errUnsafeName = errors.New("climbs out of its directory or is absolute")

// Place is where an entry goes: the region (an index into the layout's regions) and the path
// inside its root. A name that is absolute, has a ".." element, or falls outside every region
// is refused.
func (l ExtractLayout) Place(name string) (region int, rel string, err error) {
	path, err := l.path(name)
	if err != nil {
		return 0, "", err
	}
	region = -1
	for i, r := range l.regions {
		if within(path, r.allow) && (region < 0 || len(r.allow) > len(l.regions[region].allow)) {
			region = i
		}
	}
	if region < 0 {
		return 0, "", errors.New("is outside the plan's paths")
	}
	rel, err = filepath.Rel(l.regions[region].root, path)
	return region, rel, err
}

// LinkAllowed says whether a symbolic link entry may be created: its target, followed from where
// the link goes, must stay in the link's region.
func (l ExtractLayout) LinkAllowed(name, target string) bool {
	region, _, err := l.Place(name)
	if err != nil || target == "" {
		return false
	}
	path, _ := l.path(name)
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	return within(filepath.Clean(target), l.regions[region].allow)
}

func (l ExtractLayout) path(name string) (string, error) {
	trimmed := strings.TrimSuffix(name, "/")
	if trimmed == "" || strings.HasPrefix(name, "/") || strings.Contains(name, "\\") {
		return "", errUnsafeName
	}
	for _, part := range strings.Split(trimmed, "/") {
		if part == ".." {
			return "", errUnsafeName
		}
	}
	return filepath.Join(l.base, filepath.FromSlash(trimmed)), nil
}

func within(path, dir string) bool {
	return dir == string(filepath.Separator) || path == dir || strings.HasPrefix(path, dir+string(filepath.Separator))
}

// restoreFiles downloads the archive and checks it before it writes anything, then extracts it.
func restoreFiles(ctx context.Context, job RestoreJob, log *runlog.Log) error {
	var target FilesTarget
	var source FilesConfig
	if err := json.Unmarshal(job.Target, &target); err != nil {
		return err
	}
	if err := json.Unmarshal(job.Source, &source); err != nil {
		return err
	}
	layout := IntoDirectory(target.Directory)
	if target.OriginalLocations {
		layout = ToOriginalLocations(source.Paths)
	}

	path, err := job.Archive.download(ctx, job.WorkDir, log)
	if err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("the archive is not gzip: %w", err)
	}
	if target.OriginalLocations {
		log.Printf("Extracting to the original locations: %s", strings.Join(source.Paths, ", "))
	} else {
		log.Printf("Extracting under %s", target.Directory)
	}
	x := &extractor{ctx: ctx, layout: layout, roots: map[int]*os.Root{}, log: log}
	defer x.close()
	if err := x.extract(tar.NewReader(gz)); err != nil {
		return err
	}
	log.Printf("Extracted %d files, %d directories, %d links; skipped %d links leading out, %d special files",
		x.files, x.dirs, x.links, x.skippedLinks, x.special)
	return nil
}

type extractor struct {
	ctx    context.Context
	layout ExtractLayout
	roots  map[int]*os.Root
	log    *runlog.Log
	// pending are the directories, which get their mode and times after everything in them is written.
	pending []pendingDir

	files, dirs, links, skippedLinks, special int
}

type pendingDir struct {
	root  *os.Root
	rel   string
	mode  fs.FileMode
	mtime time.Time
}

func (x *extractor) extract(tr *tar.Reader) error {
	for {
		if err := x.ctx.Err(); err != nil {
			return err
		}
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("the archive is damaged: %w", err)
		}
		if err := x.entry(hdr, tr); err != nil {
			return fmt.Errorf("%s: %w", hdr.Name, err)
		}
	}
	for i := len(x.pending) - 1; i >= 0; i-- {
		d := x.pending[i]
		if err := d.root.Chmod(d.rel, d.mode); err != nil {
			return err
		}
		if err := d.root.Chtimes(d.rel, d.mtime, d.mtime); err != nil {
			return err
		}
	}
	return nil
}

func (x *extractor) entry(hdr *tar.Header, r io.Reader) error {
	switch hdr.Typeflag {
	case tar.TypeChar, tar.TypeBlock, tar.TypeFifo:
		x.special++
		return nil
	case tar.TypeDir, tar.TypeReg, tar.TypeSymlink:
	default:
		x.log.Warnf("%s: skipped, an entry of type %q", hdr.Name, hdr.Typeflag)
		x.special++
		return nil
	}

	region, rel, err := x.layout.Place(hdr.Name)
	if err != nil {
		return err
	}
	root, err := x.root(region)
	if err != nil {
		return err
	}
	mode := fs.FileMode(hdr.Mode).Perm()
	switch hdr.Typeflag {
	case tar.TypeDir:
		if err := root.MkdirAll(rel, 0o700); err != nil {
			return err
		}
		x.pending = append(x.pending, pendingDir{root: root, rel: rel, mode: mode, mtime: hdr.ModTime})
		x.dirs++
	case tar.TypeSymlink:
		if !x.layout.LinkAllowed(hdr.Name, hdr.Linkname) {
			x.log.Warnf("%s: skipped, a link to %s, which is outside the restore", hdr.Name, hdr.Linkname)
			x.skippedLinks++
			return nil
		}
		if err := replaceable(root, rel); err != nil {
			return err
		}
		if err := root.Symlink(hdr.Linkname, rel); err != nil {
			return err
		}
		x.links++
	case tar.TypeReg:
		if err := replaceable(root, rel); err != nil {
			return err
		}
		if err := writeEntry(x.ctx, root, rel, r); err != nil {
			return err
		}
		if err := root.Chmod(rel, mode); err != nil {
			return err
		}
		if err := root.Chtimes(rel, hdr.ModTime, hdr.ModTime); err != nil {
			return err
		}
		x.files++
	}
	return nil
}

// root opens a region's root on first use, creating its directory.
func (x *extractor) root(region int) (*os.Root, error) {
	if root, ok := x.roots[region]; ok {
		return root, nil
	}
	dir := x.layout.regions[region].root
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	x.roots[region] = root
	return root, nil
}

func (x *extractor) close() {
	for _, root := range x.roots {
		root.Close()
	}
}

// replaceable makes way for a file or link at rel: its directory exists, and what is there
// already, unless a directory, is removed, so a link in the way is replaced, never written through.
func replaceable(root *os.Root, rel string) error {
	if parent := filepath.Dir(rel); parent != "." {
		if err := root.MkdirAll(parent, 0o755); err != nil {
			return err
		}
	}
	info, err := root.Lstat(rel)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case err != nil:
		return err
	case info.IsDir():
		return errors.New("a directory is in the way")
	}
	return root.Remove(rel)
}

func writeEntry(ctx context.Context, root *os.Root, rel string, r io.Reader) error {
	f, err := root.OpenFile(rel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, contextReader{ctx: ctx, r: r})
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}
