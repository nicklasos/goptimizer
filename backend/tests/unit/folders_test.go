package unit

import (
	"encoding/json"
	"testing"

	"app/internal/errs"
	"app/internal/folders"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFolders_Overlaps(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"/srv/a", "/srv/a", true},
		{"/srv/a", "/srv/a/b", true},
		{"/srv/a/b", "/srv/a", true},
		{"/srv/a", "/srv/a/b/c/d", true},
		{"/srv/a", "/srv/ab", false},
		{"/srv/ab", "/srv/a", false},
		{"/srv/a", "/srv/b", false},
		{"/srv/a/b", "/srv/a/c", false},
		{"/srv", "/srvx/a", false},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, folders.Overlaps(c.a, c.b), "%s and %s", c.a, c.b)
	}
}

func TestFolders_ProfileDefaults(t *testing.T) {
	t.Run("an empty profile is the default one", func(t *testing.T) {
		var profile folders.Profile
		require.NoError(t, json.Unmarshal([]byte(`{}`), &profile))

		assert.Equal(t, folders.Profile{
			Mode:             folders.ModeLossy,
			JPEGMaxQuality:   82,
			PNGQualityMin:    65,
			PNGQualityMax:    80,
			GIFLossy:         30,
			MinSavingPercent: 3,
			MinSSIM:          0.85,
			MinAgeMinutes:    5,
			Excludes:         []string{},
		}, profile)
		assert.Nil(t, profile.Validate())
	})

	t.Run("given fields are kept and the rest default", func(t *testing.T) {
		var profile folders.Profile
		require.NoError(t, json.Unmarshal([]byte(`{"mode": "lossless", "excludes": ["cache/**"]}`), &profile))

		assert.Equal(t, folders.ModeLossless, profile.Mode)
		assert.Equal(t, []string{"cache/**"}, profile.Excludes)
		assert.Equal(t, 82, profile.JPEGMaxQuality)
		assert.Equal(t, 0.85, profile.MinSSIM)
	})

	t.Run("a request without a profile or intervals gets the defaults", func(t *testing.T) {
		var req folders.FolderRequest
		require.NoError(t, json.Unmarshal([]byte(`{"name": "Site", "path": "/srv/site"}`), &req))

		assert.Equal(t, folders.KindLocal, req.Kind)
		assert.EqualValues(t, 60, req.ScanIntervalMinutes)
		assert.EqualValues(t, 24, req.FullScanIntervalHours)
		assert.Equal(t, folders.DefaultProfile(), req.Profile)
	})
}

func TestFolders_ProfileValidate(t *testing.T) {
	cases := []struct {
		name  string
		edit  func(*folders.Profile)
		field string
		key   string
	}{
		{"unknown mode", func(p *folders.Profile) { p.Mode = "extreme" }, "profile.mode", "validation.profile.mode.oneof"},
		{"jpeg quality too low", func(p *folders.Profile) { p.JPEGMaxQuality = 39 }, "profile.jpeg_max_quality", "validation.profile.jpeg_max_quality.min"},
		{"jpeg quality too high", func(p *folders.Profile) { p.JPEGMaxQuality = 101 }, "profile.jpeg_max_quality", "validation.profile.jpeg_max_quality.max"},
		{"png min above 100", func(p *folders.Profile) { p.PNGQualityMin, p.PNGQualityMax = 101, 101 }, "profile.png_quality_min", "validation.profile.png_quality_min.max"},
		{"png min above max", func(p *folders.Profile) { p.PNGQualityMin, p.PNGQualityMax = 81, 80 }, "profile.png_quality_min", errs.ErrKeyFoldersPNGQualityRange},
		{"gif lossy too high", func(p *folders.Profile) { p.GIFLossy = 201 }, "profile.gif_lossy", "validation.profile.gif_lossy.max"},
		{"saving too high", func(p *folders.Profile) { p.MinSavingPercent = 91 }, "profile.min_saving_percent", "validation.profile.min_saving_percent.max"},
		{"ssim too low", func(p *folders.Profile) { p.MinSSIM = 0.49 }, "profile.min_ssim", "validation.profile.min_ssim.min"},
		{"ssim above one", func(p *folders.Profile) { p.MinSSIM = 1.01 }, "profile.min_ssim", "validation.profile.min_ssim.max"},
		{"negative age", func(p *folders.Profile) { p.MinAgeMinutes = -1 }, "profile.min_age_minutes", "validation.profile.min_age_minutes.min"},
		{"age over a day", func(p *folders.Profile) { p.MinAgeMinutes = 1441 }, "profile.min_age_minutes", "validation.profile.min_age_minutes.max"},
		{"broken glob", func(p *folders.Profile) { p.Excludes = []string{"cache/**", "[a-"} }, "profile.excludes", errs.ErrKeyFoldersExcludeInvalid},
		{"empty glob", func(p *folders.Profile) { p.Excludes = []string{""} }, "profile.excludes", errs.ErrKeyFoldersExcludeInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			profile := folders.DefaultProfile()
			c.edit(&profile)
			assert.Contains(t, profile.Validate()[c.field], c.key)
		})
	}

	t.Run("the edges of every range pass", func(t *testing.T) {
		profile := folders.Profile{
			Mode: folders.ModeLossless, JPEGMaxQuality: 40, PNGQualityMin: 0, PNGQualityMax: 0, GIFLossy: 200,
			MinSavingPercent: 90, MinSSIM: 0.5, MinAgeMinutes: 1440, Excludes: []string{"cache/**", "*.tmp.png", "uploads/{a,b}/*"},
		}
		assert.Nil(t, profile.Validate())
	})
}
