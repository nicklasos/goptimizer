package images

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"slices"
	"time"

	"app/internal/db"
)

// retryAfter is how long a failed image waits before each new attempt. An image that failed once
// more than this has entries stays failed, for the admin: MaxAttempts in all.
var retryAfter = [...]time.Duration{10 * time.Minute, time.Hour}

const MaxAttempts = len(retryAfter) + 1

// RetryAt is when an image that has now failed attempts times is tried again; false when never.
func RetryAt(attempts int64, failedAt time.Time) (time.Time, bool) {
	if attempts < 1 || attempts > int64(len(retryAfter)) {
		return time.Time{}, false
	}
	return failedAt.Add(retryAfter[attempts-1]), true
}

// Candidate is a folder the claim may take an image from: the formats whose tools are installed
// for its mode, and the newest mtime an image may have to count as finished uploading.
type Candidate struct {
	FolderID int64
	Formats  []Format
	Cutoff   time.Time
}

// Order is the order the claim tries folders in. The fewest images in flight go first, so one
// folder never holds every worker while another waits. Ties go round-robin, from the first folder
// after the one claimed from last, so with one worker the folders take turns.
func Order(candidates []Candidate, inFlight map[int64]int64, after int64) []Candidate {
	ordered := slices.Clone(candidates)
	slices.SortFunc(ordered, func(a, b Candidate) int {
		if c := cmp.Compare(inFlight[a.FolderID], inFlight[b.FolderID]); c != 0 {
			return c
		}
		if aAfter, bAfter := a.FolderID > after, b.FolderID > after; aAfter != bAfter {
			if aAfter {
				return -1
			}
			return 1
		}
		return cmp.Compare(a.FolderID, b.FolderID)
	})
	return ordered
}

// Claim moves the next image to optimizing and returns it: from the first folder in Order that has
// one, its pending image with the oldest mtime at or before the cutoff, in an installed format.
// after is the folder claimed from last. False when no folder has one. It runs in one write
// transaction, so two claims never take the same image.
func Claim(ctx context.Context, tx *db.TxRunner, candidates []Candidate, after int64, now time.Time) (db.Image, bool, error) {
	var claimed db.Image
	var ok bool
	err := tx.WithTx(ctx, func(q *db.Queries) error {
		counts, err := q.CountOptimizingByFolder(ctx)
		if err != nil {
			return err
		}
		inFlight := make(map[int64]int64, len(counts))
		for _, c := range counts {
			inFlight[c.FolderID] = c.Count
		}
		for _, candidate := range Order(candidates, inFlight, after) {
			id, found, err := oldest(ctx, q, candidate)
			if err != nil {
				return err
			}
			if !found {
				continue
			}
			claimed, err = q.ClaimImage(ctx, db.ClaimImageParams{ID: id, Now: now.UTC()})
			ok = err == nil
			return err
		}
		return nil
	})
	return claimed, ok, err
}

// oldest is the claimable image of one folder with the oldest mtime: one index seek per format.
func oldest(ctx context.Context, q *db.Queries, c Candidate) (int64, bool, error) {
	var best db.OldestClaimableImageRow
	found := false
	for _, format := range c.Formats {
		row, err := q.OldestClaimableImage(ctx, db.OldestClaimableImageParams{
			FolderID: c.FolderID, Format: string(format), Cutoff: c.Cutoff.UnixNano(),
		})
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return 0, false, err
		}
		if !found || row.MtimeNs < best.MtimeNs {
			best, found = row, true
		}
	}
	return best.ID, found, nil
}

// RequeueDue makes the failed images whose retry time has come pending again.
func RequeueDue(ctx context.Context, q *db.Queries, now time.Time) (int64, error) {
	return q.RequeueDueImages(ctx, now.UTC())
}

// Recover makes every image left optimizing by a process that stopped pending again. Only one
// process runs the optimizer, so at its start no image can be in flight anywhere.
func Recover(ctx context.Context, q *db.Queries, now time.Time) (int64, error) {
	return q.RecoverOptimizingImages(ctx, now.UTC())
}

// Outcome is how one claim of an image ended: the transition out of optimizing. Size and MtimeNs
// are the file on disk now. Swap is set for optimized when this claim replaced the file; optimized
// without a Swap means the file already was the optimizer's output.
type Outcome struct {
	Status  Status
	Reason  string
	Size    int64
	MtimeNs int64
	Swap    *Swap
}

// Swap is what a replacement recorded: the original's size, the result's mode and hash, and the
// size before this swap (the original's, or an earlier result's) for the day's stats.
type Swap struct {
	OriginalSize int64
	BytesBefore  int64
	Mode         string
	OutputSHA256 []byte
}

// Record writes the outcome of a claimed image as one conditional UPDATE on optimizing. False when
// the image was no longer optimizing, such as one whose folder was deleted.
func Record(ctx context.Context, tx *db.TxRunner, image db.Image, o Outcome, now time.Time) (bool, error) {
	now = now.UTC()
	var n int64
	err := tx.WithTx(ctx, func(q *db.Queries) error {
		var err error
		switch {
		case o.Status == StatusPending:
			n, err = q.ReleaseImage(ctx, db.ReleaseImageParams{ID: image.ID, Size: o.Size, MtimeNs: o.MtimeNs, Reason: reason(o.Reason), Now: now})
		case o.Status == StatusFailed:
			attempts := image.Attempts + 1
			retryAt, again := RetryAt(attempts, now)
			n, err = q.FailImage(ctx, db.FailImageParams{
				ID: image.ID, Reason: reason(o.Reason), Attempts: attempts, NextAttemptAt: sql.NullTime{Time: retryAt, Valid: again},
				Size: o.Size, MtimeNs: o.MtimeNs, Now: now,
			})
		case o.Status == StatusOptimized && o.Swap == nil:
			n, err = q.RestoreOptimized(ctx, db.RestoreOptimizedParams{ID: image.ID, Size: o.Size, MtimeNs: o.MtimeNs, Now: now})
		case o.Status == StatusOptimized:
			n, err = q.RecordOptimized(ctx, db.RecordOptimizedParams{
				ID: image.ID, Size: o.Size, MtimeNs: o.MtimeNs, OriginalSize: sql.NullInt64{Int64: o.Swap.OriginalSize, Valid: true},
				Mode: sql.NullString{String: o.Swap.Mode, Valid: true}, OutputSha256: o.Swap.OutputSHA256, Now: now,
			})
			if err == nil && n == 1 {
				err = q.AddDailyStat(ctx, db.AddDailyStatParams{FolderID: image.FolderID, Day: Day(now), BytesBefore: o.Swap.BytesBefore, BytesAfter: o.Size})
			}
		default:
			n, err = q.LeaveImage(ctx, db.LeaveImageParams{ID: image.ID, Status: string(o.Status), Reason: reason(o.Reason), Size: o.Size, MtimeNs: o.MtimeNs, Now: now})
		}
		return err
	})
	return n == 1, err
}

func reason(r string) sql.NullString { return sql.NullString{String: r, Valid: r != ""} }
