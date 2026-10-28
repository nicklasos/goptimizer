package unit

import (
	"database/sql"
	"testing"
	"time"

	"app/internal/db"
	"app/internal/plans"

	"github.com/stretchr/testify/assert"
)

func TestPlanHealth(t *testing.T) {
	at := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	success := func(s string) sql.NullTime { return sql.NullTime{Time: at(s), Valid: true} }
	run := func(status string) *db.Run { return &db.Run{Status: status} }

	daily3 := []plans.Schedule{{Time: 3 * 60, Days: []plans.Weekday{}}}
	saved := at("2026-10-01T00:00:00Z")
	// Saturday 17 October 2026, 10:00. Today's 03:00 slot is seven hours old.
	now := at("2026-10-17T10:00:00Z")

	cases := []struct {
		name        string
		enabled     bool
		schedules   []plans.Schedule
		savedAt     time.Time
		lastRun     *db.Run
		restoring   bool
		lastSuccess sql.NullTime
		now         time.Time
		want        plans.Health
	}{
		{"ran today's slot", true, daily3, saved, run("succeeded"), false, success("2026-10-17T03:02:00Z"), now, plans.HealthOK},
		{"a manual plan that succeeded", true, nil, saved, run("succeeded"), false, success("2026-10-02T09:00:00Z"), now, plans.HealthOK},
		{"missed today's slot", true, daily3, saved, run("succeeded"), false, success("2026-10-16T03:02:00Z"), now, plans.HealthOverdue},
		{"a slot less than an hour old is not missed yet", true, daily3, saved, run("succeeded"), false, success("2026-10-16T03:02:00Z"), at("2026-10-17T03:59:00Z"), plans.HealthOK},
		{"a slot exactly an hour old is missed", true, daily3, saved, run("succeeded"), false, success("2026-10-16T03:02:00Z"), at("2026-10-17T04:00:00Z"), plans.HealthOverdue},
		{"a success at the slot itself counts", true, daily3, saved, run("succeeded"), false, success("2026-10-17T03:00:00Z"), now, plans.HealthOK},
		{"a manual success after the slot covers it", true, daily3, saved, run("succeeded"), false, success("2026-10-17T08:00:00Z"), now, plans.HealthOK},
		{"weekdays only: Friday's slot was run, Saturday has none", true, []plans.Schedule{{Time: 3 * 60, Days: []plans.Weekday{plans.Monday, plans.Tuesday, plans.Wednesday, plans.Thursday, plans.Friday}}}, saved, run("succeeded"), false, success("2026-10-16T03:05:00Z"), now, plans.HealthOK},
		{"never ran, but a slot after the save has passed", true, daily3, saved, nil, false, sql.NullTime{}, now, plans.HealthOverdue},
		{"never ran, saved after the last slot", true, daily3, at("2026-10-17T05:00:00Z"), nil, false, sql.NullTime{}, now, plans.HealthNever},
		{"never ran, saved within the hour before the slot", true, daily3, at("2026-10-17T02:59:00Z"), nil, false, sql.NullTime{}, at("2026-10-17T03:30:00Z"), plans.HealthNever},
		{"never ran and has no schedules", true, nil, saved, nil, false, sql.NullTime{}, now, plans.HealthNever},
		{"a slot from before the last save does not count", true, daily3, at("2026-10-17T05:00:00Z"), run("succeeded"), false, success("2026-10-10T03:02:00Z"), now, plans.HealthOK},
		{"queued", true, daily3, saved, run("queued"), false, success("2026-10-10T03:02:00Z"), now, plans.HealthRunning},
		{"running beats paused", false, daily3, saved, run("running"), false, sql.NullTime{}, now, plans.HealthRunning},
		{"failed beats overdue", true, daily3, saved, run("failed"), false, success("2026-10-10T03:02:00Z"), now, plans.HealthFailed},
		{"failed beats paused", false, daily3, saved, run("failed"), false, sql.NullTime{}, now, plans.HealthFailed},
		{"paused beats overdue", false, daily3, saved, run("succeeded"), false, success("2026-10-10T03:02:00Z"), now, plans.HealthPaused},
		{"paused and never ran", false, daily3, saved, nil, false, sql.NullTime{}, now, plans.HealthPaused},
		{"restoring", true, daily3, saved, run("succeeded"), true, success("2026-10-17T03:02:00Z"), now, plans.HealthRestoring},
		{"restoring beats running", true, daily3, saved, run("running"), true, success("2026-10-17T03:02:00Z"), now, plans.HealthRestoring},
		{"restoring beats failed", true, daily3, saved, run("failed"), true, sql.NullTime{}, now, plans.HealthRestoring},
		{"restoring beats paused", false, daily3, saved, nil, true, sql.NullTime{}, now, plans.HealthRestoring},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, plans.PlanHealth(tc.enabled, tc.schedules, tc.savedAt, tc.lastRun, tc.restoring, tc.lastSuccess, tc.now))
		})
	}
}

func TestDailyRuns(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 30, 0, 0, time.FixedZone("UTC+2", 2*3600))
	dates := plans.DayWindow(now, 30)
	assert.Len(t, dates, 30)
	assert.Equal(t, "2026-01-30", dates[0], "the window crosses February, oldest first")
	assert.Equal(t, "2026-02-28", dates[29], "today is the UTC day: 00:30 at UTC+2 is still 28 February")

	result := plans.DailyRuns(
		[]plans.Plan{{Plan: db.Plan{ID: 1}}, {Plan: db.Plan{ID: 2}}},
		dates,
		[]db.CountFinishedRunsPerDayRow{
			{PlanID: 1, Day: "2026-01-30", Succeeded: 2},
			{PlanID: 1, Day: "2026-02-28", Succeeded: 1, Failed: 1},
			{PlanID: 2, Day: "2026-02-10", Failed: 3},
			{PlanID: 1, Day: "2026-01-29", Succeeded: 9},
			{PlanID: 3, Day: "2026-02-10", Succeeded: 9},
		},
	)

	assert.Len(t, result, 2)
	first := result[0].Days
	assert.Len(t, first, 30)
	assert.Equal(t, plans.DayCount{Date: "2026-01-30", Succeeded: 2}, first[0])
	assert.Equal(t, plans.DayCount{Date: "2026-01-31"}, first[1], "a day without runs is zeros")
	assert.Equal(t, plans.DayCount{Date: "2026-02-28", Succeeded: 1, Failed: 1}, first[29])
	assert.Equal(t, plans.DayCount{Date: "2026-02-10", Failed: 3}, result[1].Days[11])
	var total int64
	for _, day := range result[1].Days {
		total += day.Succeeded + day.Failed
	}
	assert.Equal(t, int64(3), total, "counts of other plans and days outside the window are left out")
}
