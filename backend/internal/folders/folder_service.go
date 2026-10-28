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
)

var ErrFolderNotFound = errs.NewNotFoundError(errs.ErrKeyFoldersNotFound, "Folder not found")

type FolderService struct {
	queries *db.Queries
}

func NewFolderService(queries *db.Queries) *FolderService {
	return &FolderService{queries: queries}
}

func (s *FolderService) List(ctx context.Context) ([]db.Folder, error) {
	folders, err := s.queries.ListFolders(ctx)
	if err != nil {
		return nil, errs.WrapDatabaseError(err)
	}
	return folders, nil
}

func (s *FolderService) Get(ctx context.Context, id int64) (*db.Folder, error) {
	folder, err := s.queries.GetFolderByID(ctx, id)
	if err != nil {
		return nil, notFoundOr(err)
	}
	return &folder, nil
}

func (s *FolderService) Create(ctx context.Context, req FolderRequest) (*db.Folder, error) {
	params, err := s.parse(ctx, 0, req)
	if err != nil {
		return nil, err
	}
	folder, err := s.queries.CreateFolder(ctx, params)
	if err != nil {
		return nil, uniqueOr(err)
	}
	return &folder, nil
}

func (s *FolderService) Update(ctx context.Context, id int64, req FolderRequest) (*db.Folder, error) {
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
	return &folder, nil
}

func (s *FolderService) SetEnabled(ctx context.Context, id int64, enabled bool) (*db.Folder, error) {
	folder, err := s.queries.SetFolderEnabled(ctx, db.SetFolderEnabledParams{ID: id, Enabled: enabled})
	if err != nil {
		return nil, notFoundOr(err)
	}
	return &folder, nil
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
