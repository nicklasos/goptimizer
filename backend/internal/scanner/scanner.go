// Package scanner lists a folder's directories into its image index, and runs the queued scans.
//
// Both kinds of scan list directories through the same step, list, which reconciles a directory's
// entries with its rows: new image files are inserted as pending, changed ones go back to pending,
// missing ones become gone, and a gone one that is back is pending again. They differ only in
// which directories they list. A full scan lists every directory from the root. A quick scan
// lstats the directories it knows and lists those whose mtime changed, and any new ones it finds.
package scanner

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"app/internal/db"
	"app/internal/folders"
	"app/internal/images"
	"app/internal/logger"
	"app/internal/scans"

	"github.com/bmatcuk/doublestar/v4"
)

const (
	// batchSize is how many writes go into one transaction, so the optimizer can start on a new
	// folder's first images while its first full scan is still walking.
	batchSize = 1000
	// settleTime: a directory changed this close to its listing may get another entry within the
	// same mtime tick, which would leave the mtime as stored. It is stored as 0, so it is listed again.
	settleTime = 2 * time.Second
	// tempMaxAge is when an optimizer temp file counts as left behind by a crash.
	tempMaxAge = time.Hour
	// folderCheckInterval is how often a scan looks whether its folder was paused or deleted.
	folderCheckInterval = time.Second
)

var (
	ErrFolderPaused  = errors.New("the folder was paused")
	ErrFolderDeleted = errors.New("the folder was deleted")
)

type Scanner struct {
	queries *db.Queries
	tx      *db.TxRunner
	logger  *logger.Logger
	// Now is the clock for stored times, the settle rule and temp file ages. Tests replace it.
	Now func() time.Time
}

func New(queries *db.Queries, tx *db.TxRunner, logger *logger.Logger) *Scanner {
	return &Scanner{queries: queries, tx: tx, logger: logger, Now: time.Now}
}

// Scan runs one scan of the folder and returns its counts. progress, when not nil, is called after
// each directory. A folder without a listed root is scanned in full whatever kind is asked for.
func (s *Scanner) Scan(ctx context.Context, folderID int64, kind scans.Kind, progress func(scans.Counts)) (scans.Counts, error) {
	r := &run{Scanner: s, folderID: folderID, progress: progress, removed: map[string]bool{}}
	if err := r.check(ctx); err != nil {
		return r.counts, err
	}
	var profile folders.Profile
	if err := json.Unmarshal([]byte(r.folder.Profile), &profile); err != nil {
		return r.counts, fmt.Errorf("the folder's profile cannot be read: %w", err)
	}
	r.root, r.excludes = r.folder.Path, profile.Excludes

	err := r.walk(ctx, kind)
	if err == nil {
		err = r.flush(ctx)
	}
	if r.otherDevices > 0 {
		s.logger.InfoContext(ctx, "Scan skipped directories on other filesystems", "folder", folderID, "directories", r.otherDevices)
	}
	return r.counts, err
}

// run is the state of one scan.
type run struct {
	*Scanner
	folderID int64
	folder   db.Folder
	root     string
	excludes []string
	device   uint64
	full     bool
	progress func(scans.Counts)

	// known is every dirs row: relative path to stored mtime_ns. children is the same by parent.
	known    map[string]int64
	children map[string][]string
	removed  map[string]bool
	queue    []string

	writes       []func(ctx context.Context, q *db.Queries) error
	counts       scans.Counts
	otherDevices int
	checkedAt    time.Time
}

func (r *run) walk(ctx context.Context, kind scans.Kind) error {
	rows, err := r.queries.ListDirs(ctx, r.folder.ID)
	if err != nil {
		return err
	}
	r.known = make(map[string]int64, len(rows))
	r.children = make(map[string][]string)
	for _, row := range rows {
		r.known[row.RelPath] = row.MtimeNs
		if row.RelPath != "" {
			parent := parentOf(row.RelPath)
			r.children[parent] = append(r.children[parent], row.RelPath)
		}
	}

	info, err := os.Lstat(r.root)
	if err == nil && !info.IsDir() {
		err = errors.New("not a directory")
	}
	if err != nil {
		return fmt.Errorf("the folder's path cannot be read: %w", err)
	}
	r.device = deviceOf(info)

	_, rootListed := r.known[""]
	r.full = kind == scans.KindFull || !rootListed
	if r.full {
		r.queue = []string{""}
	} else {
		// ListDirs is ordered by path, so a directory comes before everything inside it.
		r.queue = make([]string, len(rows))
		for i, row := range rows {
			r.queue[i] = row.RelPath
		}
	}

	for i := 0; i < len(r.queue); i++ {
		if err := r.check(ctx); err != nil {
			return err
		}
		if r.isRemoved(r.queue[i]) {
			continue
		}
		if err := r.visit(ctx, r.queue[i]); err != nil {
			return err
		}
		if r.progress != nil {
			r.progress(r.counts)
		}
	}
	return nil
}

