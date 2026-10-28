package plans

import (
	"context"
	"time"

	"app/internal/db"
	"app/internal/errs"
)

// DashboardDays is how many UTC days, today included, the dashboard shows runs for.
const DashboardDays = 30

// DayCount is how many runs of a plan that started on one UTC day succeeded and failed.
type DayCount struct {
	Date      string
	Succeeded int64
	Failed    int64
}

// DashboardPlan is a plan with its runs per day, oldest day first.
type DashboardPlan struct {
	Plan
	Days []DayCount
}

// Dashboard is every plan, by name, with its last DashboardDays days of runs.
func (s *PlanService) Dashboard(ctx context.Context, now time.Time) ([]DashboardPlan, error) {
	rows, err := s.queries.ListPlans(ctx)
	if err != nil {
		return nil, errs.WrapDatabaseError(err)
	}
	plans, err := s.withRelations(ctx, rows)
	if err != nil {
		return nil, err
	}
	dates := DayWindow(now, DashboardDays)
	counts, err := s.queries.CountFinishedRunsPerDay(ctx, dates[0])
	if err != nil {
		return nil, errs.WrapDatabaseError(err)
	}
	return DailyRuns(plans, dates, counts), nil
}

// DayWindow is the `days` UTC dates up to and including the day of now, oldest first, as YYYY-MM-DD.
func DayWindow(now time.Time, days int) []string {
	now = now.UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	dates := make([]string, days)
	for i := range dates {
		dates[i] = today.AddDate(0, 0, i-days+1).Format(time.DateOnly)
	}
	return dates
}

// DailyRuns gives each plan one DayCount per date, zero for a day without runs. Counts for
// another plan or a date outside dates are ignored.
func DailyRuns(plans []Plan, dates []string, counts []db.CountFinishedRunsPerDayRow) []DashboardPlan {
	type key struct {
		plan int64
		date string
	}
	byKey := make(map[key]db.CountFinishedRunsPerDayRow, len(counts))
	for _, row := range counts {
		byKey[key{row.PlanID, row.Day}] = row
	}
	result := make([]DashboardPlan, len(plans))
	for i, plan := range plans {
		days := make([]DayCount, len(dates))
		for j, date := range dates {
			row := byKey[key{plan.ID, date}]
			days[j] = DayCount{Date: date, Succeeded: row.Succeeded, Failed: row.Failed}
		}
		result[i] = DashboardPlan{Plan: plan, Days: days}
	}
	return result
}
