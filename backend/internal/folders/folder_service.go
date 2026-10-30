package folders

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"maps"
	"strings"

	"app/internal/db"
	"app/internal/errs"
	"app/internal/tools"
)

var ErrFolderNotFound = errs.NewNotFoundError(errs.ErrKeyFoldersNotFound, "Folder not found")

type FolderService struct {
	queries  *db.Queries
	reporter *tools.Reporter
}

func NewFolderService(queries *db.Queries, reporter *tools.Reporter) *FolderService {
	return &FolderService{queries: queries, reporter: reporter}
}

// Folder is a folder with the state of its index: its latest scan, nil before the first, how many
// images it has in each status, the bytes its optimizations saved out of the bytes the optimized
// images had before, and the programs its profile needs that are not installed.
type Folder struct {
	db.Folder
	Profile      Profile
	LastScan     *db.Scan
	ImageCounts  map[string]int64
	BytesSaved   int64
	BytesBefore  int64
	MissingTools []string
}

// List returns every folder with its index state, in four queries whatever the number of folders.
func (s *FolderService) List(ctx context.Context) ([]Folder, error) {
	folders, err := s.queries.ListFolders(ctx)
	if err != nil {
		return nil, errs.WrapDatabaseError(err)
	}
	latest, err := s.queries.ListLatestScans(ctx)
	if err != nil {
		return nil, errs.WrapDatabaseError(err)
	}
	counts, err := s.queries.CountImagesByStatus(ctx)
	if err != nil {
		return nil, errs.WrapDatabaseError(err)
	}
	savings, err := s.queries.SumBytesSavedByFolder(ctx)
	if err != nil {
		return nil, errs.WrapDatabaseError(err)
	}
	saved := make(map[int64]db.SumBytesSavedByFolderRow, len(savings))
	for _, row := range savings {
		saved[row.FolderID] = row
	}
	report := s.reporter.Report(ctx)
	scans := make(map[int64]*db.Scan, len(latest))
	for i := range latest {
		scans[latest[i].FolderID] = &latest[i]
	}
	byFolder := make(map[int64]map[string]int64, len(folders))
	for _, c := range counts {
		if byFolder[c.FolderID] == nil {
			byFolder[c.FolderID] = map[string]int64{}
		}
		byFolder[c.FolderID][c.Status] = c.Count
	}
	result := make([]Folder, len(folders))
	for i, folder := range folders {
		result[i] = withProfile(Folder{Folder: folder, LastScan: scans[folder.ID], ImageCounts: byFolder[folder.ID], BytesSaved: saved[folder.ID].Saved, BytesBefore: saved[folder.ID].Original}, report)
		if result[i].ImageCounts == nil {
			result[i].ImageCounts = map[string]int64{}
		}
	}
	return result, nil
}

// withProfile parses the stored profile, which is only ever written from a validated one, and
// names the programs it needs that are missing.
func withProfile(f Folder, report tools.Report) Folder {
	_ = json.Unmarshal([]byte(f.Folder.Profile), &f.Profile)
	f.MissingTools = f.Profile.MissingTools(report)
	return f
}

func (s *FolderService) Get(ctx context.Context, id int64) (*Folder, error) {
	folder, err := s.queries.GetFolderByID(ctx, id)
	if err != nil {
		return nil, notFoundOr(err)
	}
	return s.withIndex(ctx, folder)
}

func (s *FolderService) Create(ctx context.Context, req FolderRequest) (*Folder, error) {
	params, err := s.parse(ctx, 0, req)
	if err != nil {
		return nil, err
	}
	folder, err := s.queries.CreateFolder(ctx, params)
	if err != nil {
		return nil, uniqueOr(err)
	}
	return s.withIndex(ctx, folder)
}

func (s *FolderService) Update(ctx context.Context, id int64, req FolderRequest) (*Folder, error) {
	if _, err := s.Get(ctx, id); err != nil {
		return nil, err
	}
	params, err := s.parse(ctx, id, req)
	if err != nil {
		return nil, err
	}
	folder, err := s.queries.UpdateFolder(ctx, db.UpdateFolderParams{
		ID:                    id,
		Name:                  params.Name,
		Kind:                  params.Kind,
		Path:                  params.Path,
		ScanIntervalMinutes:   params.ScanIntervalMinutes,
		FullScanIntervalHours: params.FullScanIntervalHours,
		Profile:               params.Profile,
	})
	if err != nil {
		return nil, uniqueOr(err)
	}
	return s.withIndex(ctx, folder)
}

