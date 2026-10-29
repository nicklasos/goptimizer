package scans

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"app/internal/db"
	"app/internal/errs"
)

// listLimit is how many scans of a folder GET /folders/:id/scans returns, newest first.
const listLimit = 20

var (
	ErrFolderNotFound = errs.NewNotFoundError(errs.ErrKeyFoldersNotFound, "Folder not found")
	ErrFolderPaused   = errs.NewDomainError(errs.ErrKeyFoldersPaused, "The folder is paused", http.StatusConflict)
	ErrScanInProgress = errs.NewDomainError(errs.ErrKeyScansInProgress, "A scan of this folder is already queued or running", http.StatusConflict)
)

type ScanService struct {
	queries *db.Queries
	// wake tells the scan worker in this process that a scan was queued. Nil when the worker runs
	// in cmd/cron, which finds it at its next poll.
	wake func()
}

func NewScanService(queries *db.Queries, wake func()) *ScanService {
	return &ScanService{queries: queries, wake: wake}
}

// Start queues a scan of the folder by hand.
func (s *ScanService) Start(ctx context.Context, folderID int64, kind Kind) (*db.Scan, error) {
	folder, err := s.folder(ctx, folderID)
	if err != nil {
		return nil, err
	}
	if !folder.Enabled {
		return nil, ErrFolderPaused
	}
	scan, err := s.enqueue(ctx, folderID, kind, TriggerManual, time.Now())
	if err != nil {
		return nil, err
	}
	s.notify()
	return &scan, nil
}

// List returns the folder's latest scans, newest first.
func (s *ScanService) List(ctx context.Context, folderID int64) ([]db.Scan, error) {
	if _, err := s.folder(ctx, folderID); err != nil {
		return nil, err
	}
	scans, err := s.queries.ListScansOfFolder(ctx, db.ListScansOfFolderParams{FolderID: folderID, Limit: listLimit})
	if err != nil {
		return nil, errs.WrapDatabaseError(err)
	}
	return scans, nil
}

// EnqueueDue queues the scan each enabled folder is due for at now, and returns how many it
// queued. A folder with a scan already queued or running gets nothing, here or in another process.
func (s *ScanService) EnqueueDue(ctx context.Context, now time.Time) (int, error) {
	folders, err := s.queries.ListFolders(ctx)
	if err != nil {
		return 0, errs.WrapDatabaseError(err)
	}
	queued := 0
	var problems []error
	for _, folder := range folders {
		if !folder.Enabled {
			continue
		}
		lastFull, err := startedAt(s.queries.LastSucceededFullScanStart(ctx, folder.ID))
		if err != nil {
			problems = append(problems, fmt.Errorf("folder %d: last full scan: %w", folder.ID, err))
			continue
		}
		lastAny, err := startedAt(s.queries.LastFinishedScanStart(ctx, folder.ID))
		if err != nil {
			problems = append(problems, fmt.Errorf("folder %d: last scan: %w", folder.ID, err))
			continue
		}
		kind, ok := Due(folder, lastFull, lastAny, now)
		if !ok {
			continue
		}
		_, err = s.enqueue(ctx, folder.ID, kind, TriggerSchedule, now)
		if errors.Is(err, ErrScanInProgress) {
			continue
		}
		if err != nil {
			problems = append(problems, fmt.Errorf("folder %d: %w", folder.ID, err))
			continue
		}
		queued++
	}
	if queued > 0 {
		s.notify()
	}
	return queued, errors.Join(problems...)
}

func (s *ScanService) enqueue(ctx context.Context, folderID int64, kind Kind, trigger Trigger, at time.Time) (db.Scan, error) {
	scan, err := s.queries.EnqueueScan(ctx, db.EnqueueScanParams{
		FolderID: folderID,
		Kind:     string(kind),
		Trigger:  string(trigger),
		QueuedAt: at.UTC(),
	})
	if errs.IsUniqueViolation(err, "scans.folder_id") {
		return db.Scan{}, ErrScanInProgress
	}
	if err != nil {
		return db.Scan{}, errs.WrapDatabaseError(err)
	}
	return scan, nil
}

func (s *ScanService) folder(ctx context.Context, id int64) (db.Folder, error) {
	folder, err := s.queries.GetFolderByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return db.Folder{}, ErrFolderNotFound
	}
	if err != nil {
		return db.Folder{}, errs.WrapDatabaseError(err)
	}
	return folder, nil
}

func (s *ScanService) notify() {
	if s.wake != nil {
		s.wake()
	}
}

func startedAt(t sql.NullTime, err error) (*time.Time, error) {
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !t.Valid) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t.Time, nil
}