// visit lists a directory, or in a quick scan only lstats it when it is known and unchanged.
func (r *run) visit(ctx context.Context, rel string) error {
	r.counts.DirsSeen++
	if rel != "" && r.excluded(rel) {
		r.remove(rel)
		return nil
	}
	abs := filepath.Join(r.root, rel)
	if stored, known := r.known[rel]; !r.full && known && stored != 0 {
		info, err := os.Lstat(abs)
		switch {
		case rel != "" && (missing(err) || (err == nil && !info.IsDir())):
			r.remove(rel)
			return nil
		case err != nil:
			return err
		case info.ModTime().UnixNano() == stored:
			return nil
		}
	}
	return r.list(ctx, rel, abs)
}

type file struct {
	format  images.Format
	size    int64
	mtimeNs int64
}

func (r *run) list(ctx context.Context, rel, abs string) error {
	dir, err := os.OpenFile(abs, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if rel != "" && missing(err) {
		r.remove(rel)
		return nil
	}
	if err != nil {
		return err
	}
	defer dir.Close()
	info, err := dir.Stat()
	if err != nil {
		return err
	}
	if deviceOf(info) != r.device {
		r.otherDevices++
		r.remove(rel)
		return nil
	}
	listedAt := r.Now()
	entries, err := dir.ReadDir(-1)
	if err != nil {
		return err
	}
	r.counts.DirsListed++
	rows, err := r.queries.ListImagesInDir(ctx, db.ListImagesInDirParams{FolderID: r.folder.ID, Dir: rel})
	if err != nil {
		return err
	}

	files := map[string]file{}
	subdirs := map[string]bool{}
	var found []string
	for _, entry := range entries {
		name := entry.Name()
		child := path.Join(rel, name)
		switch kind := entry.Type(); {
		case kind.IsDir():
			if r.excluded(child) {
				continue
			}
			subdirs[child] = true
			if _, known := r.known[child]; known {
				if r.full {
					r.queue = append(r.queue, child)
				}
				continue
			}
			info, err := entry.Info()
			if missing(err) {
				continue
			}
			if err != nil {
				return err
			}
			if deviceOf(info) != r.device {
				r.otherDevices++
				continue
			}
			found = append(found, child)
		case kind.IsRegular():
			if isTemp(name) {
				r.cleanTemp(ctx, entry, abs)
				continue
			}
			format, ok := images.FormatOf(name)
			if !ok || r.excluded(child) {
				continue
			}
			info, err := entry.Info()
			if missing(err) {
				continue
			}
			if err != nil {
				return err
			}
			files[name] = file{format: format, size: info.Size(), mtimeNs: info.ModTime().UnixNano()}
		}
	}
	r.counts.FilesSeen += int64(len(files))

	if err := r.reconcile(ctx, rel, files, rows); err != nil {
		return err
	}
	for _, child := range r.children[rel] {
		if !subdirs[child] {
			r.remove(child)
		}
	}
	// A new directory gets its row before this one stores its mtime, so a scan that stops
	// between the two still finds it: mtime 0 never matches, and the next quick scan lists it.
	for _, child := range found {
		if err := r.storeDir(ctx, child, 0); err != nil {
			return err
		}
		r.queue = append(r.queue, child)
	}
	mtime := info.ModTime().UnixNano()
	if !info.ModTime().Before(listedAt.Add(-settleTime)) {
		mtime = 0
	}
	if stored, known := r.known[rel]; !known || stored != mtime {
		return r.storeDir(ctx, rel, mtime)
	}
	return nil
}

// reconcile brings a directory's rows in line with its image files. It writes nothing for a file
// whose size and mtime match its row, so a scan that finds no change writes nothing. A row being
// optimized is the optimizer's until it is recorded: its file may be mid-swap. Each write applies
// only if the row is still as read here, so a flush after the optimizer recorded a swap is a no-op.
func (r *run) reconcile(ctx context.Context, rel string, files map[string]file, rows []db.ListImagesInDirRow) error {
	now := r.Now().UTC()
	for _, row := range rows {
		f, onDisk := files[row.Name]
		delete(files, row.Name)
		var write func(ctx context.Context, q *db.Queries) error
		switch {
		case row.Status == string(images.StatusOptimizing):
			continue
		case !onDisk && row.Status != string(images.StatusGone):
			r.counts.FilesGone++
			write = func(ctx context.Context, q *db.Queries) error {
				return q.MarkImageGone(ctx, db.MarkImageGoneParams{
					ID: row.ID, Now: now, ReadStatus: row.Status, ReadSize: row.Size, ReadMtimeNs: row.MtimeNs,
				})
			}
		case !onDisk:
			continue
		case row.Status == string(images.StatusGone):
			r.counts.FilesNew++
		case f.size != row.Size || f.mtimeNs != row.MtimeNs:
			r.counts.FilesChanged++
		default:
			continue
		}
		if write == nil {
			write = func(ctx context.Context, q *db.Queries) error {
				return q.MarkImagePending(ctx, db.MarkImagePendingParams{
					ID: row.ID, Size: f.size, MtimeNs: f.mtimeNs, Now: now,
					ReadStatus: row.Status, ReadSize: row.Size, ReadMtimeNs: row.MtimeNs,
				})
			}
		}
		if err := r.write(ctx, write); err != nil {
			return err
		}
	}
	for name, f := range files {
		r.counts.FilesNew++
		params := db.InsertImageParams{
			FolderID: r.folder.ID, Dir: rel, Name: name, Format: string(f.format), Size: f.size, MtimeNs: f.mtimeNs, Now: now,
		}
		if err := r.write(ctx, func(ctx context.Context, q *db.Queries) error { return q.InsertImage(ctx, params) }); err != nil {
			return err
		}
	}
	return nil
}

// remove forgets a directory that is gone, excluded, a symlink now or on another filesystem:
// every image under it becomes gone and its dirs rows go. The root is never removed; a missing
// root fails the scan instead.
func (r *run) remove(rel string) {
	r.removed[rel] = true
	now := r.Now().UTC()
	folderID := r.folder.ID
	// Queued like the other writes; an error surfaces from the flush that runs it.
	r.writes = append(r.writes, func(ctx context.Context, q *db.Queries) error {
		gone, err := q.MarkImagesGoneUnder(ctx, db.MarkImagesGoneUnderParams{FolderID: folderID, Dir: rel, Now: now})
		if err != nil {
			return err
		}
		r.counts.FilesGone += gone
		return q.DeleteDirsUnder(ctx, db.DeleteDirsUnderParams{FolderID: folderID, RelPath: rel})
	})
}

func (r *run) isRemoved(rel string) bool {
	for p := rel; ; p = parentOf(p) {
		if r.removed[p] {
			return true
		}
		if p == "" {
			return false
		}
	}
}

func (r *run) storeDir(ctx context.Context, rel string, mtime int64) error {
	r.known[rel] = mtime
	params := db.UpsertDirParams{FolderID: r.folder.ID, RelPath: rel, MtimeNs: mtime}
	return r.write(ctx, func(ctx context.Context, q *db.Queries) error { return q.UpsertDir(ctx, params) })
}

// write queues one write and commits the batch when it is full. Writes run in the order they were
// queued, so a directory's mtime is never stored before the rows of its files.
func (r *run) write(ctx context.Context, w func(ctx context.Context, q *db.Queries) error) error {
	r.writes = append(r.writes, w)
	if len(r.writes) < batchSize {
		return nil
	}
	return r.flush(ctx)
}

func (r *run) flush(ctx context.Context) error {
	if len(r.writes) == 0 {
		return nil
	}
	err := r.tx.WithTx(ctx, func(q *db.Queries) error {
		for _, w := range r.writes {
			if err := w(ctx, q); err != nil {
				return err
			}
		}
		return nil
	})
	r.writes = r.writes[:0]
	return err
}

// check stops the scan when the service stops, or when its folder was paused or deleted. The
// database is asked at most once per folderCheckInterval.
func (r *run) check(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !r.checkedAt.IsZero() && time.Since(r.checkedAt) < folderCheckInterval {
		return nil
	}
	r.checkedAt = time.Now()
	folder, err := r.queries.GetFolderByID(ctx, r.folderID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrFolderDeleted
	}
	if err != nil {
		return err
	}
	if !folder.Enabled {
		return ErrFolderPaused
	}
	r.folder = folder
	return nil
}

func (r *run) excluded(rel string) bool {
	for _, pattern := range r.excludes {
		if ok, _ := doublestar.Match(pattern, rel); ok {
			return true
		}
	}
	return false
}

// isTemp is the optimizer's temp file, .goptimizer-<random>.tmp. It is never indexed.
func isTemp(name string) bool {
	return strings.HasPrefix(name, ".goptimizer-") && strings.HasSuffix(name, ".tmp")
}

// cleanTemp deletes a temp file a crash left behind. Its age is the inode change time: the
// optimizer gives the temp file the original's mtime, but cannot set the ctime back.
func (r *run) cleanTemp(ctx context.Context, entry fs.DirEntry, dir string) {
	info, err := entry.Info()
	if err != nil || r.Now().Sub(changeTime(info)) <= tempMaxAge {
		return
	}
	if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil && !missing(err) {
		r.logger.WarnContext(ctx, "Could not delete a leftover temp file", "path", filepath.Join(dir, entry.Name()), "error", err)
	}
}

// missing is an error that means the path is no longer a directory to list: gone, replaced by a
// file, or replaced by a symlink (O_NOFOLLOW refuses it with ELOOP).
func missing(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) || errors.Is(err, syscall.ELOOP)
}

func parentOf(rel string) string {
	if i := strings.LastIndexByte(rel, '/'); i >= 0 {
		return rel[:i]
	}
	return ""
}

func deviceOf(info fs.FileInfo) uint64 {
	return uint64(info.Sys().(*syscall.Stat_t).Dev)
}
