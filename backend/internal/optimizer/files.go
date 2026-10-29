package optimizer

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// openDir opens the image's directory under the folder's root one component at a time, refusing a
// symlink at every level, root included. Every later step works relative to this descriptor, so
// a directory renamed or replaced by a symlink while the tools run cannot redirect the swap.
func openDir(root, rel string) (*os.File, error) {
	fd, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	if rel != "" {
		for _, name := range strings.Split(rel, "/") {
			next, err := unix.Openat(fd, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
			unix.Close(fd)
			if err != nil {
				return nil, err
			}
			fd = next
		}
	}
	return os.NewFile(uintptr(fd), root+"/"+rel), nil
}

func lstatAt(dir *os.File, name string) (unix.Stat_t, error) {
	var st unix.Stat_t
	err := unix.Fstatat(int(dir.Fd()), name, &st, unix.AT_SYMLINK_NOFOLLOW)
	return st, err
}

// sameFile is the test before the swap: the original is the file that was read, unchanged.
func sameFile(a, b unix.Stat_t) bool {
	return a.Ino == b.Ino && a.Size == b.Size && a.Mtim == b.Mtim && a.Nlink == b.Nlink
}

func isRegular(st unix.Stat_t) bool { return st.Mode&unix.S_IFMT == unix.S_IFREG }
func isSymlink(st unix.Stat_t) bool { return st.Mode&unix.S_IFMT == unix.S_IFLNK }
func mtimeNs(st unix.Stat_t) int64  { return unix.TimespecToNsec(st.Mtim) }

var errChanged = errors.New("the file changed while it was read")

// readFile reads the file that lstat described, all of it. It fails with errChanged when what was
// opened is not that file or its size or mtime moved during the read.
func readFile(dir *os.File, name string, st unix.Stat_t) ([]byte, error) {
	fd, err := unix.Openat(int(dir.Fd()), name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), name)
	defer f.Close()
	var opened unix.Stat_t
	if err := unix.Fstat(fd, &opened); err != nil {
		return nil, err
	}
	if !sameFile(st, opened) {
		return nil, errChanged
	}
	data, err := io.ReadAll(io.LimitReader(f, st.Size+1))
	if err != nil {
		return nil, err
	}
	var after unix.Stat_t
	if err := unix.Fstat(fd, &after); err != nil {
		return nil, err
	}
	if int64(len(data)) != st.Size || !sameFile(st, after) {
		return nil, errChanged
	}
	return data, nil
}

// changedError is an original that is no longer the file the tools started from. stat is the
// file now; missing is set when there is none.
type changedError struct {
	stat    unix.Stat_t
	missing bool
}

func (e *changedError) Error() string { return "the file changed while it was optimized" }

// swap replaces name in dir with data and gives it the original's owner, group, mode, access time
// and modification time, as orig describes it. The data goes to a new temp file next to it,
// .goptimizer-<random>.tmp, which is fsynced; the original is lstatted again immediately before
// the rename and must still be the file that was read, then the rename replaces it atomically
// and the directory is fsynced. Any failure before the rename removes the temp file.
//
// The window between that last lstat and the rename cannot be closed without locks the projects'
// writers do not take: a write in it is lost. It is a few system calls long.
func swap(dir *os.File, name string, data []byte, orig unix.Stat_t) error {
	dirfd := int(dir.Fd())
	tmp, err := tempName()
	if err != nil {
		return err
	}
	fd, err := unix.Openat(dirfd, tmp, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0o600)
	if err != nil {
		return fmt.Errorf("create the temp file: %w", err)
	}
	renamed := false
	defer func() {
		if !renamed {
			_ = unix.Unlinkat(dirfd, tmp, 0)
		}
	}()
	f := os.NewFile(uintptr(fd), tmp)
	err = writeTemp(f, dirfd, tmp, data, orig)
	if cerr := f.Close(); err == nil && cerr != nil {
		err = fmt.Errorf("close the temp file: %w", cerr)
	}
	if err != nil {
		return err
	}

	now, err := lstatAt(dir, name)
	if errors.Is(err, unix.ENOENT) {
		return &changedError{missing: true}
	}
	if err != nil {
		return err
	}
	if !sameFile(orig, now) {
		return &changedError{stat: now}
	}
	if err := unix.Renameat(dirfd, tmp, dirfd, name); err != nil {
		return fmt.Errorf("rename over the original: %w", err)
	}
	renamed = true
	if err := dir.Sync(); err != nil {
		return fmt.Errorf("%w: %v", errDirSync, err)
	}
	return nil
}

// errDirSync is a directory fsync that failed after the rename: the new file is in place.
var errDirSync = errors.New("fsync the directory after the rename")

// readAllLimit reads r to its end, failing when it holds more than limit bytes.
func readAllLimit(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err == nil && int64(len(data)) > limit {
		err = fmt.Errorf("larger than %d bytes", limit)
	}
	return data, err
}

// writeTemp fills the temp file and makes it look like the original. The owner is set before the
// mode, because chown clears the setuid and setgid bits; the times last, after every write.
func writeTemp(f *os.File, dirfd int, tmp string, data []byte, orig unix.Stat_t) error {
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write the temp file: %w", err)
	}
	if err := f.Chown(int(orig.Uid), int(orig.Gid)); err != nil {
		return fmt.Errorf("give the temp file the original's owner: %w", err)
	}
	if err := unix.Fchmod(int(f.Fd()), uint32(orig.Mode)&0o7777); err != nil {
		return fmt.Errorf("give the temp file the original's mode: %w", err)
	}
	times := []unix.Timespec{orig.Atim, orig.Mtim}
	if err := unix.UtimesNanoAt(dirfd, tmp, times, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return fmt.Errorf("give the temp file the original's times: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("fsync the temp file: %w", err)
	}
	return nil
}

func tempName() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return ".goptimizer-" + hex.EncodeToString(b) + ".tmp", nil
}
