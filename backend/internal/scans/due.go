package scans

import (
	"time"

	"app/internal/db"
)

// Due is the scan a folder needs at now, if any. lastFull is when its last successful full scan
// started, lastAny when its last finished scan of either kind started, nil when there is none.
//
// Nothing is due within scan_interval_minutes of lastAny, failed scans included, so a folder that
// cannot be read is retried on its interval and not every minute. Past that, a full scan is due
// when the folder never had a successful one or the last is full_scan_interval_hours old, and a
// quick scan otherwise.
func Due(folder db.Folder, lastFull, lastAny *time.Time, now time.Time) (Kind, bool) {
	if !folder.Enabled {
		return "", false
	}
	quick := time.Duration(folder.ScanIntervalMinutes) * time.Minute
	full := time.Duration(folder.FullScanIntervalHours) * time.Hour
	if lastAny != nil && elapsed(*lastAny, now) < quick {
		return "", false
	}
	if lastFull == nil || elapsed(*lastFull, now) >= full {
		return KindFull, true
	}
	return KindQuick, true
}

// elapsed counts whole minutes: the schedule ticks at the start of each minute and a scan starts
// a moment after its tick, so an hourly scan would otherwise slip to every 61 minutes.
func elapsed(since, now time.Time) time.Duration {
	return now.Truncate(time.Minute).Sub(since.Truncate(time.Minute))
}
