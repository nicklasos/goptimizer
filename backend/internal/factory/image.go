package factory

import (
	"database/sql"
	"time"

	"app/internal/db"
	"app/internal/folders"
	"app/internal/images"
)

type imageAttrs struct {
	dir           string
	name          string
	size          int64
	mtimeNs       int64
	status        images.Status
	originalSize  sql.NullInt64
	optimizedSize sql.NullInt64
	mode          sql.NullString
	reason        sql.NullString
	attempts      int64
	nextAttemptAt sql.NullTime
	outputSHA256  []byte
	optimizedAt   sql.NullTime
}

type ImageOption func(*imageAttrs)

// At places the image: dir is relative to the folder (” for its root).
func At(dir, name string) ImageOption {
	return func(a *imageAttrs) { a.dir, a.name = dir, name }
}

// AsOnDisk records the size and mtime the scanner would have read.
func AsOnDisk(size, mtimeNs int64) ImageOption {
	return func(a *imageAttrs) { a.size, a.mtimeNs = size, mtimeNs }
}

// Optimized is an image the optimizer shrank, lossily, from originalSize to its size.
func Optimized(originalSize int64) ImageOption {
	return func(a *imageAttrs) {
		a.status = images.StatusOptimized
		a.originalSize = sql.NullInt64{Int64: originalSize, Valid: true}
		a.mode = sql.NullString{String: string(folders.ModeLossy), Valid: true}
		a.optimizedAt = sql.NullTime{Time: time.Now().UTC(), Valid: true}
	}
}

// WithOutputHash is the SHA-256 of the bytes the optimizer wrote to the file.
func WithOutputHash(sum []byte) ImageOption { return func(a *imageAttrs) { a.outputSHA256 = sum } }

func WithImageStatus(status images.Status) ImageOption {
	return func(a *imageAttrs) { a.status = status }
}

func WithReason(reason string) ImageOption {
	return func(a *imageAttrs) { a.reason = sql.NullString{String: reason, Valid: true} }
}

// Failed is an image whose tools failed attempts times, tried again at retryAt (zero: not again).
func Failed(attempts int64, retryAt time.Time) ImageOption {
	return func(a *imageAttrs) {
		a.status = images.StatusFailed
		a.attempts = attempts
		a.nextAttemptAt = sql.NullTime{Time: retryAt.UTC(), Valid: !retryAt.IsZero()}
	}
}

// Image creates a pending PNG in the folder's root that no file on disk backs.
func (f *Factory) Image(folder *db.Folder, opts ...ImageOption) *db.Image {
	f.t.Helper()

	attrs := imageAttrs{name: "image-" + unique() + ".png", size: 1024, mtimeNs: 1, status: images.StatusPending}
	for _, opt := range opts {
		opt(&attrs)
	}
	if attrs.status == images.StatusOptimized {
		attrs.optimizedSize = sql.NullInt64{Int64: attrs.size, Valid: true}
	}
	format, _ := images.FormatOf(attrs.name)
	image, err := f.queries.FactoryCreateImage(ctx, db.FactoryCreateImageParams{
		FolderID:      folder.ID,
		Dir:           attrs.dir,
		Name:          attrs.name,
		Format:        string(format),
		Size:          attrs.size,
		MtimeNs:       attrs.mtimeNs,
		Status:        string(attrs.status),
		OriginalSize:  attrs.originalSize,
		OptimizedSize: attrs.optimizedSize,
		Mode:          attrs.mode,
		Reason:        attrs.reason,
		Attempts:      attrs.attempts,
		NextAttemptAt: attrs.nextAttemptAt,
		OutputSha256:  attrs.outputSHA256,
		OptimizedAt:   attrs.optimizedAt,
		Now:           time.Now().UTC(),
	})
	if err != nil {
		f.t.Fatalf("factory.Image: %v", err)
	}
	return &image
}

// DailyStat records a day of a folder's optimizations: how many, and their bytes before and after.
func (f *Factory) DailyStat(folder *db.Folder, day time.Time, optimized, bytesBefore, bytesAfter int64) *db.DailyStat {
	f.t.Helper()

	stat, err := f.queries.FactoryCreateDailyStat(ctx, db.FactoryCreateDailyStatParams{
		FolderID:    folder.ID,
		Day:         images.Day(day),
		Optimized:   optimized,
		BytesBefore: bytesBefore,
		BytesAfter:  bytesAfter,
	})
	if err != nil {
		f.t.Fatalf("factory.DailyStat: %v", err)
	}
	return &stat
}
