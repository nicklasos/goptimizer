package folders

import (
	"encoding/json"
	"slices"

	"app/internal/errs"
	"app/internal/images"
	"app/internal/tools"

	"github.com/bmatcuk/doublestar/v4"
)

type Mode string

const (
	ModeLossy    Mode = "lossy"
	ModeLossless Mode = "lossless"
)

// pipelines is the tool need that optimizes each format in each mode. tools.NeedSteps lists the
// programs a need runs; internal/optimizer knows how to call each one.
var pipelines = map[Mode]map[images.Format]tools.Need{
	ModeLossy:    {images.FormatPNG: tools.NeedPNGLossy, images.FormatJPEG: tools.NeedJPEG, images.FormatGIF: tools.NeedGIF},
	ModeLossless: {images.FormatPNG: tools.NeedPNGLossless, images.FormatJPEG: tools.NeedJPEG, images.FormatGIF: tools.NeedGIF},
}

// Need is what optimizes images of the format under this profile.
func (p Profile) Need(format images.Format) tools.Need { return pipelines[p.Mode][format] }

// MissingTools is every program this profile's pipelines need that is not installed. Images of a
// format whose need is missing stay pending until it is.
func (p Profile) MissingTools(report tools.Report) []string {
	missing := []string{}
	for _, format := range images.Formats {
		for _, name := range report.Missing(p.Need(format)) {
			if !slices.Contains(missing, name) {
				missing = append(missing, name)
			}
		}
	}
	return missing
}

// Profile is how a folder's images are compressed and which of them are left alone.
// It is stored as JSON in folders.profile.
type Profile struct {
	Mode             Mode     `json:"mode" enums:"lossy,lossless"`
	JPEGMaxQuality   int      `json:"jpeg_max_quality"`
	PNGQualityMin    int      `json:"png_quality_min"`
	PNGQualityMax    int      `json:"png_quality_max"`
	GIFLossy         int      `json:"gif_lossy"`
	MinSavingPercent int      `json:"min_saving_percent"`
	MinSSIM          float64  `json:"min_ssim"`
	MinAgeMinutes    int      `json:"min_age_minutes"`
	Excludes         []string `json:"excludes"`
}

func DefaultProfile() Profile {
	return Profile{
		Mode:             ModeLossy,
		JPEGMaxQuality:   82,
		PNGQualityMin:    65,
		PNGQualityMax:    80,
		GIFLossy:         30,
		MinSavingPercent: 3,
		// The lowest 64-pixel tile's SSIM, set from 88 real and generated images:
		// docs/benchmarks/ssim.md.
		MinSSIM:       0.85,
		MinAgeMinutes: 5,
		Excludes:      []string{},
	}
}

// UnmarshalJSON starts from the defaults, so a request that leaves a field out, or a row
// stored before a field existed, gets that field's default.
func (p *Profile) UnmarshalJSON(data []byte) error {
	type plain Profile
	decoded := plain(DefaultProfile())
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*p = Profile(decoded)
	if p.Excludes == nil {
		p.Excludes = []string{}
	}
	return nil
}

// Validate returns the failures keyed by field path ("profile.gif_lossy"), or nil.
func (p Profile) Validate() errs.Fields {
	fields := errs.Fields{}
	if p.Mode != ModeLossy && p.Mode != ModeLossless {
		fields.Add("profile.mode", "oneof")
	}
	between(fields, "profile.jpeg_max_quality", float64(p.JPEGMaxQuality), 40, 100)
	between(fields, "profile.png_quality_min", float64(p.PNGQualityMin), 0, 100)
	between(fields, "profile.png_quality_max", float64(p.PNGQualityMax), 0, 100)
	between(fields, "profile.gif_lossy", float64(p.GIFLossy), 0, 200)
	between(fields, "profile.min_saving_percent", float64(p.MinSavingPercent), 0, 90)
	between(fields, "profile.min_ssim", p.MinSSIM, 0.5, 1)
	between(fields, "profile.min_age_minutes", float64(p.MinAgeMinutes), 0, 1440)
	if p.PNGQualityMin > p.PNGQualityMax {
		fields["profile.png_quality_min"] = append(fields["profile.png_quality_min"], errs.ErrKeyFoldersPNGQualityRange)
	}
	for _, pattern := range p.Excludes {
		if pattern == "" || !doublestar.ValidatePattern(pattern) {
			fields["profile.excludes"] = []string{errs.ErrKeyFoldersExcludeInvalid}
			break
		}
	}
	if len(fields) == 0 {
		return nil
	}
	return fields
}

func between(fields errs.Fields, field string, value, lo, hi float64) {
	if value < lo {
		fields.Add(field, "min")
	} else if value > hi {
		fields.Add(field, "max")
	}
}
