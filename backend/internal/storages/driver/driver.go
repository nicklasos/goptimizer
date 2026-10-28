// Package driver talks to the places backups are kept: a local directory, an S3-compatible
// bucket or a Google Cloud Storage bucket. Every driver works with keys relative to its own
// root (the directory, or the bucket prefix) and streams object contents, so an archive of
// any size passes through without being held in memory.
package driver

import (
	"context"
	"errors"
	"io"
	"path"
	"sort"
	"strings"
	"time"
)

type Driver interface {
	// Put writes the object, replacing any object with the same key. The size is not known up front.
	Put(ctx context.Context, key string, r io.Reader) error
	// Open returns ErrNotFound for a key with no object.
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	// List returns one level under the prefix: its folders, then its objects, each by name.
	// A prefix with nothing under it is an empty list, not an error.
	List(ctx context.Context, prefix string) ([]Entry, error)
	// Delete succeeds when the object is already gone.
	Delete(ctx context.Context, key string) error
	// Close releases the connection the driver holds. Every driver that was opened is closed.
	Close() error
}

// Entry is one item of a listing. Name is relative to the listed prefix and has no slash.
type Entry struct {
	Name       string
	IsDir      bool
	Size       int64
	ModifiedAt time.Time
}

var (
	ErrInvalidKey = errors.New("invalid object key")
	ErrNotFound   = errors.New("object not found")
)

// CleanKey checks a key that names one object: relative, slash-separated, and with no
// segment that could climb out of the driver's root.
func CleanKey(key string) (string, error) {
	cleaned, err := CleanPrefix(key)
	if err != nil {
		return "", err
	}
	if cleaned == "" {
		return "", ErrInvalidKey
	}
	return strings.TrimSuffix(cleaned, "/"), nil
}

// CleanPrefix checks a folder for List. It returns "" for the root and "a/b/" otherwise.
func CleanPrefix(prefix string) (string, error) {
	if strings.HasPrefix(prefix, "/") || strings.ContainsAny(prefix, "\\\x00") {
		return "", ErrInvalidKey
	}
	for _, segment := range strings.Split(prefix, "/") {
		if segment == ".." {
			return "", ErrInvalidKey
		}
	}
	cleaned := path.Clean("/" + prefix)[1:]
	if cleaned == "" {
		return "", nil
	}
	return cleaned + "/", nil
}

// sortEntries puts folders first, then objects, each alphabetically.
func sortEntries(entries []Entry) []Entry {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return entries[i].Name < entries[j].Name
	})
	return entries
}

// joinPrefix puts a bucket prefix ("backups", "backups/", "") in front of a cleaned key or folder.
func joinPrefix(bucketPrefix, rest string) string {
	bucketPrefix = strings.Trim(bucketPrefix, "/")
	if bucketPrefix == "" {
		return rest
	}
	return bucketPrefix + "/" + rest
}
