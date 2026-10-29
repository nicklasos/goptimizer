package images

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strings"
	"time"

	"app/internal/db"
	"app/internal/errs"
)

var (
	ErrFolderNotFound = errs.NewNotFoundError(errs.ErrKeyFoldersNotFound, "Folder not found")
	ErrImageNotFound  = errs.NewNotFoundError(errs.ErrKeyImagesNotFound, "Image not found")
	ErrImageNotFailed = errs.NewDomainError(errs.ErrKeyImagesNotFailed, "Only a failed image can be retried", http.StatusConflict)
)

// Sort is the order a page of a folder's images comes in. A search lists its matches by path, the
// one order that stays fast for a text anywhere in the path.
type Sort string

const (
	SortUpdated Sort = "updated"
	SortSaved   Sort = "saved"
	SortPath    Sort = "path"
)

// ChartDays is how many UTC days the savings chart covers, today included.
const ChartDays = 30

// Query is one page of a folder's images: of one status, or every status when Status is empty,
// whose path contains Search when it is set, after the cursor. Sort is SortUpdated or, for
// optimized images, SortSaved.
type Query struct {
	Status Status
	Search string
	Sort   Sort
	After  *Cursor
	Limit  int64
}

// Cursor is the sort key of the last image of a page; the next page starts after it.
type Cursor struct {
	UpdatedAt time.Time `json:"u,omitzero"`
	Saved     int64     `json:"s,omitempty"`
	Dir       string    `json:"d,omitempty"`
	Name      string    `json:"n,omitempty"`
	ID        int64     `json:"i,omitempty"`
}

// Page is a page of images, the order they are in, and where the next page starts: nil when this
// is the last.
type Page struct {
	Images []db.Image
	Sort   Sort
	Next   *Cursor
}

// DaySavings is what the optimizations of one UTC day saved.
type DaySavings struct {
	Day        string
	Optimized  int64
	BytesSaved int64
}

type ImageService struct {
	queries *db.Queries
}

func NewImageService(queries *db.Queries) *ImageService {
	return &ImageService{queries: queries}
}

// List returns one page of the folder's images. Each page is one index seek and a short scan,
// however many images the folder has, except a search: that reads idx_images_path until the page
// is full, the whole index at worst.
func (s *ImageService) List(ctx context.Context, folderID int64, q Query) (Page, error) {
	if err := s.folderExists(ctx, folderID); err != nil {
		return Page{}, err
	}
	order := q.Sort
	if q.Search != "" {
		order = SortPath
	}
	after := Cursor{UpdatedAt: time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC), Saved: math.MaxInt64, ID: math.MaxInt64}
	if q.After != nil {
		after = *q.After
	}
	limit := q.Limit + 1
	var rows []db.Image
	var err error
	switch {
	case order == SortPath && q.Status == "":
		rows, err = s.queries.SearchImages(ctx, db.SearchImagesParams{
			FolderID: folderID, Dir: after.Dir, Name: after.Name, Needle: strings.ToLower(q.Search), PageLimit: limit,
		})
	case order == SortPath:
		rows, err = s.queries.SearchImagesOfStatus(ctx, db.SearchImagesOfStatusParams{
			FolderID: folderID, Status: string(q.Status), Dir: after.Dir, Name: after.Name, Needle: strings.ToLower(q.Search), PageLimit: limit,
		})
	case order == SortSaved:
		rows, err = s.queries.ListOptimizedImagesBySaved(ctx, db.ListOptimizedImagesBySavedParams{
			FolderID: folderID, Saved: sql.NullInt64{Int64: after.Saved, Valid: true}, ID: after.ID, PageLimit: limit,
		})
	case q.Status == "":
		rows, err = s.queries.ListImagesByUpdated(ctx, db.ListImagesByUpdatedParams{
			FolderID: folderID, UpdatedAt: after.UpdatedAt.UTC(), ID: after.ID, PageLimit: limit,
		})
	default:
		rows, err = s.queries.ListImagesOfStatusByUpdated(ctx, db.ListImagesOfStatusByUpdatedParams{
			FolderID: folderID, Status: string(q.Status), UpdatedAt: after.UpdatedAt.UTC(), ID: after.ID, PageLimit: limit,
		})
	}
	if err != nil {
		return Page{}, errs.WrapDatabaseError(err)
	}
	page := Page{Images: rows, Sort: order}
	if int64(len(rows)) > q.Limit {
		page.Images = rows[:q.Limit]
		next := cursorOf(page.Images[q.Limit-1], order)
		page.Next = &next
	}
	return page, nil
}

