package factory

import (
	"encoding/json"

	"app/internal/db"
	"app/internal/folders"
)

type folderAttrs struct {
	name          string
	path          string
	enabled       bool
	profile       folders.Profile
	scanMinutes   int64
	fullScanHours int64
}

type FolderOption func(*folderAttrs)

func WithFolderName(name string) FolderOption { return func(a *folderAttrs) { a.name = name } }

// WithFolderPath sets the stored path. The factory does not check it exists.
func WithFolderPath(path string) FolderOption { return func(a *folderAttrs) { a.path = path } }

func WithProfile(profile folders.Profile) FolderOption {
	return func(a *folderAttrs) { a.profile = profile }
}

// WithScanIntervals sets the quick scan interval in minutes and the full scan interval in hours.
func WithScanIntervals(minutes, hours int64) FolderOption {
	return func(a *folderAttrs) { a.scanMinutes, a.fullScanHours = minutes, hours }
}

// Paused is a folder an admin switched off.
func Paused() FolderOption { return func(a *folderAttrs) { a.enabled = false } }

// Folder creates an enabled local folder with the default intervals and profile, at a path
// under /srv that does not exist.
func (f *Factory) Folder(opts ...FolderOption) *db.Folder {
	f.t.Helper()

	id := unique()
	attrs := folderAttrs{
		name:          "Folder " + id,
		path:          "/srv/factory-" + id,
		enabled:       true,
		profile:       folders.DefaultProfile(),
		scanMinutes:   folders.DefaultScanIntervalMinutes,
		fullScanHours: folders.DefaultFullScanIntervalHours,
	}
	for _, opt := range opts {
		opt(&attrs)
	}

	profile, err := json.Marshal(attrs.profile)
	if err != nil {
		f.t.Fatalf("factory.Folder: %v", err)
	}
	folder, err := f.queries.FactoryCreateFolder(ctx, db.FactoryCreateFolderParams{
		Name:                  attrs.name,
		Kind:                  string(folders.KindLocal),
		Path:                  attrs.path,
		Enabled:               attrs.enabled,
		ScanIntervalMinutes:   attrs.scanMinutes,
		FullScanIntervalHours: attrs.fullScanHours,
		Profile:               string(profile),
	})
	if err != nil {
		f.t.Fatalf("factory.Folder: %v", err)
	}
	return &folder
}
