package settings

import (
	"context"
	"runtime"
	"time"

	"app/internal/cache"
	"app/internal/db"
	"app/internal/errs"
	"app/internal/tools"
)

const (
	toolsCacheKey = "tools:report"
	// Looking up five programs runs five processes; the page is opened rarely but reloaded in bursts.
	toolsCacheTTL = 30 * time.Second
)

type SettingsService struct {
	queries *db.Queries
	cache   *cache.MemoryCache
	finder  tools.Finder
}

func NewSettingsService(queries *db.Queries, cache *cache.MemoryCache, finder tools.Finder) *SettingsService {
	return &SettingsService{queries: queries, cache: cache, finder: finder}
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

// Tools reports the image tools, from a cache of a few seconds unless refresh is set.
func (s *SettingsService) Tools(ctx context.Context, refresh bool) (tools.Report, error) {
	if refresh {
		_ = s.cache.Forget(ctx, toolsCacheKey)
	}
	var report tools.Report
	err := s.cache.Remember(ctx, toolsCacheKey, toolsCacheTTL, func() (interface{}, error) {
		return tools.Check(ctx, s.finder), nil
	}, &report)
	return report, err
}
