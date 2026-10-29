package scanner

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"app/internal/db"
	"app/internal/logger"
	"app/internal/scans"
)

const (
	// pollInterval is how often the worker looks for queued scans it was not woken for: scans the
	// API queued while the worker lives in cmd/cron.
	pollInterval = 5 * time.Second
	// progressInterval is how often a running scan writes its counts, for a page that shows them.
	progressInterval = 2 * time.Second
	// finishTimeout bounds the write after a scan, which goes on after a stop has cancelled it.
	finishTimeout = 30 * time.Second
)

// Worker runs queued scans, at most settings.scan_workers at once. The database keeps a folder
// to one scan at a time. It lives in the process that runs the scheduler and starts and stops
// with it.
type Worker struct {
	queries *db.Queries
	scanner *Scanner
	logger  *logger.Logger
	// found is called after a scan that found new or changed images, to wake the optimizer.
	found func()

	running atomic.Int64
	wake    chan struct{}
	cancel  context.CancelFunc
	done    chan struct{}
}

func NewWorker(queries *db.Queries, tx *db.TxRunner, logger *logger.Logger, found func()) *Worker {
	return &Worker{
		queries: queries,
		scanner: New(queries, tx, logger),
		logger:  logger,
		found:   found,
		wake:    make(chan struct{}, 1),
	}
}

// Wake makes the worker look for queued scans now instead of at its next poll.
func (w *Worker) Wake() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// Start fails the scans a previous process left running, then runs queued scans until Stop.
func (w *Worker) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	n, err := scans.Recover(ctx, w.queries, time.Now())
	if err != nil {
		cancel()
		return err
	}
	if n > 0 {
		w.logger.Warn("Marked scans interrupted by a restart as failed", "scans", n)
	}
	w.cancel = cancel
	w.done = make(chan struct{})
	go w.loop(ctx)
	w.logger.Info("Scan worker started")
	return nil
}

// Stop cancels the scans in progress and waits for them to be recorded as failed.
func (w *Worker) Stop() {
	if w.cancel == nil {
		return
	}
	w.cancel()
	<-w.done
	w.logger.Info("Scan worker stopped")
}

func (w *Worker) loop(ctx context.Context) {
	defer close(w.done)
	var running sync.WaitGroup
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		w.dispatch(ctx, &running)
		select {
		case <-ctx.Done():
			running.Wait()
			return
		case <-w.wake:
		case <-ticker.C:
		}
	}
}

// dispatch starts queued scans while fewer than scan_workers run. The setting is read before each
// claim, so a change applies without a restart.
func (w *Worker) dispatch(ctx context.Context, running *sync.WaitGroup) {
	for ctx.Err() == nil {
		settings, err := w.queries.GetSettings(ctx)
		if err != nil {
			w.logger.Error("Could not read the scan worker limit", "error", err)
			return
		}
		if w.running.Load() >= settings.ScanWorkers {
			return
		}
		scan, ok, err := scans.ClaimNext(ctx, w.queries, time.Now())
		if err != nil {
			w.logger.Error("Could not claim a scan", "error", err)
			return
		}
		if !ok {
			return
		}
		w.running.Add(1)
		running.Add(1)
		go func() {
			defer running.Done()
			w.execute(ctx, scan)
			w.running.Add(-1)
			w.Wake()
		}()
	}
}

// Drain runs every queued scan, one after another, and returns when none is left. Tests use it to
// drive the worker without timing.
func (w *Worker) Drain(ctx context.Context) {
	for {
		scan, ok, err := scans.ClaimNext(ctx, w.queries, time.Now())
		if err != nil || !ok {
			return
		}
		w.execute(ctx, scan)
	}
}

// execute runs one claimed scan to its end and records how it ended. A scan stopped by a shutdown
// is recorded as failed, with the write done after the stop.
func (w *Worker) execute(ctx context.Context, scan db.Scan) {
	var written time.Time
	progress := func(c scans.Counts) {
		if time.Since(written) < progressInterval {
			return
		}
		written = time.Now()
		if err := scans.Progress(ctx, w.queries, scan.ID, c); err != nil && ctx.Err() == nil {
			w.logger.Error("Could not record a scan's progress", "scan", scan.ID, "error", err)
		}
	}
	counts, err := w.scanner.Scan(ctx, scan.FolderID, scans.Kind(scan.Kind), progress)
	if err != nil && ctx.Err() != nil {
		err = errors.New(scans.ErrInterruptedStop)
	}
	if counts.FilesNew+counts.FilesChanged > 0 && w.found != nil {
		w.found()
	}

	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), finishTimeout)
	defer cancel()
	if _, ferr := scans.Finish(finishCtx, w.queries, scan.ID, counts, err, time.Now()); ferr != nil {
		w.logger.Error("Could not record the end of a scan", "scan", scan.ID, "error", ferr)
	}
	attrs := []any{
		"scan", scan.ID, "folder", scan.FolderID, "kind", scan.Kind,
		"duration", time.Since(scan.StartedAt.Time).Round(time.Millisecond),
		"dirs_listed", counts.DirsListed, "files_new", counts.FilesNew,
		"files_changed", counts.FilesChanged, "files_gone", counts.FilesGone,
	}
	if err != nil {
		w.logger.Warn("Scan failed", append(attrs, "error", err)...)
		return
	}
	w.logger.Info("Scan finished", attrs...)
}
