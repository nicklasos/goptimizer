package factory

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"time"

	"app/internal/db"
	"app/internal/runs"
)

type runAttrs struct {
	trigger      runs.Trigger
	scheduledFor sql.NullTime
	status       runs.Status
	startedAt    sql.NullTime
	finishedAt   sql.NullTime
	sizeBytes    sql.NullInt64
	errText      sql.NullString
	took         time.Duration
}

type RunOption func(*runAttrs)

// WithRunStatus sets where the run is. Running and finished runs get a start time, finished
// ones an end time, and a failed one an error.
func WithRunStatus(status runs.Status) RunOption { return func(a *runAttrs) { a.status = status } }

// ScheduledFor makes it the scheduled run of a slot.
func ScheduledFor(slot time.Time) RunOption {
	return func(a *runAttrs) {
		a.trigger, a.scheduledFor = runs.TriggerSchedule, sql.NullTime{Time: slot.UTC(), Valid: true}
	}
}

// StartedAt backdates the run's start; a finished run ends a minute later.
func StartedAt(at time.Time) RunOption {
	return func(a *runAttrs) {
		a.startedAt = sql.NullTime{Time: at.UTC(), Valid: true}
		a.finishedAt = sql.NullTime{Time: at.UTC().Add(time.Minute), Valid: true}
	}
}

// FinishedAfter makes a finished run end this long after it started, instead of a minute.
func FinishedAfter(d time.Duration) RunOption { return func(a *runAttrs) { a.took = d } }

// RunError is why a failed run failed.
func RunError(text string) RunOption {
	return func(a *runAttrs) { a.errText = sql.NullString{String: text, Valid: true} }
}

// RunSize is the size of what the run stored.
func RunSize(bytes int64) RunOption {
	return func(a *runAttrs) { a.sizeBytes = sql.NullInt64{Int64: bytes, Valid: true} }
}

// Run creates a run of the plan: a queued manual one unless options say otherwise.
func (f *Factory) Run(plan *db.Plan, opts ...RunOption) *db.Run {
	f.t.Helper()

	attrs := runAttrs{trigger: runs.TriggerManual, status: runs.StatusQueued}
	for _, opt := range opts {
		opt(&attrs)
	}
	now := sql.NullTime{Time: time.Now().UTC(), Valid: true}
	if attrs.status != runs.StatusQueued && !attrs.startedAt.Valid {
		attrs.startedAt = now
	}
	finished := attrs.status == runs.StatusSucceeded || attrs.status == runs.StatusFailed
	if finished && !attrs.finishedAt.Valid {
		attrs.finishedAt = now
	}
	if finished && attrs.took > 0 {
		attrs.finishedAt = sql.NullTime{Time: attrs.startedAt.Time.Add(attrs.took), Valid: true}
	}
	if !finished {
		attrs.finishedAt = sql.NullTime{}
	}
	if attrs.status == runs.StatusFailed && !attrs.errText.Valid {
		attrs.errText = sql.NullString{String: "factory: failed", Valid: true}
	}

	run, err := f.queries.FactoryCreateRun(ctx, db.FactoryCreateRunParams{
		PlanID:       plan.ID,
		Trigger:      string(attrs.trigger),
		ScheduledFor: attrs.scheduledFor,
		Status:       string(attrs.status),
		StartedAt:    attrs.startedAt,
		FinishedAt:   attrs.finishedAt,
		SizeBytes:    attrs.sizeBytes,
		Error:        attrs.errText,
	})
	if err != nil {
		f.t.Fatalf("factory.Run: %v", err)
	}
	return &run
}

type archiveAttrs struct {
	key       string
	content   []byte
	size      int64
	createdAt time.Time
	deletedAt sql.NullTime
}

type ArchiveOption func(*archiveAttrs)

// ArchiveKey is the object's key in the storage. The factory records the archive; it does not
// write the object.
func ArchiveKey(key string) ArchiveOption { return func(a *archiveAttrs) { a.key = key } }

// ArchiveContent sets size_bytes and sha256 to those of content.
func ArchiveContent(content []byte) ArchiveOption {
	return func(a *archiveAttrs) { a.content = content }
}

// ArchiveSize records this size instead of the content's, for an archive too large to hold.
func ArchiveSize(bytes int64) ArchiveOption { return func(a *archiveAttrs) { a.size = bytes } }

func ArchiveCreatedAt(at time.Time) ArchiveOption {
	return func(a *archiveAttrs) { a.createdAt = at.UTC() }
}

// ArchiveDeleted makes it an archive that retention removed.
func ArchiveDeleted() ArchiveOption {
	return func(a *archiveAttrs) { a.deletedAt = sql.NullTime{Time: time.Now().UTC(), Valid: true} }
}

// Archive records an archive of the run on the storage, created now unless an option says otherwise.
func (f *Factory) Archive(run *db.Run, storage *db.Storage, opts ...ArchiveOption) *db.Archive {
	f.t.Helper()

	attrs := archiveAttrs{key: "factory/archive-" + unique() + ".tar.gz", createdAt: time.Now().UTC()}
	for _, opt := range opts {
		opt(&attrs)
	}
	sum := sha256.Sum256(attrs.content)
	if attrs.size == 0 {
		attrs.size = int64(len(attrs.content))
	}

	archive, err := f.queries.FactoryCreateArchive(ctx, db.FactoryCreateArchiveParams{
		RunID:     run.ID,
		PlanID:    run.PlanID,
		StorageID: storage.ID,
		ObjectKey: attrs.key,
		SizeBytes: attrs.size,
		Sha256:    hex.EncodeToString(sum[:]),
		CreatedAt: attrs.createdAt,
		DeletedAt: attrs.deletedAt,
	})
	if err != nil {
		f.t.Fatalf("factory.Archive: %v", err)
	}
	return &archive
}