func cursorOf(image db.Image, sort Sort) Cursor {
	switch sort {
	case SortPath:
		return Cursor{Dir: image.Dir, Name: image.Name}
	case SortSaved:
		return Cursor{Saved: image.OriginalSize.Int64 - image.OptimizedSize.Int64, ID: image.ID}
	default:
		return Cursor{UpdatedAt: image.UpdatedAt.UTC(), ID: image.ID}
	}
}

// EncodeCursor is the cursor as the API sends it: opaque to clients.
func EncodeCursor(c Cursor) string {
	data, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(data)
}

// DecodeCursor reads a cursor EncodeCursor wrote.
func DecodeCursor(s string) (Cursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, err
	}
	var c Cursor
	return c, json.Unmarshal(data, &c)
}

// Retry makes a failed image pending again, with its attempts back: one conditional UPDATE.
func (s *ImageService) Retry(ctx context.Context, id int64, now time.Time) (db.Image, error) {
	n, err := s.queries.RetryImage(ctx, db.RetryImageParams{ID: id, Now: now.UTC()})
	if err != nil {
		return db.Image{}, errs.WrapDatabaseError(err)
	}
	image, err := s.queries.GetImage(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return db.Image{}, ErrImageNotFound
	}
	if err != nil {
		return db.Image{}, errs.WrapDatabaseError(err)
	}
	if n == 0 {
		return db.Image{}, ErrImageNotFailed
	}
	return image, nil
}

// RetryFailed makes every failed image of the folder pending again, with its attempts back, and
// returns how many there were.
func (s *ImageService) RetryFailed(ctx context.Context, folderID int64, now time.Time) (int64, error) {
	if err := s.folderExists(ctx, folderID); err != nil {
		return 0, err
	}
	n, err := s.queries.RetryFailedImages(ctx, db.RetryFailedImagesParams{FolderID: folderID, Now: now.UTC()})
	if err != nil {
		return 0, errs.WrapDatabaseError(err)
	}
	return n, nil
}

// Savings is what the optimizations saved on each of the ChartDays UTC days up to now, oldest
// first, a day without any as zero. A nil folderID is every folder together.
func (s *ImageService) Savings(ctx context.Context, folderID *int64, now time.Time) ([]DaySavings, error) {
	first := now.UTC().AddDate(0, 0, 1-ChartDays)
	since := Day(first)
	byDay := map[string]DaySavings{}
	if folderID == nil {
		rows, err := s.queries.ListDailySavings(ctx, since)
		if err != nil {
			return nil, errs.WrapDatabaseError(err)
		}
		for _, row := range rows {
			byDay[row.Day] = DaySavings{Day: row.Day, Optimized: row.Optimized, BytesSaved: row.Saved}
		}
	} else {
		if err := s.folderExists(ctx, *folderID); err != nil {
			return nil, err
		}
		rows, err := s.queries.ListDailySavingsOfFolder(ctx, db.ListDailySavingsOfFolderParams{FolderID: *folderID, Since: since})
		if err != nil {
			return nil, errs.WrapDatabaseError(err)
		}
		for _, row := range rows {
			byDay[row.Day] = DaySavings{Day: row.Day, Optimized: row.Optimized, BytesSaved: row.Saved}
		}
	}
	days := make([]DaySavings, ChartDays)
	for i := range days {
		day := Day(first.AddDate(0, 0, i))
		days[i] = DaySavings{Day: day}
		if saved, ok := byDay[day]; ok {
			days[i] = saved
		}
	}
	return days, nil
}

func (s *ImageService) folderExists(ctx context.Context, id int64) error {
	_, err := s.queries.GetFolderByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrFolderNotFound
	}
	if err != nil {
		return errs.WrapDatabaseError(err)
	}
	return nil
}
