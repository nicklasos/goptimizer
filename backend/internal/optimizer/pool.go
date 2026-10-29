package optimizer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"app/internal/db"
	"app/internal/folders"
	"app/internal/images"
	"app/internal/logger"
	"app/internal/settings"
	"app/internal/tools"
)

const (
	// pollInterval is how often the pool looks for work it was not woken for: images that came of
	// age, failed ones due again, scans by another process.
	pollInterval = 5 * time.Second
	// requeueInterval is how often failed images due for a retry go back to pending.
	requeueInterval = 30 * time.Second
	// recordTimeout bounds the write after an image, which goes on after a stop.
	recordTimeout = 30 * time.Second
)

// Pool runs the optimizer on pending images, at most settings.workers at once, taking them fairly
// from every enabled folder (images.Claim). It lives in the process that runs the scheduler and
// starts and stops with it.
type Pool struct {
	queries   *db.Queries
	tx        *db.TxRunner
	logger    *logger.Logger
	reporter  *tools.Reporter
	workDir   string
	Optimizer *Optimizer

	running atomic.Int64
	wake    chan struct{}
	cancel  context.CancelFunc
	done    chan struct{}

	// claimMu serializes claims, and guards the folder claimed from last and the last requeue.
	claimMu    sync.Mutex
	after      int64
	requeuedAt time.Time
}

func NewPool(queries *db.Queries, tx *db.TxRunner, logger *logger.Logger, reporter *tools.Reporter, workDir string) *Pool {
	return &Pool{
		queries:   queries,
		tx:        tx,
		logger:    logger,
		reporter:  reporter,
		workDir:   workDir,
		Optimizer: New(queries, workDir),
		wake:      make(chan struct{}, 1),
	}
}

// Wake makes the pool look for pending images now instead of at its next poll.
func (p *Pool) Wake() {
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

// Start prepares the work dir, makes the images a previous process left optimizing pending again,
// and runs pending images until Stop.
func (p *Pool) Start() error {
	if err := PrepareWorkDir(p.workDir); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	n, err := images.Recover(ctx, p.queries, time.Now())
	if err != nil {
		cancel()
		return err
	}
	if n > 0 {
		p.logger.Warn("Made images interrupted by a restart pending again", "images", n)
	}
	p.cancel = cancel
	p.done = make(chan struct{})
	go p.loop(ctx)
	p.logger.Info("Optimizer started", "work_dir", p.workDir)
	return nil
}

// Stop stops the tools in progress, lets a swap already under way finish, and waits for every
// image to be recorded.
func (p *Pool) Stop() {
	if p.cancel == nil {
		return
	}
	p.cancel()
	<-p.done
	p.logger.Info("Optimizer stopped")
}

func (p *Pool) loop(ctx context.Context) {
	defer close(p.done)
	var running sync.WaitGroup
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		p.dispatch(ctx, &running)
		select {
		case <-ctx.Done():
			running.Wait()
			return
		case <-p.wake:
		case <-ticker.C:
		}
	}
}

// dispatch starts pending images while fewer than settings.workers run. The settings are read
// before each claim, so a change applies without a restart.
func (p *Pool) dispatch(ctx context.Context, running *sync.WaitGroup) {
	for ctx.Err() == nil {
		setting, err := p.queries.GetSettings(ctx)
		if err != nil {
			p.logger.Error("Could not read the optimizer's settings", "error", err)
			return
		}
		if p.running.Load() >= settings.EffectiveWorkers(setting.Workers) {
			return
		}
		job, ok, err := p.claim(ctx, setting.Nice)
		if err != nil {
			if ctx.Err() == nil {
				p.logger.Error("Could not claim an image", "error", err)
			}
			return
		}
		if !ok {
			return
		}
		p.running.Add(1)
		running.Add(1)
		go func() {
			defer running.Done()
			p.process(ctx, job)
			p.running.Add(-1)
			p.Wake()
		}()
	}
}

// Drain optimizes every claimable image, one after another, and returns how many it handled.
// Tests use it to drive the pool without timing.
func (p *Pool) Drain(ctx context.Context) int {
	handled := 0
	for {
		setting, err := p.queries.GetSettings(ctx)
		if err != nil {
			return handled
		}
		job, ok, err := p.claim(ctx, setting.Nice)
		if err != nil || !ok {
			return handled
		}
		p.process(ctx, job)
		handled++
	}
}

