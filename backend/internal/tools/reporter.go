package tools

import (
	"context"
	"slices"
	"sync"
	"time"
)

// Reporter keeps the last Check for a while, so the settings page, the folder cards and the
// optimizer's claims share one look at PATH instead of starting five programs each time.
type Reporter struct {
	finder Finder
	ttl    time.Duration

	mu      sync.Mutex
	report  Report
	checked time.Time
}

// ReportTTL is how long a report is reused. Looking up five programs runs five processes; the
// settings page is opened rarely but reloaded in bursts, and the optimizer claims constantly.
const ReportTTL = 30 * time.Second

func NewReporter(finder Finder, ttl time.Duration) *Reporter {
	return &Reporter{finder: finder, ttl: ttl}
}

// Report is the cached report, looked up again when it is older than the TTL.
func (r *Reporter) Report(ctx context.Context) Report {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.checked.IsZero() || time.Since(r.checked) >= r.ttl {
		r.report, r.checked = Check(ctx, r.finder), time.Now()
	}
	return r.report
}

// Refresh looks every program up again now.
func (r *Reporter) Refresh(ctx context.Context) Report {
	r.mu.Lock()
	r.checked = time.Time{}
	r.mu.Unlock()
	return r.Report(ctx)
}

// Path is where the report found a program; false when it is not installed.
func (r Report) Path(name string) (string, bool) {
	i := slices.IndexFunc(r.Programs, func(p ProgramStatus) bool { return p.Name == name })
	if i < 0 || !r.Programs[i].Installed {
		return "", false
	}
	return r.Programs[i].Path, true
}

// Satisfied reports whether every step of the need has an installed program.
func (r Report) Satisfied(need Need) bool {
	i := slices.IndexFunc(r.Needs, func(n NeedStatus) bool { return n.Need == need })
	return i >= 0 && r.Needs[i].Satisfied
}

// Missing is the preferred program of each step of the need that nothing installed can do.
func (r Report) Missing(need Need) []string {
	i := slices.IndexFunc(r.Needs, func(n NeedStatus) bool { return n.Need == need })
	if i < 0 {
		return nil
	}
	return r.Needs[i].Missing
}
