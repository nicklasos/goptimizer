package plans

import (
	"database/sql"
	"time"

	"app/internal/db"
)

// Health is how a plan is doing, the colour of its card.
type Health string

const (
	HealthRestoring Health = "restoring"
	HealthRunning   Health = "running"
	HealthFailed    Health = "failed"
	HealthPaused    Health = "paused"
	HealthOverdue   Health = "overdue"
	HealthNever     Health = "never"
	HealthOK        Health = "ok"
)

// Healths is every value, in the order PlanHealth checks them.
var Healths = []Health{HealthRestoring, HealthRunning, HealthFailed, HealthPaused, HealthOverdue, HealthNever, HealthOK}

// OverdueAfter is how long a scheduled slot may go without a successful backup before the plan
// is overdue. A run takes a while, and the scheduler starts a slot within a minute.
const OverdueAfter = time.Hour

// PlanHealth is the health of a plan, the first that applies of:
//
//   - restoring: it has a restore queued or running
//   - running: its latest run is queued or running
//   - failed: its latest run failed
//   - paused: it is disabled
//   - overdue: a slot at least OverdueAfter old, later than the plan's last save, has no
//     successful run since: the scheduler did not run it, or the run never finished
//   - never: it has no runs
//   - ok: its latest run succeeded
//
// Slots before the last save do not count, the same as for the scheduler (runs.Due): saving
// a plan, or resuming it, never makes it overdue for a slot the scheduler was told to skip.
func PlanHealth(enabled bool, schedules []Schedule, savedAt time.Time, lastRun *db.Run, restoring bool, lastSuccessAt sql.NullTime, now time.Time) Health {
	switch {
	case restoring:
		return HealthRestoring
	case lastRun != nil && (lastRun.Status == "queued" || lastRun.Status == "running"):
		return HealthRunning
	case lastRun != nil && lastRun.Status == "failed":
		return HealthFailed
	case !enabled:
		return HealthPaused
	}
	if slot, ok := PrevRun(schedules, now.Add(-OverdueAfter)); ok && slot.After(savedAt) {
		if !lastSuccessAt.Valid || lastSuccessAt.Time.Before(slot) {
			return HealthOverdue
		}
	}
	if lastRun == nil {
		return HealthNever
	}
	return HealthOK
}
