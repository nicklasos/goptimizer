package runs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"app/internal/db"
	"app/internal/plans"
	"app/internal/runlog"
	"app/internal/storages/driver"
)

// execute runs one claimed run to its end and records how it ended. A run stopped by a
// shutdown is recorded as failed, with the writes done after the stop.
func (w *Worker) execute(ctx context.Context, run db.Run) {
	log := runlog.New()
	stopFlushing := w.flushWhileRunning(ctx, log, func(ctx context.Context, text string) error {
		return w.queries.UpdateRunLog(ctx, db.UpdateRunLogParams{ID: run.ID, Log: text})
	})
	plan, size, err := w.perform(ctx, run, log)
	if err != nil && ctx.Err() != nil {
		err = errors.New(ErrInterruptedStop)
	}
	if err != nil {
		log.Printf("Failed: %v", err)
	} else {
		log.Printf("Succeeded in %s", w.now().Sub(run.StartedAt.Time).Round(time.Millisecond))
	}
	stopFlushing()

	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), finishTimeout)
	defer cancel()
	outcome := Outcome{Err: err, Size: size, Log: log.String()}
	if _, ferr := Finish(finishCtx, w.queries, run.ID, outcome, w.now()); ferr != nil {
		w.logger.Error("Could not record the end of a run", "run", run.ID, "error", ferr)
	}
	if err != nil && plan != nil {
		w.alert(finishCtx, backupFailed, plan, fmt.Sprintf("/plans/%d", plan.ID), err, log)
	}
}

// perform does the work of a run: dump the source once into every storage of the plan, record
// an archive per storage that stored it, then apply retention. It returns the plan (nil when
// it is gone) and the archive size when one was stored.
func (w *Worker) perform(ctx context.Context, run db.Run, log *runlog.Log) (*db.Plan, sql.NullInt64, error) {
	var noSize sql.NullInt64
	plan, err := w.queries.GetPlanByID(ctx, run.PlanID)
	if err != nil {
		return nil, noSize, fmt.Errorf("could not load the plan: %w", err)
	}
	targets, err := w.queries.ListStoragesForPlan(ctx, plan.ID)
	if err != nil {
		return &plan, noSize, fmt.Errorf("could not load the plan's storages: %w", err)
	}
	log.Printf("Run %d of %q (%s run, %s source) to %d storage(s)", run.ID, plan.Name, run.Trigger, plan.SourceKind, len(targets))
	if len(targets) == 0 {
		return &plan, noSize, errors.New("the plan has no storage to upload to")
	}

	started := run.StartedAt.Time.UTC()
	key := ObjectKey(plan.Name, run.ID, started, plans.ArchiveExtension(plans.SourceKind(plan.SourceKind)))
	var failures []string
	var open []opened
	for _, storage := range targets {
		d, err := w.storages.Driver(ctx, &storage)
		if err != nil {
			log.Printf("%s: cannot be opened: %v", storage.Name, err)
			failures = append(failures, fmt.Sprintf("%s: %v", storage.Name, err))
			continue
		}
		defer d.Close()
		open = append(open, opened{storage: storage, driver: d})
	}
	if len(open) == 0 {
		return &plan, noSize, storageFailures(failures, len(targets))
	}

	log.Printf("Writing %s", key)
	destinations := make([]Destination, len(open))
	for i, o := range open {
		destinations[i] = Destination{Name: o.storage.Name, Put: func(ctx context.Context, r io.Reader) error {
			return o.driver.Put(ctx, key, r)
		}}
	}
	fan := NewFanOut(ctx, destinations)
	dumpErr := w.plans.Dump(ctx, &plan, fan, log)
	storagesGaveUp := dumpErr != nil && fan.Live() == 0
	var uploads []Upload
	if dumpErr != nil {
		uploads = fan.Abort(dumpErr)
	} else {
		uploads = fan.Close()
	}

	var stored []opened
	for i, upload := range uploads {
		o := open[i]
		if upload.Err != nil {
			if dumpErr == nil || storagesGaveUp {
				log.Printf("%s: upload failed: %v", o.storage.Name, upload.Err)
				failures = append(failures, fmt.Sprintf("%s: %v", o.storage.Name, upload.Err))
			}
			continue
		}
		_, err := w.queries.CreateArchive(ctx, db.CreateArchiveParams{
			RunID:     run.ID,
			PlanID:    plan.ID,
			StorageID: o.storage.ID,
			ObjectKey: key,
			SizeBytes: fan.Size(),
			Sha256:    fan.SHA256(),
			CreatedAt: started,
		})
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: stored, but the archive could not be recorded: %v", o.storage.Name, err))
			continue
		}
		log.Printf("%s: stored %s", o.storage.Name, formatBytes(fan.Size()))
		stored = append(stored, o)
	}

	switch {
	case dumpErr != nil && !storagesGaveUp:
		return &plan, noSize, fmt.Errorf("the dump failed: %w", dumpErr)
	case dumpErr != nil:
		return &plan, noSize, storageFailures(failures, len(targets))
	}
	log.Printf("Archive: %s, sha256 %s", formatBytes(fan.Size()), fan.SHA256())
	size := sql.NullInt64{Int64: fan.Size(), Valid: true}
	if len(failures) > 0 {
		return &plan, size, storageFailures(failures, len(targets))
	}

	policy := Retention{MaxAgeDays: plan.MaxAgeDays, KeepWeekly: plan.KeepWeekly, KeepMonthly: plan.KeepMonthly}
	for _, o := range stored {
		w.prune(ctx, &plan, o, policy, log)
	}
	return &plan, size, nil
}

