package runs

import (
	"sort"
	"time"
)

// Retention is a plan's rule for old archives, as stored on the plan. MaxAgeDays 0 keeps every
// archive. KeepWeekly and KeepMonthly keep, of the archives older than MaxAgeDays, the first of
// each ISO week and of each calendar month (UTC), for the last N weeks or months counted back
// from now, the current one included.
type Retention struct {
	MaxAgeDays  int64
	KeepWeekly  int64
	KeepMonthly int64
}

// Archive is what retention needs to know about one stored archive.
type Archive struct {
	ID        int64
	CreatedAt time.Time
}

// Prune returns the archives the policy removes, of one plan on one storage. The newest archive
// is never removed, even when it is past MaxAgeDays: a plan whose backups stopped keeps its
// last copy.
func Prune(archives []Archive, policy Retention, now time.Time) []int64 {
	if policy.MaxAgeDays <= 0 || len(archives) == 0 {
		return nil
	}
	sorted := append([]Archive(nil), archives...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if !sorted[i].CreatedAt.Equal(sorted[j].CreatedAt) {
			return sorted[i].CreatedAt.Before(sorted[j].CreatedAt)
		}
		return sorted[i].ID < sorted[j].ID
	})

	now = now.UTC()
	cutoff := now.Add(-time.Duration(policy.MaxAgeDays) * 24 * time.Hour)
	firstWeek := weekStart(now).AddDate(0, 0, -7*int(policy.KeepWeekly-1))
	firstMonth := monthStart(now).AddDate(0, -int(policy.KeepMonthly-1), 0)
	keptWeeks := map[time.Time]bool{}
	keptMonths := map[time.Time]bool{}

	var remove []int64
	for i, archive := range sorted {
		at := archive.CreatedAt.UTC()
		if i == len(sorted)-1 || at.After(cutoff) {
			continue
		}
		keep := false
		if week := weekStart(at); policy.KeepWeekly > 0 && !week.Before(firstWeek) && !keptWeeks[week] {
			keptWeeks[week], keep = true, true
		}
		if month := monthStart(at); policy.KeepMonthly > 0 && !month.Before(firstMonth) && !keptMonths[month] {
			keptMonths[month], keep = true, true
		}
		if !keep {
			remove = append(remove, archive.ID)
		}
	}
	return remove
}

// weekStart is the Monday 00:00 UTC that starts t's ISO week.
func weekStart(t time.Time) time.Time {
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return day.AddDate(0, 0, -(int(day.Weekday())+6)%7)
}

func monthStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}
