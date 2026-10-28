package folders

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"app/internal/errs"
)

type Kind string

const KindLocal Kind = "local"

// kind is what differs between the places a folder can live. SSH or S3 are new entries.
type kind struct {
	// resolve checks the path a user typed and returns the form to store, or the errors of "path".
	resolve func(path string) (string, errs.Fields)
}

var kinds = map[Kind]kind{
	KindLocal: {resolve: resolveLocalPath},
}

// systemRoots are refused, with everything inside them. Any top-level "lib*" is too.
var systemRoots = []string{"/proc", "/sys", "/dev", "/boot", "/etc", "/bin", "/sbin", "/usr", "/var/lib"}

func resolveLocalPath(raw string) (string, errs.Fields) {
	path := strings.TrimSpace(raw)
	if !filepath.IsAbs(path) {
		return "", pathError(errs.ErrKeyFoldersPathNotAbsolute)
	}
	path = filepath.Clean(path)
	if isSystemPath(path) {
		return "", pathError(errs.ErrKeyFoldersPathSystem)
	}

	resolved, err := filepath.EvalSymlinks(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", pathError(errs.ErrKeyFoldersPathNotFound)
	}
	if err != nil {
		return "", pathError(errs.ErrKeyFoldersPathUnreadable)
	}
	// Checked again after resolving: a link can point into a system root, and on macOS /etc is /private/etc.
	if isSystemPath(resolved) {
		return "", pathError(errs.ErrKeyFoldersPathSystem)
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return "", pathError(errs.ErrKeyFoldersPathUnreadable)
	}
	if !info.IsDir() {
		return "", pathError(errs.ErrKeyFoldersPathNotDirectory)
	}
	dir, err := os.Open(resolved)
	if err != nil {
		return "", pathError(errs.ErrKeyFoldersPathUnreadable)
	}
	defer dir.Close()
	if _, err := dir.Readdirnames(1); err != nil && err != io.EOF {
		return "", pathError(errs.ErrKeyFoldersPathUnreadable)
	}
	return resolved, nil
}

func isSystemPath(path string) bool {
	if path == "/" {
		return true
	}
	top, _, _ := strings.Cut(strings.TrimPrefix(path, "/"), "/")
	if strings.HasPrefix(top, "lib") {
		return true
	}
	for _, root := range systemRoots {
		if within(path, root) {
			return true
		}
	}
	return false
}

// Overlaps reports whether one cleaned absolute path is the other or inside it. It compares
// whole segments, so /srv/a and /srv/ab do not overlap.
func Overlaps(a, b string) bool {
	return within(a, b) || within(b, a)
}

func within(path, root string) bool {
	return path == root || strings.HasPrefix(path, strings.TrimSuffix(root, "/")+"/")
}

func pathError(key string) errs.Fields {
	return errs.Fields{"path": {key}}
}
