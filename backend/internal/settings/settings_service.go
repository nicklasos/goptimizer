package settings

import (
	"context"
	"runtime"

	"app/internal/db"
	"app/internal/errs"
	"app/internal/tools"
)

type SettingsService struct {
	queries  *db.Queries
	reporter *tools.Reporter
}

func NewSettingsService(queries *db.Queries, reporter *tools.Reporter) *SettingsService {
	return &SettingsService{queries: queries, reporter: reporter}
}

// EffectiveWorkers is how many images are optimized at once: the stored number, or for 0
// one less than the CPU count, so the server keeps a core for its projects.
func EffectiveWorkers(stored int64) int64 {
	if stored > 0 {
		return stored
	}
	return max(1, int64(runtime.NumCPU())-1)
}

func (s *SettingsService) Get(ctx context.Context) (*db.Setting, error) {
	setting, err := s.queries.GetSettings(ctx)
	if err != nil {
		return nil, errs.WrapDatabaseError(err)
	}
	return &setting, nil
}

func (s *SettingsService) Update(ctx context.Context, req SettingsRequest) (*db.Setting, error) {
	setting, err := s.queries.UpdateSettings(ctx, db.UpdateSettingsParams{
		Workers:     *req.Workers,
		ScanWorkers: *req.ScanWorkers,
		Nice:        *req.Nice,
	})
	if err != nil {
		return nil, errs.WrapDatabaseError(err)
	}
	return &setting, nil
}

// Tools reports the image tools, from the shared report unless refresh is set.
func (s *SettingsService) Tools(ctx context.Context, refresh bool) tools.Report {
	if refresh {
		return s.reporter.Refresh(ctx)
	}
	return s.reporter.Report(ctx)
}
