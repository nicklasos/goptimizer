package scheduler

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"app/config"
	"app/internal/db"
	"app/internal/logger"
	"app/internal/optimizer"
	"app/internal/scanner"
	"app/internal/scans"
	"app/internal/scheduler/jobs"

	"github.com/robfig/cron/v3"
)

// Dependencies contains all services that might be needed by the scheduler
type Dependencies struct {
	Config  *config.Config
	DB      db.DBTX
	Queries *db.Queries
	Tx      *db.TxRunner
	Logger  *logger.Logger
	// ScanWorker runs the queued scans and Optimizer the pending images. They start and stop with
	// the scheduler, so scans and optimizations run in whichever process runs the schedule.
	ScanWorker *scanner.Worker
	Optimizer  *optimizer.Pool
}

// Job represents a cron job that can be executed
type Job interface {
	Execute(ctx context.Context) error
	Name() string
	Description() string
}

// Scheduler manages all cron jobs for the application
type Scheduler struct {
	cron *cron.Cron
	deps *Dependencies
	mu   sync.RWMutex
}

// NewScheduler creates a new scheduler instance with all dependencies
func NewScheduler(deps *Dependencies) *Scheduler {
	cronLogger := cron.VerbosePrintfLogger(log.New(os.Stdout, "scheduler: ", log.LstdFlags))
	c := cron.New(cron.WithLogger(cronLogger))

	return &Scheduler{
		cron: c,
		deps: deps,
	}
}

// RegisterJobs registers all application cron jobs
func (s *Scheduler) RegisterJobs() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.registerCleanupRefreshTokensJob(); err != nil {
		return fmt.Errorf("failed to register cleanup refresh tokens job: %w", err)
	}
	if err := s.registerEnqueueScansJob(); err != nil {
		return fmt.Errorf("failed to register enqueue scans job: %w", err)
	}

	return nil
}

// Start starts the scan worker, which first fails the scans a previous process left running, and
// the optimizer, which first makes the images left optimizing pending again, then begins executing
// all registered cron jobs
func (s *Scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.deps.ScanWorker.Start(); err != nil {
		return fmt.Errorf("failed to start the scan worker: %w", err)
	}
	if err := s.deps.Optimizer.Start(); err != nil {
		s.deps.ScanWorker.Stop()
		return fmt.Errorf("failed to start the optimizer: %w", err)
	}
	s.cron.Start()
	s.deps.Logger.Info("Scheduler started", "jobs", len(s.cron.Entries()))
	return nil
}

// Stop stops scheduling new runs and waits for the jobs already running to finish. Scans in
// progress are stopped and recorded as failed; images in progress go back to pending, unless their
// swap had begun, which finishes.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	<-s.cron.Stop().Done()
	s.deps.ScanWorker.Stop()
	s.deps.Optimizer.Stop()
	s.deps.Logger.Info("Scheduler stopped")
}

func (s *Scheduler) registerEnqueueScansJob() error {
	job := jobs.NewEnqueueScansJob(scans.NewScanService(s.deps.Queries, s.deps.ScanWorker.Wake), s.deps.Logger)

	_, err := s.cron.AddFunc("* * * * *", func() {
		if err := job.Execute(context.Background()); err != nil {
			s.deps.Logger.Error("Enqueue scans job failed", "error", err)
		}
	})
	if err != nil {
		return fmt.Errorf("failed to add enqueue scans job: %w", err)
	}

	s.deps.Logger.Info("Registered enqueue-scans job (every minute)")
	return nil
}

func (s *Scheduler) registerCleanupRefreshTokensJob() error {
	job := jobs.NewCleanupRefreshTokensJob(s.deps.Config, s.deps.Queries, s.deps.Logger)

	_, err := s.cron.AddFunc("@daily", func() {
		if err := job.Execute(context.Background()); err != nil {
			s.deps.Logger.Error("Cleanup refresh tokens job failed", "error", err)
		}
	})
	if err != nil {
		return fmt.Errorf("failed to add cleanup refresh tokens job: %w", err)
	}

	s.deps.Logger.Info("Registered cleanup-refresh-tokens job (daily)")
	return nil
}
