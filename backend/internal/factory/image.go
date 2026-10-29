package factory

import (
	"database/sql"
	"time"

	"app/internal/db"
	"app/internal/images"
)

type imageAttrs struct {
	dir          string
	name         string
	size         int64
	mtimeNs      int64
	status       images.Status
	originalSize sql.NullInt64
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

// Optimized is an image the optimizer shrank from originalSize to its size.
func Optimized(originalSize int64) ImageOption {
	return func(a *imageAttrs) {
		a.status = images.StatusOptimized
		a.originalSize = sql.NullInt64{Int64: originalSize, Valid: true}
	}
}

func WithImageStatus(status images.Status) ImageOption {
	return func(a *imageAttrs) { a.status = status }
}

// Image creates a pending PNG in the folder's root that no file on disk backs.
func (f *Factory) Image(folder *db.Folder, opts ...ImageOption) *db.Image {
	f.t.Helper()

	attrs := imageAttrs{name: "image-" + unique() + ".png", size: 1024, mtimeNs: 1, status: images.StatusPending}
	for _, opt := range opts {
		opt(&attrs)
	}
	format, _ := images.FormatOf(attrs.name)
	image, err := f.queries.FactoryCreateImage(ctx, db.FactoryCreateImageParams{
		FolderID:     folder.ID,
		Dir:          attrs.dir,
		Name:         attrs.name,
		Format:       string(format),
		Size:         attrs.size,
		MtimeNs:      attrs.mtimeNs,
		Status:       string(attrs.status),
		OriginalSize: attrs.originalSize,
		Now:          time.Now().UTC(),
	})
	if err != nil {
		f.t.Fatalf("factory.Image: %v", err)
	}
	return &image
}