// Claim takes the next image the way the pool does, without optimizing it.
func (p *Pool) Claim(ctx context.Context) (Job, bool, error) { return p.claim(ctx, false) }

func (p *Pool) claim(ctx context.Context, nice bool) (Job, bool, error) {
	p.claimMu.Lock()
	defer p.claimMu.Unlock()
	now := p.Optimizer.Now()
	if now.Sub(p.requeuedAt) >= requeueInterval {
		if _, err := images.RequeueDue(ctx, p.queries, now); err != nil {
			return Job{}, false, err
		}
		p.requeuedAt = now
	}

	enabled, err := p.queries.ListEnabledFolders(ctx)
	if err != nil {
		return Job{}, false, err
	}
	report := p.reporter.Report(ctx)
	byID := make(map[int64]Job, len(enabled))
	candidates := make([]images.Candidate, 0, len(enabled))
	for _, folder := range enabled {
		var profile folders.Profile
		if err := json.Unmarshal([]byte(folder.Profile), &profile); err != nil {
			p.logger.Error("Skipping a folder whose profile cannot be read", "folder", folder.ID, "error", err)
			continue
		}
		candidate := images.Candidate{FolderID: folder.ID, Cutoff: now.Add(-minAge(profile))}
		for _, format := range images.Formats {
			if report.Satisfied(profile.Need(format)) {
				candidate.Formats = append(candidate.Formats, format)
			}
		}
		candidates = append(candidates, candidate)
		byID[folder.ID] = Job{Root: folder.Path, Profile: profile, Report: report, Nice: nice}
	}

	image, ok, err := images.Claim(ctx, p.tx, candidates, p.after, now)
	if err != nil || !ok {
		return Job{}, false, err
	}
	p.after = image.FolderID
	job := byID[image.FolderID]
	job.Image = image
	return job, true, nil
}

// process optimizes one claimed image and records the outcome, even after a stop.
func (p *Pool) process(ctx context.Context, job Job) {
	started := time.Now()
	outcome := p.Optimizer.Process(ctx, job)
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recordTimeout)
	defer cancel()
	if _, err := images.Record(recordCtx, p.tx, job.Image, outcome, p.Optimizer.Now()); err != nil {
		p.logger.Error("Could not record an optimized image", "image", job.Image.ID, "status", outcome.Status, "error", err)
		return
	}
	attrs := []any{
		"image", job.Image.ID, "folder", job.Image.FolderID, "path", filepath.Join(job.Image.Dir, job.Image.Name),
		"status", outcome.Status, "took", time.Since(started).Round(time.Millisecond),
	}
	switch outcome.Status {
	case images.StatusFailed:
		p.logger.Warn("Image failed", append(attrs, "reason", outcome.Reason)...)
	case images.StatusOptimized:
		p.logger.Debug("Image optimized", append(attrs, "bytes", outcome.Size)...)
	default:
		p.logger.Debug("Image left as it was", append(attrs, "reason", outcome.Reason)...)
	}
}

// PrepareWorkDir creates the work dir, or checks that an existing one is a directory only this
// process's user can use, then deletes the job directories a previous run left in it, and only
// those, in case WORK_DIR names a directory that holds anything else. A work dir someone else
// could replace or fill would let them steer what the tools read.
func PrepareWorkDir(dir string) error {
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return fmt.Errorf("work dir: %w", err)
	}
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("work dir: %w", err)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("work dir: %w", err)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	switch {
	case !info.IsDir():
		return fmt.Errorf("work dir %s is not a directory", dir)
	case !ok || int(st.Uid) != os.Geteuid():
		return fmt.Errorf("work dir %s belongs to another user", dir)
	case info.Mode().Perm()&0o077 != 0:
		return fmt.Errorf("work dir %s is open to other users (mode %o); make it 0700", dir, info.Mode().Perm())
	}
	leftovers, err := filepath.Glob(filepath.Join(dir, "job-*"))
	if err != nil {
		return fmt.Errorf("work dir: %w", err)
	}
	for _, path := range leftovers {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("work dir: %w", err)
		}
	}
	return nil
}