func (s *FolderService) SetEnabled(ctx context.Context, id int64, enabled bool) (*Folder, error) {
	folder, err := s.queries.SetFolderEnabled(ctx, db.SetFolderEnabledParams{ID: id, Enabled: enabled})
	if err != nil {
		return nil, notFoundOr(err)
	}
	return s.withIndex(ctx, folder)
}

func (s *FolderService) Delete(ctx context.Context, id int64) error {
	deleted, err := s.queries.DeleteFolder(ctx, id)
	if err != nil {
		return errs.WrapDatabaseError(err)
	}
	if deleted == 0 {
		return ErrFolderNotFound
	}
	return nil
}

// parse checks a request against the rules and the other folders, and returns the columns to
// store. id is the folder being replaced, 0 for a new one.
func (s *FolderService) parse(ctx context.Context, id int64, req FolderRequest) (db.CreateFolderParams, error) {
	fields := errs.Fields{}
	maps.Copy(fields, req.Profile.Validate())

	name := strings.TrimSpace(req.Name)
	if name == "" {
		fields.Add("name", "required")
	}

	var path string
	if k, ok := kinds[req.Kind]; !ok {
		fields.Add("kind", "oneof")
	} else if resolved, pathErrors := k.resolve(req.Path); pathErrors != nil {
		maps.Copy(fields, pathErrors)
	} else {
		overlaps, err := s.overlapsAnother(ctx, id, resolved)
		if err != nil {
			return db.CreateFolderParams{}, err
		}
		if overlaps {
			fields["path"] = []string{errs.ErrKeyFoldersPathOverlaps}
		}
		path = resolved
	}

	if len(fields) > 0 {
		return db.CreateFolderParams{}, fields
	}
	profile, err := json.Marshal(req.Profile)
	if err != nil {
		return db.CreateFolderParams{}, err
	}
	return db.CreateFolderParams{
		Name:                  name,
		Kind:                  string(req.Kind),
		Path:                  path,
		ScanIntervalMinutes:   req.ScanIntervalMinutes,
		FullScanIntervalHours: req.FullScanIntervalHours,
		Profile:               string(profile),
	}, nil
}

func (s *FolderService) overlapsAnother(ctx context.Context, id int64, path string) (bool, error) {
	folders, err := s.queries.ListFolders(ctx)
	if err != nil {
		return false, errs.WrapDatabaseError(err)
	}
	for _, folder := range folders {
		if folder.ID != id && Overlaps(folder.Path, path) {
			return true, nil
		}
	}
	return false, nil
}

func (s *FolderService) withIndex(ctx context.Context, folder db.Folder) (*Folder, error) {
	result := withProfile(Folder{Folder: folder, ImageCounts: map[string]int64{}}, s.reporter.Report(ctx))
	scan, err := s.queries.GetLatestScanOfFolder(ctx, folder.ID)
	switch {
	case err == nil:
		result.LastScan = &scan
	case !errors.Is(err, sql.ErrNoRows):
		return nil, errs.WrapDatabaseError(err)
	}
	counts, err := s.queries.CountImagesOfFolderByStatus(ctx, folder.ID)
	if err != nil {
		return nil, errs.WrapDatabaseError(err)
	}
	for _, c := range counts {
		result.ImageCounts[c.Status] = c.Count
	}
	sums, err := s.queries.SumBytesSavedOfFolder(ctx, folder.ID)
	if err != nil {
		return nil, errs.WrapDatabaseError(err)
	}
	result.BytesSaved, result.BytesBefore = sums.Saved, sums.Original
	return &result, nil
}

func notFoundOr(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrFolderNotFound
	}
	return errs.WrapDatabaseError(err)
}

// uniqueOr maps the unique indexes to field errors. The overlap check runs first, so a
// duplicate path gets here only when two saves race.
func uniqueOr(err error) error {
	switch {
	case errs.IsUniqueViolation(err, "idx_folders_name_lower"):
		return errs.Fields{"name": {errs.ErrKeyFoldersNameTaken}}
	case errs.IsUniqueViolation(err, "folders.path"):
		return errs.Fields{"path": {errs.ErrKeyFoldersPathOverlaps}}
	}
	return errs.WrapDatabaseError(err)
}
