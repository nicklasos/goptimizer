// Package images is what the index knows about one image file: its format, and where it is in its
// life from found to optimized or gone.
package images

import (
	"path/filepath"
	"strings"
	"time"
)

// Status is an image's place in its life. Each step is one conditional UPDATE on the current
// status. The scanner writes pending and gone; the optimizer the rest.
type Status string

const (
	StatusPending    Status = "pending"
	StatusOptimizing Status = "optimizing"
	StatusOptimized  Status = "optimized"
	StatusKept       Status = "kept"
	StatusSkipped    Status = "skipped"
	StatusFailed     Status = "failed"
	StatusGone       Status = "gone"
)

type Format string

const (
	FormatPNG  Format = "png"
	FormatJPEG Format = "jpeg"
	FormatGIF  Format = "gif"
)

// Formats is every format the index holds, in the order the optimizer's checks list them.
var Formats = []Format{FormatPNG, FormatJPEG, FormatGIF}

// extensions decides which files are indexed. Whether the content matches is the optimizer's check.
var extensions = map[string]Format{
	".png":  FormatPNG,
	".jpg":  FormatJPEG,
	".jpeg": FormatJPEG,
	".gif":  FormatGIF,
}

// FormatOf is the format a file name says it has, by its extension in any case.
func FormatOf(name string) (Format, bool) {
	format, ok := extensions[strings.ToLower(filepath.Ext(name))]
	return format, ok
}

// Day is the daily_stats key of a moment: its UTC date.
func Day(at time.Time) string { return at.UTC().Format(time.DateOnly) }