type opened struct {
	storage db.Storage
	driver  driver.Driver
}

func storageFailures(failures []string, total int) error {
	if len(failures) == total {
		return fmt.Errorf("every storage failed: %s", strings.Join(failures, "; "))
	}
	return fmt.Errorf("%d of %d storages failed: %s", len(failures), total, strings.Join(failures, "; "))
}

// prune applies the plan's retention to its archives on one storage. An object that cannot be
// deleted keeps its row, so the next run's prune tries it again.
func (w *Worker) prune(ctx context.Context, plan *db.Plan, o opened, policy Retention, log *runlog.Log) {
	if policy.MaxAgeDays <= 0 {
		log.Printf("%s: retention keeps every archive", o.storage.Name)
		return
	}
	rows, err := w.queries.ListLiveArchivesOfPlanStorage(ctx, db.ListLiveArchivesOfPlanStorageParams{PlanID: plan.ID, StorageID: o.storage.ID})
	if err != nil {
		log.Warnf("%s: retention skipped: %v", o.storage.Name, err)
		return
	}
	archives := make([]Archive, len(rows))
	keys := make(map[int64]string, len(rows))
	for i, row := range rows {
		archives[i] = Archive{ID: row.ID, CreatedAt: row.CreatedAt}
		keys[row.ID] = row.ObjectKey
	}
	remove := Prune(archives, policy, w.now())
	deleted := 0
	for _, id := range remove {
		if err := o.driver.Delete(ctx, keys[id]); err != nil {
			log.Warnf("%s: could not delete %s, the next run tries again: %v", o.storage.Name, keys[id], err)
			continue
		}
		err := w.queries.MarkArchiveDeleted(ctx, db.MarkArchiveDeletedParams{ID: id, DeletedAt: sql.NullTime{Time: w.now().UTC(), Valid: true}})
		if err != nil {
			log.Warnf("%s: deleted %s but could not record it: %v", o.storage.Name, keys[id], err)
			continue
		}
		deleted++
	}
	log.Printf("%s: retention kept %d archive(s), deleted %d", o.storage.Name, len(rows)-deleted, deleted)
}

// flushWhileRunning writes the log of a job with write every few seconds until the returned
// func is called, so the log can be followed while the job goes on.
func (w *Worker) flushWhileRunning(ctx context.Context, log *runlog.Log, write func(ctx context.Context, text string) error) (stop func()) {
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		ticker := time.NewTicker(logFlushInterval)
		defer ticker.Stop()
		var written uint64
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				text, version := log.Snapshot()
				if version == written {
					continue
				}
				if err := write(ctx, text); err != nil {
					w.logger.Warn("Could not write a job's log", "error", err)
					continue
				}
				written = version
			}
		}
	}()
	return func() {
		close(done)
		<-finished
	}
}

// ObjectKey is where a run's archive goes in each storage: the plan's folder, and a name with
// the plan, the start time in UTC and the run, so two runs never write the same object.
func ObjectKey(planName string, runID int64, started time.Time, extension string) string {
	slug := plans.Slug(planName)
	return fmt.Sprintf("%s%s-%s-%d.%s", plans.Folder(planName), slug, started.UTC().Format("20060102T150405Z"), runID, extension)
}

func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB (%d bytes)", float64(n)/float64(div), "KMGTPE"[exp], n)
}
