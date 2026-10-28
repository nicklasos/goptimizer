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
	"app/internal/runs"
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
	// Worker runs the queued backups. It starts and stops with the scheduler, so backups run in
	// whichever process runs the schedule.
	Worker *runs.Worker
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
	if err := s.registerEnqueueRunsJob(); err != nil {
		return fmt.Errorf("failed to register enqueue runs job: %w", err)
	}

	return nil
}

// Start starts the backup worker, which first fails the runs a previous process left running,
// then begins executing all registered cron jobs
func (s *Scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.deps.Worker.Start(); err != nil {
		return fmt.Errorf("failed to start the backup worker: %w", err)
	}
	s.cron.Start()
	s.deps.Logger.Info("Scheduler started", "jobs", len(s.cron.Entries()))
	return nil
}

// Stop stops scheduling new runs and waits for the jobs already running to finish. Backups in
// progress are stopped and recorded as failed.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	<-s.cron.Stop().Done()
	s.deps.Worker.Stop()
	s.deps.Logger.Info("Scheduler stopped")
}

func (s *Scheduler) registerEnqueueRunsJob() error {
	job := jobs.NewEnqueueRunsJob(runs.NewRunService(s.deps.Queries, s.deps.Tx, s.deps.Worker.Wake), s.deps.Logger)

	// At the start of every minute, so a 03:00 slot is queued at 03:00 and not up to a minute later.
	_, err := s.cron.AddFunc("* * * * *", func() {
		if err := job.Execute(context.Background()); err != nil {
			s.deps.Logger.Error("Enqueue runs job failed", "error", err)
		}
	})
	if err != nil {
		return fmt.Errorf("failed to add enqueue runs job: %w", err)
	}

	s.deps.Logger.Info("Registered enqueue-runs job (every minute)")
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
