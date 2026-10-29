package unit

import (
	"image"
	"image/color"
	"testing"

	"app/internal/verify"

	"github.com/stretchr/testify/assert"
)

// The expected values come from the SSIM formula of Wang et al. (2004) with C1 = (0.01·255)² and
// C2 = (0.03·255)², worked by hand for windows whose means, variances and covariance are known.
const (
	c1 = 6.5025
	c2 = 58.5225
)

func plane(w, h int, value func(x, y int) uint8) []uint8 {
	p := make([]uint8, w*h)
	for y := range h {
		for x := range w {
			p[y*w+x] = value(x, y)
		}
	}
	return p
}

func TestSSIM_KnownValues(t *testing.T) {
	constant := func(v uint8) func(int, int) uint8 { return func(int, int) uint8 { return v } }
	checker := func(x, y int) uint8 {
		if (x+y)%2 == 0 {
			return 0
		}
		return 200
	}
	inverse := func(x, y int) uint8 { return 200 - checker(x, y) }

	for _, tc := range []struct {
		name string
		x, y func(int, int) uint8
		want float64
	}{
		{"identical", checker, checker, 1},
		// Constant windows: only the luminance term is left, (2·100·110 + C1) / (100² + 110² + C1).
		{"two flat grays", constant(100), constant(110), (2*100*110 + c1) / (100*100 + 110*110 + c1)},
		// Means 100 and 100, variances 10000 and 0, covariance 0.
		{"texture against flat", checker, constant(100), c2 / (10000 + c2)},
		// Same means and variances, covariance -10000.
		{"inverted texture", checker, inverse, (-20000 + c2) / (20000 + c2)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			score := verify.SSIMPlanes(plane(8, 8, tc.x), plane(8, 8, tc.y), 8, 8)
			assert.InDelta(t, tc.want, score.Mean, 1e-9)
			assert.InDelta(t, tc.want, score.Worst, 1e-9, "one window is one tile")
		})
	}
}

func TestSSIM_WorstTileFindsDamageTheMeanHides(t *testing.T) {
	const w, h = 512, 512
	ramp := func(x, y int) uint8 { return uint8(x / 2) }
	banded := func(x, y int) uint8 {
		if x >= 256 && y >= 256 && x < 320 && y < 320 {
			return uint8(x/2) &^ 15
		}
		return ramp(x, y)
	}

	score := verify.SSIMPlanes(plane(w, h, ramp), plane(w, h, banded), w, h)

	assert.Greater(t, score.Mean, 0.99, "one banded tile in 64 barely moves the mean")
	assert.Less(t, score.Worst, 0.9, "the banded tile itself scores low")
	assert.Equal(t, image.Point{X: 256, Y: 256}, score.At)
}

func TestSSIM_TransparencyCounts(t *testing.T) {
	a := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	b := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			c := color.NRGBA{uint8(4 * x), uint8(4 * y), 90, 255}
			a.SetNRGBA(x, y, c)
			if x < 32 {
				c.A = 0
			}
			b.SetNRGBA(x, y, c)
		}
	}

	assert.Equal(t, 1.0, verify.SSIM(a, a).Worst)
	assert.Less(t, verify.SSIM(a, b).Worst, 0.5, "the same colors made transparent are a difference")
}

func TestSSIM_ColorShiftCountsForPalettesButNotForJPEGLuma(t *testing.T) {
	a := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	b := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			a.SetNRGBA(x, y, color.NRGBA{255, 255, 255, 255})
			b.SetNRGBA(x, y, color.NRGBA{255, 255, 255, 255})
			if x > 16 && x < 48 && y > 24 && y < 40 {
				a.SetNRGBA(x, y, color.NRGBA{255, 240, 240, 255})
				b.SetNRGBA(x, y, color.NRGBA{244, 244, 244, 255})
			}
		}
	}

	assert.Less(t, verify.SSIM(a, b).Worst, 0.97, "a pale pink tag that turned gray counts per channel")
	assert.Greater(t, verify.SSIMLuma(a, b).Worst, 0.999, "its luma stayed the same")
}
