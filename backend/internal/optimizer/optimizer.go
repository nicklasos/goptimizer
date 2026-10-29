// Package optimizer optimizes the images the scanner indexed: a pool of workers claims pending
// images fairly across folders, and each claim runs the folder's pipeline on a copy in the work
// dir, verifies the result and swaps it in place of the original. No copy of an original is
// kept, so every rule here errs towards leaving the file alone.
package optimizer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	"app/internal/db"
	"app/internal/folders"
	"app/internal/images"
	"app/internal/tools"
	"app/internal/verify"

	"golang.org/x/sys/unix"
)

const (
	// MaxPixels bounds what one image may decode to, checked from its headers before any decode,
	// so a small file that claims to be huge cannot exhaust memory. 50 million covers the 48 MP
	// photos of current phones. Verifying an image that large holds about 13 bytes per pixel,
	// about 650 MB, while it runs.
	MaxPixels = 50_000_000
	// MaxFileBytes bounds what is read into memory. No image within MaxPixels is near it.
	MaxFileBytes = 256 << 20
	// DefaultToolTimeout bounds one tool run. The slowest step on the calibration corpus took a
	// few seconds; a tool still running after this is treated as hung.
	DefaultToolTimeout = 5 * time.Minute
)

// Reasons the optimizer records, besides the verify reasons for a result that was refused.
const (
	ReasonSymlink          = "symlink"
	ReasonNotRegular       = "not_regular_file"
	ReasonHardLink         = "hard_link"
	ReasonWrongContent     = "wrong_content"
	ReasonAPNG             = "apng"
	ReasonTooManyPixels    = "too_many_pixels"
	ReasonTooLarge         = "too_large"
	ReasonUnreadable       = "unreadable"
	ReasonAlreadyOptimized = "already_optimized"
	ReasonTooNew           = "too_new"
	ReasonChanged          = "changed_while_optimizing"
	ReasonInterrupted      = "interrupted"
	ReasonToolMissing      = "tool_missing"
	ReasonToolFailed       = "tool_failed"
	ReasonSwapFailed       = "swap_failed"
)

// Job is one claimed image and what optimizing it needs.
type Job struct {
	Image   db.Image
	Root    string
	Profile folders.Profile
	Report  tools.Report
	Nice    bool
}

// Optimizer runs the steps for one image. It writes nothing to the database but the output hash
// before a swap; the caller records the outcome.
type Optimizer struct {
	queries     *db.Queries
	workDir     string
	ToolTimeout time.Duration
	// Now is the clock of the min_age_minutes rule. Tests replace it.
	Now func() time.Time
}

func New(queries *db.Queries, workDir string) *Optimizer {
	return &Optimizer{queries: queries, workDir: workDir, ToolTimeout: DefaultToolTimeout, Now: time.Now}
}

// Process optimizes one image and returns how it ended. The steps follow docs/PLAN.md, "Optimizing
// one image". ctx stops the tools; once a result has passed verification the swap and its record
// go on regardless, so a stop never leaves a swap half done.
func (o *Optimizer) Process(ctx context.Context, job Job) images.Outcome {
	img := job.Image
	format := images.Format(img.Format)
	unchanged := images.Outcome{Status: images.StatusPending, Size: img.Size, MtimeNs: img.MtimeNs}

	dir, err := openDir(job.Root, img.Dir)
	if err != nil {
		if gone(err) {
			return images.Outcome{Status: images.StatusGone, Size: img.Size, MtimeNs: img.MtimeNs}
		}
		return leave(unchanged, images.StatusSkipped, ReasonUnreadable)
	}
	defer dir.Close()

	st, err := lstatAt(dir, img.Name)
	switch {
	case errors.Is(err, unix.ENOENT):
		return images.Outcome{Status: images.StatusGone, Size: img.Size, MtimeNs: img.MtimeNs}
	case err != nil:
		return leave(unchanged, images.StatusSkipped, ReasonUnreadable)
	}
	onDisk := images.Outcome{Size: st.Size, MtimeNs: mtimeNs(st)}
	switch {
	case isSymlink(st):
		return leave(onDisk, images.StatusSkipped, ReasonSymlink)
	case !isRegular(st):
		return leave(onDisk, images.StatusSkipped, ReasonNotRegular)
	case st.Nlink > 1:
		return leave(onDisk, images.StatusSkipped, ReasonHardLink)
	case mtimeNs(st) > o.Now().Add(-minAge(job.Profile)).UnixNano():
		return leave(onDisk, images.StatusPending, ReasonTooNew)
	case st.Size > MaxFileBytes:
		return leave(onDisk, images.StatusSkipped, ReasonTooLarge)
	}

	original, err := readFile(dir, img.Name, st)
	switch {
	case errors.Is(err, errChanged):
		return leave(unchanged, images.StatusPending, ReasonChanged)
	case err != nil:
		return leave(onDisk, images.StatusSkipped, ReasonUnreadable)
	}
	if real, ok := verify.Sniff(original); !ok || real != format {
		return leave(onDisk, images.StatusSkipped, ReasonWrongContent)
	}
	if format == images.FormatPNG && verify.IsAPNG(original) {
		return leave(onDisk, images.StatusSkipped, ReasonAPNG)
	}
	pixels, err := verify.Pixels(original, format)
	switch {
	case err != nil:
		return leave(onDisk, images.StatusSkipped, ReasonUnreadable)
	case pixels > MaxPixels:
		return leave(onDisk, images.StatusSkipped, ReasonTooManyPixels)
	}

	sum := sha256.Sum256(original)
	if bytes.Equal(sum[:], img.OutputSha256) {
		return leave(onDisk, images.StatusOptimized, "")
	}
	if ours, err := o.queries.IsOptimizerOutput(ctx, sum[:]); err != nil {
		return leave(unchanged, images.StatusPending, ReasonInterrupted)
	} else if ours {
		return leave(onDisk, images.StatusKept, ReasonAlreadyOptimized)
	}

	if _, ok := Pipeline(format, job.Profile, job.Report); !ok {
		return leave(onDisk, images.StatusPending, ReasonToolMissing)
	}
	profile := job.Profile
	result, refused, err := o.attempt(ctx, job, profile, format, original)
	if refused != nil && profile.Mode == folders.ModeLossy {
		profile.Mode = folders.ModeLossless
		var lossless *verify.Failure
		result, lossless, err = o.attempt(ctx, job, profile, format, original)
		if lossless != nil {
			refused.Detail += "; lossless: " + lossless.Error()
		} else if err == nil {
			refused = nil
		}
	}
	switch {
	case ctx.Err() != nil:
		return leave(unchanged, images.StatusPending, ReasonInterrupted)
	case err != nil:
		return leave(onDisk, images.StatusFailed, fmt.Sprintf("%s: %v", ReasonToolFailed, err))
	case refused != nil:
		return leave(onDisk, images.StatusKept, refused.Error())
	}
	job.Profile = profile
	return o.replace(context.WithoutCancel(ctx), dir, job, st, original, result)
}

