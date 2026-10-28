package factory

import (
	"database/sql"
	"encoding/json"
	"time"

	"app/internal/db"
	"app/internal/runs"
)

type restoreAttrs struct {
	status    runs.Status
	target    any
	createdBy sql.NullInt64
}

type RestoreOption func(*restoreAttrs)

// WithRestoreStatus sets where the restore is. Running and finished restores get a start time,
// finished ones an end time, and a failed one an error.
func WithRestoreStatus(status runs.Status) RestoreOption {
	return func(a *restoreAttrs) { a.status = status }
}

// RestoreTarget is the stored target, such as a plans.SQLiteTarget. Without it the plan's own
// source config is the target.
func RestoreTarget(target any) RestoreOption { return func(a *restoreAttrs) { a.target = target } }

func RestoredBy(user *db.User) RestoreOption {
	return func(a *restoreAttrs) { a.createdBy = sql.NullInt64{Int64: user.ID, Valid: true} }
}

// Restore creates a restore of the archive: a queued one unless options say otherwise.
func (f *Factory) Restore(archive *db.Archive, opts ...RestoreOption) *db.Restore {
	f.t.Helper()

	attrs := restoreAttrs{status: runs.StatusQueued}
	for _, opt := range opts {
		opt(&attrs)
	}
	plan, err := f.queries.GetPlanByID(ctx, archive.PlanID)
	if err != nil {
		f.t.Fatalf("factory.Restore: %v", err)
	}
	target := plan.SourceConfig
	if attrs.target != nil {
		raw, err := json.Marshal(attrs.target)
		if err != nil {
			f.t.Fatalf("factory.Restore: %v", err)
		}
		target = string(raw)
	}
	now := sql.NullTime{Time: time.Now().UTC(), Valid: true}
	var started, finished sql.NullTime
	var errText sql.NullString
	if attrs.status != runs.StatusQueued {
		started = now
	}
	if attrs.status == runs.StatusSucceeded || attrs.status == runs.StatusFailed {
		finished = now
	}
	if attrs.status == runs.StatusFailed {
		errText = sql.NullString{String: "factory: failed", Valid: true}
	}

	r, err := f.queries.FactoryCreateRestore(ctx, db.FactoryCreateRestoreParams{
		ArchiveID:    sql.NullInt64{Int64: archive.ID, Valid: true},
		PlanID:       archive.PlanID,
		SourceKind:   plan.SourceKind,
		TargetConfig: target,
		Status:       string(attrs.status),
		StartedAt:    started,
		FinishedAt:   finished,
		Error:        errText,
		CreatedBy:    attrs.createdBy,
	})
	if err != nil {
		f.t.Fatalf("factory.Restore: %v", err)
	}
	return &r
}
