package unit

import (
	"testing"
	"time"

	"app/internal/db"
	"app/internal/scans"

	"github.com/stretchr/testify/assert"
)

func TestDue(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 20*int(time.Millisecond), time.UTC)
	at := func(ago time.Duration) *time.Time {
		t := now.Add(-ago)
		return &t
	}
	folder := db.Folder{Enabled: true, ScanIntervalMinutes: 60, FullScanIntervalHours: 24}
	paused := folder
	paused.Enabled = false

	for _, tc := range []struct {
		name     string
		folder   db.Folder
		lastFull *time.Time
		lastAny  *time.Time
		kind     scans.Kind
		due      bool
	}{
		{"a new folder gets a full scan", folder, nil, nil, scans.KindFull, true},
		{"a paused folder gets nothing", paused, nil, nil, "", false},
		{"nothing within the quick interval", folder, at(10 * time.Minute), at(10 * time.Minute), "", false},
		{"quick once the interval passed", folder, at(61 * time.Minute), at(61 * time.Minute), scans.KindQuick, true},
		{"an hourly scan that started just after its tick is due on the hour", folder, at(time.Hour - 10*time.Millisecond), at(time.Hour - 10*time.Millisecond), scans.KindQuick, true},
		{"full once the full interval passed", folder, at(24 * time.Hour), at(time.Hour), scans.KindFull, true},
		{"a full scan waits for the quick interval too", folder, at(25 * time.Hour), at(5 * time.Minute), "", false},
		{"a failure is retried on the quick interval", folder, nil, at(30 * time.Minute), "", false},
		{"and then as a full scan if none ever succeeded", folder, nil, at(60 * time.Minute), scans.KindFull, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kind, due := scans.Due(tc.folder, tc.lastFull, tc.lastAny, now)
			assert.Equal(t, tc.due, due)
			assert.Equal(t, tc.kind, kind)
		})
	}
}
