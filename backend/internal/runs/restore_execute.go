package runs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"app/internal/db"
	"app/internal/plans"
	"app/internal/runlog"
)

// executeRestore runs one claimed restore to its end and records how it ended, like execute
// does for a run.
func (w *Worker) executeRestore(ctx context.Context, restore db.Restore) {
	log := runlog.New()
	stopFlushing := w.flushWhileRunning(ctx, log, func(ctx context.Context, text string) error {
		return w.queries.UpdateRestoreLog(ctx, db.UpdateRestoreLogParams{ID: restore.ID, Log: text})
	})
	plan, err := w.performRestore(ctx, restore, log)
	if err != nil && ctx.Err() != nil {
		err = errors.New(ErrInterruptedStop)
	}
	if err != nil {
		log.Printf("Failed: %v", err)
	} else {
		log.Printf("Succeeded in %s", w.now().Sub(restore.StartedAt.Time).Round(time.Millisecond))
	}
	stopFlushing()

	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), finishTimeout)
	defer cancel()
	if _, ferr := FinishRestore(finishCtx, w.queries, restore.ID, err, log.String(), w.now()); ferr != nil {
		w.logger.Error("Could not record the end of a restore", "restore", restore.ID, "error", ferr)
	}
	if err != nil && plan != nil {
		w.alert(finishCtx, restoreFailed, plan, fmt.Sprintf("/plans/%d?tab=restores&restore=%d", plan.ID, restore.ID), err, log)
	}
}

// performRestore restores the archive into the target in a work directory of its own, which
// it removes after. It returns the plan, nil when it is gone.
func (w *Worker) performRestore(ctx context.Context, restore db.Restore, log *runlog.Log) (*db.Plan, error) {
	plan, err := w.queries.GetPlanByID(ctx, restore.PlanID)
	if err != nil {
		return nil, fmt.Errorf("could not load the plan: %w", err)
	}
	if !restore.ArchiveID.Valid {
		return &plan, errors.New("the archive was deleted")
	}
	archive, err := liveArchive(ctx, w.queries, restore.ArchiveID.Int64)
	if err != nil {
		return &plan, fmt.Errorf("the archive is gone: %w", err)
	}
	log.Printf("Restore %d of %q (%s source) from %s, made %s", restore.ID, plan.Name, plan.SourceKind, archive.ObjectKey, archive.CreatedAt.UTC().Format(time.RFC3339))

	dir, err := os.MkdirTemp(w.workDir, fmt.Sprintf("restore-%d-", restore.ID))
	if err != nil {
		return &plan, fmt.Errorf("could not make a work directory in %s: %w", w.workDir, err)
	}
	defer os.RemoveAll(dir)

	source := plans.Archive{
		Name:   path.Base(archive.ObjectKey),
		Size:   archive.SizeBytes,
		SHA256: archive.Sha256,
		Open: func(ctx context.Context) (io.ReadCloser, error) {
			return w.archives.Open(ctx, archive.ID)
		},
	}
	return &plan, w.plans.Restore(ctx, &plan, &restore, source, dir, w.restoreJobs, log)
}
