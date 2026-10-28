package folders

import (
	"encoding/json"

	"app/internal/errs"

	"github.com/bmatcuk/doublestar/v4"
)

type Mode string

const (
	ModeLossy    Mode = "lossy"
	ModeLossless Mode = "lossless"
)

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
		// A placeholder until phase 4 measures SSIM on real photos, screenshots and illustrations.
		MinSSIM:       0.98,
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
