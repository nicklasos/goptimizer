package unit

import (
	"testing"
	"time"

	"app/internal/images"

	"github.com/stretchr/testify/assert"
)

func folderIDs(candidates []images.Candidate) []int64 {
	ids := make([]int64, len(candidates))
	for i, c := range candidates {
		ids[i] = c.FolderID
	}
	return ids
}

func TestOrder_FewestInFlightFirstThenRoundRobin(t *testing.T) {
	candidates := []images.Candidate{{FolderID: 1}, {FolderID: 2}, {FolderID: 3}, {FolderID: 4}}

	for _, tc := range []struct {
		name     string
		inFlight map[int64]int64
		after    int64
		want     []int64
	}{
		{"nothing in flight, nothing claimed yet", nil, 0, []int64{1, 2, 3, 4}},
		{"turns go on after the folder claimed last", nil, 2, []int64{3, 4, 1, 2}},
		{"and wrap around", nil, 4, []int64{1, 2, 3, 4}},
		{"a folder with work in flight waits for the others", map[int64]int64{1: 3, 3: 1}, 0, []int64{2, 4, 3, 1}},
		{"ties among the busy go round-robin too", map[int64]int64{1: 1, 2: 1, 3: 1, 4: 1}, 3, []int64{4, 1, 2, 3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, folderIDs(images.Order(candidates, tc.inFlight, tc.after)))
		})
	}
}

// One worker, a folder of a million images and one of ten: claiming one image at a time and
// finishing it before the next, the small folder gets every other claim.
func TestOrder_AHugeFolderDoesNotStarveASmallOne(t *testing.T) {
	pending := map[int64]int{1: 1_000_000, 2: 10}
	var after int64
	var claims []int64
	for range 20 {
		for _, c := range images.Order([]images.Candidate{{FolderID: 1}, {FolderID: 2}}, nil, after) {
			if pending[c.FolderID] > 0 {
				pending[c.FolderID]--
				after = c.FolderID
				claims = append(claims, c.FolderID)
				break
			}
		}
	}
	assert.Equal(t, []int64{1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2}, claims)
	assert.Zero(t, pending[2])
}

func TestRetryAt(t *testing.T) {
	failedAt := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)

	first, again := images.RetryAt(1, failedAt)
	assert.True(t, again)
	assert.Equal(t, failedAt.Add(10*time.Minute), first, "after the first failure, ten minutes")

	second, again := images.RetryAt(2, failedAt)
	assert.True(t, again)
	assert.Equal(t, failedAt.Add(time.Hour), second, "after the second, an hour")

	_, again = images.RetryAt(3, failedAt)
	assert.False(t, again, "after the third, never: it stays failed")
	assert.Equal(t, 3, images.MaxAttempts)
}
