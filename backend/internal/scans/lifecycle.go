// Package scans keeps track of the scans that fill each folder's image index: queued on a schedule
// or by hand, claimed by the scan worker (internal/scanner), and finished with what they found.
package scans

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"app/internal/db"
)

// Kind is how much of the folder a scan lists. A quick scan lists the directories whose mtime
// changed; a full scan lists every directory and so also sees a file rewritten in place.
type Kind string

const (
	KindQuick Kind = "quick"
	KindFull  Kind = "full"
)

type Trigger string

const (
	TriggerSchedule Trigger = "schedule"
	TriggerManual   Trigger = "manual"
)

// Status is where a scan is in its life: queued, then running, then succeeded or failed. Each step
// is one conditional UPDATE on the current status, so a scan moves forward once however many
// workers or processes try.
type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

const (
	ErrInterruptedRestart = "interrupted: the service restarted"
	ErrInterruptedStop    = "interrupted: the service stopped"
)

// Counts is what a scan has done so far. New includes files that came back after being gone.
type Counts struct {
	DirsSeen     int64
	DirsListed   int64
	FilesSeen    int64
	FilesNew     int64
	FilesChanged int64
	FilesGone    int64
}

// ClaimNext moves the oldest queued scan to running and returns it. False when none is queued.
func ClaimNext(ctx context.Context, q *db.Queries, at time.Time) (db.Scan, bool, error) {
	scan, err := q.ClaimNextScan(ctx, sql.NullTime{Time: at.UTC(), Valid: true})
	if errors.Is(err, sql.ErrNoRows) {
		return db.Scan{}, false, nil
	}
	return scan, err == nil, err
}

// Progress records the counts of a running scan, for a page that shows it live.
func Progress(ctx context.Context, q *db.Queries, id int64, c Counts) error {
	return q.UpdateScanProgress(ctx, db.UpdateScanProgressParams{
		ID:           id,
		DirsSeen:     c.DirsSeen,
		DirsListed:   c.DirsListed,
		FilesSeen:    c.FilesSeen,
		FilesNew:     c.FilesNew,
		FilesChanged: c.FilesChanged,
		FilesGone:    c.FilesGone,
	})
}

const Keep = 100

// Finish moves a running scan to succeeded, or to failed when err is not nil, and deletes what
// Keep no longer covers. False when it was not running, such as a scan that recovery already failed.
func Finish(ctx context.Context, q *db.Queries, id int64, c Counts, err error, at time.Time) (bool, error) {
	params := db.FinishScanParams{
		ID:           id,
		Status:       string(StatusSucceeded),
		FinishedAt:   sql.NullTime{Time: at.UTC(), Valid: true},
		DirsSeen:     c.DirsSeen,
		DirsListed:   c.DirsListed,
		FilesSeen:    c.FilesSeen,
		FilesNew:     c.FilesNew,
		FilesChanged: c.FilesChanged,
		FilesGone:    c.FilesGone,
	}
	if err != nil {
		params.Status = string(StatusFailed)
		params.Error = sql.NullString{String: err.Error(), Valid: true}
	}
	folderID, ferr := q.FinishScan(ctx, params)
	if errors.Is(ferr, sql.ErrNoRows) {
		return false, nil
	}
	if ferr != nil {
		return false, ferr
	}
	if _, err := q.PruneScans(ctx, db.PruneScansParams{FolderID: folderID, Keep: Keep}); err != nil {
		return true, fmt.Errorf("delete the folder's old scans: %w", err)
	}
	return true, nil
}

// Recover fails every scan left running by a process that ended without finishing it. Only one
// process runs the worker, so at its start no scan can still be running anywhere.
func Recover(ctx context.Context, q *db.Queries, at time.Time) (int64, error) {
	return q.FailRunningScans(ctx, db.FailRunningScansParams{
		FinishedAt: sql.NullTime{Time: at.UTC(), Valid: true},
		Error:      sql.NullString{String: ErrInterruptedRestart, Valid: true},
	})
}
