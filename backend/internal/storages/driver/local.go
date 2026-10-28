package driver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Local keeps objects as files under one directory on this server. The directory must exist:
// a mistyped path fails the connection test instead of quietly creating a new tree.
type Local struct {
	root string
}

func NewLocal(root string) (*Local, error) {
	if !filepath.IsAbs(root) {
		return nil, fmt.Errorf("local storage path must be absolute, got %q", root)
	}
	return &Local{root: filepath.Clean(root)}, nil
}

// openRoot gives an os.Root, whose operations refuse to leave the directory, through ".." or
// a symlink alike. CleanKey already rejects ".."; the root is the guard that does not depend on it.
func (l *Local) openRoot() (*os.Root, error) {
	return os.OpenRoot(l.root)
}

func (l *Local) Put(ctx context.Context, key string, r io.Reader) error {
	key, err := CleanKey(key)
	if err != nil {
		return err
	}
	root, err := l.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()

	if dir := path.Dir(key); dir != "." {
		if err := root.MkdirAll(dir, 0o750); err != nil {
			return err
		}
	}

	// Written beside the target and renamed into place, so a reader never sees half an archive.
	tmp := key + ".tmp-" + randomSuffix()
	if err := writeFile(ctx, root, tmp, r); err != nil {
		_ = root.Remove(tmp)
		return err
	}
	if err := root.Rename(tmp, key); err != nil {
		_ = root.Remove(tmp)
		return err
	}
	return nil
}

func writeFile(ctx context.Context, root *os.Root, name string, r io.Reader) error {
	f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, contextReader{ctx: ctx, r: r}); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func (l *Local) Open(_ context.Context, key string) (io.ReadCloser, error) {
	key, err := CleanKey(key)
	if err != nil {
		return nil, err
	}
	root, err := l.openRoot()
	if err != nil {
		return nil, err
	}
	defer root.Close()

	f, err := root.Open(key)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if info, err := f.Stat(); err != nil || info.IsDir() {
		f.Close()
		return nil, ErrNotFound
	}
	return f, nil
}

func (l *Local) List(_ context.Context, prefix string) ([]Entry, error) {
	prefix, err := CleanPrefix(prefix)
	if err != nil {
		return nil, err
	}
	root, err := l.openRoot()
	if err != nil {
		return nil, err
	}
	defer root.Close()

	dir := strings.TrimSuffix(prefix, "/")
	if dir == "" {
		dir = "."
	}
	f, err := root.Open(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return []Entry{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	items, err := f.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(items))
	for _, item := range items {
		info, err := item.Info()
		if err != nil {
			continue
		}
		entry := Entry{Name: item.Name(), IsDir: item.IsDir(), ModifiedAt: info.ModTime().UTC()}
		if !entry.IsDir {
			entry.Size = info.Size()
		}
		entries = append(entries, entry)
	}
	return sortEntries(entries), nil
}

func (l *Local) Delete(_ context.Context, key string) error {
	key, err := CleanKey(key)
	if err != nil {
		return err
	}
	root, err := l.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()

	if err := root.Remove(key); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// Close has nothing to release: every call opens and closes the directory itself.
func (l *Local) Close() error { return nil }

func randomSuffix() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// contextReader stops a long copy when the request or job is cancelled.
type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (c contextReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}
