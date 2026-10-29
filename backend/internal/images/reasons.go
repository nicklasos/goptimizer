package images

import "strings"

var ReasonCodes = []string{
	"symlink", "not_regular_file", "hard_link", "access_control_list", "wrong_content", "apng",
	"extra_data", "too_many_pixels", "too_large", "unreadable", "already_optimized", "too_new",
	"changed_while_optimizing", "interrupted", "tool_missing", "tool_failed", "swap_failed",
	"no_saving", "format_changed", "result_undecodable", "dimensions_changed", "frames_changed",
	"animation_changed", "pixels_changed", "ssim_below_threshold", "color_changed", "metadata_changed",
}

// LosslessJoin joins the refusal of a lossy result to how the lossless attempt that followed it
// ended: "ssim_below_threshold: 0.7100, below 0.8500, at 128,64; lossless: no_saving: ...".
const LosslessJoin = "; lossless: "

// Reason is a stored reason read back: a code from ReasonCodes and the specifics measured, such as
// "0.7100, below 0.8500, at 128,64". Code is empty for text that starts with no known code; Detail
// is then all of it.
type Reason struct {
	Code   string
	Detail string
}

var knownReasons = func() map[string]bool {
	known := make(map[string]bool, len(ReasonCodes))
	for _, code := range ReasonCodes {
		known[code] = true
	}
	return known
}()

// ParseReason reads a stored reason, "code" or "code: detail". For a lossy result that was
// refused, lossless is how the lossless attempt after it ended; nil otherwise.
func ParseReason(stored string) (reason Reason, lossless *Reason) {
	if first, rest, found := strings.Cut(stored, LosslessJoin); found {
		after := parsePart(rest)
		return parsePart(first), &after
	}
	return parsePart(stored), nil
}

func parsePart(part string) Reason {
	code, detail, _ := strings.Cut(part, ": ")
	if !knownReasons[code] {
		return Reason{Detail: part}
	}
	return Reason{Code: code, Detail: detail}
}
