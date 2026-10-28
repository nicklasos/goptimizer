package runs

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"app/config"
	"app/internal/db"
	"app/internal/logger"
	"app/internal/mail"
	"app/internal/plans"
	"app/internal/secretbox"
	"app/internal/storages"
	"app/internal/tools"
)

const (
	// pollInterval is how often the worker looks for queued jobs it was not woken for: jobs the
	// API queued while the worker lives in cmd/cron.
	pollInterval = 5 * time.Second
	// logFlushInterval is how often a running run's log is written, for a page that shows it live.
	logFlushInterval = 2 * time.Second
	// finishTimeout bounds the writes after a run, which go on after a stop has cancelled it.
	finishTimeout = 30 * time.Second
)

// Deps is what a worker needs. The API and cmd/cron fill it from their own setup.
type Deps struct {
	Config  *config.Config
	Queries *db.Queries
	Tx      *db.TxRunner
	Secrets *secretbox.Box
	Tools   tools.Finder
	Mail    mail.Sender
	Logger  *logger.Logger
}

// Worker runs queued jobs, runs and restores, at most Config.RunnerConcurrency at once and
// never two of one plan. It lives in the process that runs the scheduler and starts and stops
// with it.
type Worker struct {
	queries     *db.Queries
	plans       *plans.PlanService
	storages    *storages.StorageService
	archives    *ArchiveService
	mail        mail.Sender
	logger      *logger.Logger
	concurrency int
	appName     string
	frontendURL string
	workDir     string
	restoreJobs int
	now         func() time.Time

	wake   chan struct{}
	cancel context.CancelFunc
	done   chan struct{}
}

func NewWorker(d Deps) *Worker {
	storageService := storages.NewStorageService(d.Queries, d.Secrets)
	return &Worker{
		queries:     d.Queries,
		plans:       plans.NewPlanService(d.Queries, d.Tx, d.Secrets, d.Tools),
		storages:    storageService,
		archives:    NewArchiveService(d.Queries, storageService, d.Secrets),
		mail:        d.Mail,
		logger:      d.Logger,
		concurrency: max(d.Config.RunnerConcurrency, 1),
		appName:     d.Config.AppName,
		frontendURL: d.Config.FrontendURL,
		workDir:     d.Config.WorkDir,
		restoreJobs: max(d.Config.RestoreJobs, 1),
		now:         time.Now,
		wake:        make(chan struct{}, 1),
	}
}

// Wake makes the worker look for queued jobs now instead of at its next poll.
func (w *Worker) Wake() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// Start fails the jobs a previous process left running, makes the work directory, then runs
// queued jobs until Stop.
func (w *Worker) Start() error {
	if err := os.MkdirAll(w.workDir, 0o700); err != nil {
		return fmt.Errorf("could not create WORK_DIR: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if n, err := Recover(ctx, w.queries, w.now()); err != nil {
		cancel()
		return err
	} else if n > 0 {
		w.logger.Warn("Marked jobs interrupted by a restart as failed", "jobs", n)
	}
	w.cancel = cancel
	w.done = make(chan struct{})
	go w.loop(ctx)
	w.logger.Info("Runner started", "concurrency", w.concurrency, "work_dir", w.workDir)
	return nil
}

// Stop cancels the jobs in progress, which kills their programs and fails them, and waits
// for them to be recorded.
func (w *Worker) Stop() {
	if w.cancel == nil {
		return
	}
	w.cancel()
	<-w.done
	w.logger.Info("Runner stopped")
}

func (w *Worker) loop(ctx context.Context) {
	defer close(w.done)
	slots := make(chan struct{}, w.concurrency)
	var running sync.WaitGroup
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		w.dispatch(ctx, slots, &running)
		select {
		case <-ctx.Done():
			running.Wait()
			return
		case <-w.wake:
		case <-ticker.C:
		}
	}
}

// dispatch starts queued jobs while there are free slots.
func (w *Worker) dispatch(ctx context.Context, slots chan struct{}, running *sync.WaitGroup) {
	for {
		select {
		case slots <- struct{}{}:
		default:
			return
		}
		job, ok := w.claimNext(ctx)
		if !ok {
			<-slots
			return
		}
		running.Add(1)
		go func() {
			defer running.Done()
			job(ctx)
			<-slots
			w.Wake()
		}()
	}
}

// Drain runs every queued job that can be claimed, one after another, and returns when none
// is left. Tests use it to drive the worker without timing.
func (w *Worker) Drain(ctx context.Context) {
	for {
		job, ok := w.claimNext(ctx)
		if !ok {
			return
		}
		job(ctx)
	}
}

// job is a claimed run or restore: calling it executes it to its end.
type job func(ctx context.Context)

// claimers claim a job of each kind the queue lists, and return it ready to execute. False
// when another worker took it first or its plan became busy.
var claimers = map[string]func(w *Worker, ctx context.Context, id int64) (job, bool, error){
	"run":     (*Worker).claimRun,
	"restore": (*Worker).claimRestore,
}

// claimNext claims the oldest queued job whose plan is not busy, if there is one.
func (w *Worker) claimNext(ctx context.Context) (job, bool) {
	if ctx.Err() != nil {
		return nil, false
	}
	candidates, err := w.queries.ListClaimableJobs(ctx, 10)
	if err != nil {
		w.logger.Error("Could not list queued jobs", "error", err)
		return nil, false
	}
	for _, candidate := range candidates {
		claimed, ok, err := claimers[candidate.Kind](w, ctx, candidate.ID)
		if err != nil {
			w.logger.Error("Could not claim a job", "kind", candidate.Kind, "id", candidate.ID, "error", err)
			return nil, false
		}
		if ok {
			return claimed, true
		}
	}
	return nil, false
}

func (w *Worker) claimRun(ctx context.Context, id int64) (job, bool, error) {
	ok, err := Claim(ctx, w.queries, id, w.now())
	if err != nil || !ok {
		return nil, false, err
	}
	run, err := w.queries.GetRunByID(ctx, id)
	if err != nil {
		return nil, false, err
	}
	return func(ctx context.Context) { w.execute(ctx, run) }, true, nil
}

func (w *Worker) claimRestore(ctx context.Context, id int64) (job, bool, error) {
	ok, err := ClaimRestore(ctx, w.queries, id, w.now())
	if err != nil || !ok {
		return nil, false, err
	}
	restore, err := w.queries.GetRestoreJob(ctx, id)
	if err != nil {
		return nil, false, err
	}
	return func(ctx context.Context) { w.executeRestore(ctx, restore) }, true, nil
}
