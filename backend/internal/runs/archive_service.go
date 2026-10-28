package runs

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
	"time"

	"app/internal/db"
	"app/internal/errs"
	"app/internal/secretbox"
	"app/internal/storages"
	"app/internal/storages/driver"
)

// DownloadLinkTTL is how long a download link works. The browser follows it at once; a link
// that leaks through history or a log is dead by the time anyone reads it.
const DownloadLinkTTL = time.Minute

const downloadLinkPurpose = "archive-download"

var (
	ErrArchiveNotFound = errs.NewNotFoundError(errs.ErrKeyArchivesNotFound, "Archive not found")
	ErrInvalidLink     = errs.NewForbiddenError(errs.ErrKeyArchivesInvalidLink, "The download link is invalid or has expired")
)

// Download is an archive's object, open for reading. Closing it closes the object and the
// storage it came from.
type Download struct {
	Name string
	Size int64
	body io.ReadCloser
	d    driver.Driver
}

func (d *Download) Read(p []byte) (int, error) { return d.body.Read(p) }

func (d *Download) Close() error { return errors.Join(d.body.Close(), d.d.Close()) }

// ArchiveService hands out the archives that runs stored.
type ArchiveService struct {
	queries  *db.Queries
	storages *storages.StorageService
	secrets  *secretbox.Box
}

func NewArchiveService(queries *db.Queries, storageService *storages.StorageService, secrets *secretbox.Box) *ArchiveService {
	return &ArchiveService{queries: queries, storages: storageService, secrets: secrets}
}

// Open opens a live archive's object in its storage. An archive that retention deleted is a 404;
// a storage that fails, or no longer has the object, a 502.
func (s *ArchiveService) Open(ctx context.Context, id int64) (*Download, error) {
	archive, err := liveArchive(ctx, s.queries, id)
	if err != nil {
		return nil, err
	}
	storage, err := s.storages.GetStorage(ctx, archive.StorageID)
	if err != nil {
		return nil, err
	}
	d, err := s.storages.Driver(ctx, storage)
	if err != nil {
		return nil, storages.Unreachable(err)
	}
	body, err := d.Open(ctx, archive.ObjectKey)
	if err != nil {
		d.Close()
		return nil, storages.Unreachable(err)
	}
	return &Download{Name: path.Base(archive.ObjectKey), Size: archive.SizeBytes, body: body, d: d}, nil
}

// liveArchive is an archive that retention has not deleted, or a 404.
func liveArchive(ctx context.Context, q *db.Queries, id int64) (db.Archive, error) {
	archive, err := q.GetArchiveByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && archive.DeletedAt.Valid) {
		return db.Archive{}, ErrArchiveNotFound
	}
	if err != nil {
		return db.Archive{}, errs.WrapDatabaseError(err)
	}
	return archive, nil
}

// Link is a token that downloads the archive without a session until DownloadLinkTTL from now,
// so a browser can save a large archive straight to disk instead of holding it in memory. It
// is sealed with the key that seals credentials: it cannot be forged or changed to name
// another archive.
func (s *ArchiveService) Link(ctx context.Context, id int64, now time.Time) (string, time.Time, error) {
	if _, err := liveArchive(ctx, s.queries, id); err != nil {
		return "", time.Time{}, err
	}
	expires := now.Add(DownloadLinkTTL).UTC().Truncate(time.Second)
	plain := fmt.Sprintf("%s:%d:%d", downloadLinkPurpose, id, expires.Unix())
	return base64.RawURLEncoding.EncodeToString(s.secrets.Seal([]byte(plain))), expires, nil
}

// OpenLink opens the archive a token from Link names, while the token is valid.
func (s *ArchiveService) OpenLink(ctx context.Context, token string, now time.Time) (*Download, error) {
	id, ok := s.readLink(token, now)
	if !ok {
		return nil, ErrInvalidLink
	}
	return s.Open(ctx, id)
}

func (s *ArchiveService) readLink(token string, now time.Time) (int64, bool) {
	sealed, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return 0, false
	}
	plain, err := s.secrets.Open(sealed)
	if err != nil {
		return 0, false
	}
	parts := strings.Split(string(plain), ":")
	if len(parts) != 3 || parts[0] != downloadLinkPurpose {
		return 0, false
	}
	id, idErr := strconv.ParseInt(parts[1], 10, 64)
	expires, expErr := strconv.ParseInt(parts[2], 10, 64)
	if idErr != nil || expErr != nil || !now.Before(time.Unix(expires, 0)) {
		return 0, false
	}
	return id, true
}
