package runs

import (
	"time"

	"app/internal/plans"
)

// Due is the scheduled slot a plan should run now, if any: the latest slot at or before now,
// when it is later than both the last slot already enqueued and the last time the plan was saved.
// After downtime that is the latest missed slot, once, never one run per missed slot. Counting
// from the last save means re-enabling a paused plan, or adding a time that has already passed
// today, waits for the next slot instead of starting a backup on the spot.
func Due(schedules []plans.Schedule, enabled bool, savedAt time.Time, lastScheduled *time.Time, now time.Time) (time.Time, bool) {
	if !enabled {
		return time.Time{}, false
	}
	slot, ok := plans.PrevRun(schedules, now)
	if !ok || !slot.After(savedAt) {
		return time.Time{}, false
	}
	if lastScheduled != nil && !slot.After(*lastScheduled) {
		return time.Time{}, false
	}
	return slot, true
}
