package factory

import (
	"database/sql"
	"time"

	"app/internal/db"
	"app/internal/scans"
)

type scanAttrs struct {
	kind      scans.Kind
	status    scans.Status
	startedAt time.Time
	err       string
}

type ScanOption func(*scanAttrs)

func WithScanKind(kind scans.Kind) ScanOption { return func(a *scanAttrs) { a.kind = kind } }

// WithScanStatus sets the status. A scan past queued gets a start time, and a finished one an end.
func WithScanStatus(status scans.Status) ScanOption {
	return func(a *scanAttrs) { a.status = status }
}

// StartedAt sets when a running or finished scan started.
func StartedAt(at time.Time) ScanOption { return func(a *scanAttrs) { a.startedAt = at } }

func WithScanError(message string) ScanOption { return func(a *scanAttrs) { a.err = message } }

// Scan creates a queued quick scan of the folder, triggered by the schedule.
func (f *Factory) Scan(folder *db.Folder, opts ...ScanOption) *db.Scan {
	f.t.Helper()

	attrs := scanAttrs{kind: scans.KindQuick, status: scans.StatusQueued, startedAt: time.Now()}
	for _, opt := range opts {
		opt(&attrs)
	}

	params := db.FactoryCreateScanParams{
		FolderID: folder.ID,
		Kind:     string(attrs.kind),
		Trigger:  string(scans.TriggerSchedule),
		Status:   string(attrs.status),
		QueuedAt: attrs.startedAt.UTC(),
		Error:    sql.NullString{String: attrs.err, Valid: attrs.err != ""},
	}
	if attrs.status != scans.StatusQueued {
		params.StartedAt = sql.NullTime{Time: attrs.startedAt.UTC(), Valid: true}
	}
	if attrs.status == scans.StatusSucceeded || attrs.status == scans.StatusFailed {
		params.FinishedAt = sql.NullTime{Time: attrs.startedAt.UTC(), Valid: true}
	}
	scan, err := f.queries.FactoryCreateScan(ctx, params)
	if err != nil {
		f.t.Fatalf("factory.Scan: %v", err)
	}
	return &scan
}