// attempt runs the profile's pipeline on the original and verifies the result. A lossy result that
// is refused gets one more attempt from the caller, with the lossless pipeline and its stricter
// check: every pixel the same. A refusal is not an error; a tool that fails is.
func (o *Optimizer) attempt(ctx context.Context, job Job, profile folders.Profile, format images.Format, original []byte) ([]byte, *verify.Failure, error) {
	steps, ok := Pipeline(format, profile, job.Report)
	if !ok {
		return nil, &verify.Failure{Reason: ReasonToolMissing}, nil
	}
	result, err := RunPipeline(ctx, o.workDir, steps, profile, job.Nice, o.ToolTimeout, extension(format, job.Image.Name), original)
	if err != nil {
		return nil, nil, err
	}
	check := verify.Check{
		Lossy:            profile.Mode == folders.ModeLossy,
		MinSSIM:          profile.MinSSIM,
		MinSavingPercent: profile.MinSavingPercent,
	}
	return result, verify.Compare(original, result, format, check), nil
}

// replace swaps the verified result in. The output hash is stored first, so if the process dies
// between the rename and the record, the file is still recognized as the optimizer's output.
func (o *Optimizer) replace(ctx context.Context, dir *os.File, job Job, st unix.Stat_t, original, result []byte) images.Outcome {
	img := job.Image
	onDisk := images.Outcome{Size: st.Size, MtimeNs: mtimeNs(st)}
	sum := sha256.Sum256(result)
	n, err := o.queries.SetImageOutputHash(ctx, db.SetImageOutputHashParams{ID: img.ID, OutputSha256: sum[:], Now: o.Now().UTC()})
	if err != nil || n == 0 {
		return leave(onDisk, images.StatusPending, ReasonInterrupted)
	}

	err = swap(dir, img.Name, result, st)
	var changed *changedError
	switch {
	case errors.As(err, &changed) && changed.missing:
		return images.Outcome{Status: images.StatusGone, Size: st.Size, MtimeNs: mtimeNs(st)}
	case errors.As(err, &changed):
		return images.Outcome{Status: images.StatusPending, Reason: ReasonChanged, Size: changed.stat.Size, MtimeNs: mtimeNs(changed.stat)}
	case err != nil && !renamedThen(err):
		return leave(onDisk, images.StatusFailed, fmt.Sprintf("%s: %v", ReasonSwapFailed, err))
	}
	return images.Outcome{
		Status:  images.StatusOptimized,
		Size:    int64(len(result)),
		MtimeNs: mtimeNs(st),
		Swap: &images.Swap{
			OriginalSize: int64(len(original)),
			BytesBefore:  int64(len(original)),
			Mode:         string(job.Profile.Mode),
			OutputSHA256: sum[:],
		},
	}
}

// renamedThen is an error from after the rename: the new file is in place, so the swap counts.
func renamedThen(err error) bool { return errors.Is(err, errDirSync) }

// RunPipeline copies the original into a fresh directory under workDir, runs every step on that
// copy and returns what it became. The project's file is never given to a tool.
func RunPipeline(ctx context.Context, workDir string, steps []Step, profile folders.Profile, nice bool, timeout time.Duration, ext string, original []byte) ([]byte, error) {
	work, err := os.MkdirTemp(workDir, "job-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(work)
	file := filepath.Join(work, "image"+ext)
	if err := os.WriteFile(file, original, 0o600); err != nil {
		return nil, err
	}
	for _, s := range steps {
		if err := runStep(ctx, s, profile, file, work, nice, timeout); err != nil {
			return nil, err
		}
	}
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readAllLimit(f, MaxFileBytes)
}

func leave(o images.Outcome, status images.Status, reason string) images.Outcome {
	o.Status, o.Reason = status, reason
	return o
}

func minAge(p folders.Profile) time.Duration { return time.Duration(p.MinAgeMinutes) * time.Minute }

// gone is a directory that is no longer there as a real directory: missing, a file, or a symlink.
func gone(err error) bool {
	return errors.Is(err, unix.ENOENT) || errors.Is(err, unix.ENOTDIR) || errors.Is(err, unix.ELOOP)
}

// extension keeps the image's own extension on the work copy; some tools pick their behavior by it.
func extension(format images.Format, name string) string {
	if ext := path.Ext(name); ext != "" {
		return ext
	}
	return "." + string(format)
}
