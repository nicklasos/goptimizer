package verify

import (
	"image"
	"image/color"
	"image/draw"
	"math"
)

// ssimWindow is the side of the square window SSIM is computed over, at every position.
const ssimWindow = 8

// The constants of Wang et al. (2004) for 8-bit values: (0.01 * 255)² and (0.03 * 255)².
const (
	ssimC1 = 6.5025
	ssimC2 = 58.5225
)

// SSIM scores two images of the same size at full resolution: the lowest Score over the red, green
// and blue channels of the images composited on black and, when either has transparency, on
// white. Compositing makes a change of alpha a change of color, so a pixel that turns transparent
// counts against the result. 1 means identical.
func SSIM(a, b image.Image) Score {
	x, y := toNRGBA(a), toNRGBA(b)
	backgrounds := []uint8{0}
	if !opaque(x) || !opaque(y) {
		backgrounds = append(backgrounds, 255)
	}
	w, h := x.Rect.Dx(), x.Rect.Dy()
	px, py := make([]uint8, w*h), make([]uint8, w*h)
	lowest := Score{Mean: math.Inf(1), Worst: math.Inf(1)}
	for _, bg := range backgrounds {
		for channel := range 3 {
			plane(x, channel, bg, px)
			plane(y, channel, bg, py)
			s := ssimPlane(px, py, w, h)
			lowest.Mean = min(lowest.Mean, s.Mean)
			if s.Worst < lowest.Worst {
				lowest.Worst, lowest.At = s.Worst, s.At
			}
		}
	}
	return lowest
}

// SSIMLuma scores the luma (BT.601) of two opaque images of the same size, for JPEG. A JPEG
// re-encode moves the color channels at the scale of its chroma subsampling, which the eye does
// not resolve, and would dominate a score per color channel.
func SSIMLuma(a, b image.Image) Score {
	x, y := toNRGBA(a), toNRGBA(b)
	w, h := x.Rect.Dx(), x.Rect.Dy()
	px, py := make([]uint8, w*h), make([]uint8, w*h)
	lumaPlane(x, px)
	lumaPlane(y, py)
	return ssimPlane(px, py, w, h)
}

func lumaPlane(m *image.NRGBA, out []uint8) {
	w, h := m.Rect.Dx(), m.Rect.Dy()
	for row := range h {
		pix := m.Pix[row*m.Stride : row*m.Stride+4*w]
		dst := out[row*w : row*w+w]
		for i := range w {
			r, g, b := uint32(pix[4*i]), uint32(pix[4*i+1]), uint32(pix[4*i+2])
			dst[i] = uint8((299*r + 587*g + 114*b + 500) / 1000)
		}
	}
}

// SSIMPlanes scores two 8-bit planes of w by h values, row by row.
func SSIMPlanes(x, y []uint8, w, h int) Score { return ssimPlane(x, y, w, h) }

// plane writes one channel of the image composited on a gray background of value bg.
func plane(m *image.NRGBA, channel int, bg uint8, out []uint8) {
	w, h := m.Rect.Dx(), m.Rect.Dy()
	for row := range h {
		pix := m.Pix[row*m.Stride : row*m.Stride+4*w]
		dst := out[row*w : row*w+w]
		for i := range w {
			c, a := uint32(pix[4*i+channel]), uint32(pix[4*i+3])
			dst[i] = uint8((c*a + uint32(bg)*(255-a) + 127) / 255)
		}
	}
}

// ssimTile is the side of the squares local SSIM is averaged over, besides the whole image.
const ssimTile = 64

// Score is the SSIM of one plane: the mean over every window position, and the lowest mean of a
// ssimTile-square tile, which a small damaged area such as a banded shadow pulls down when the
// whole-image mean hardly moves.
type Score struct {
	Mean  float64
	Worst float64
	// At is the top-left corner of the worst tile.
	At image.Point
}

// ssimPlane slides the window over every position, keeping per-column sums of the window's rows so
// each step adds one row or column and drops one: O(w·h) whatever the window size. A window counts
// towards the tile its top-left corner is in.
func ssimPlane(x, y []uint8, w, h int) Score {
	ww, wh := min(ssimWindow, w), min(ssimWindow, h)
	if ww == 0 || wh == 0 {
		return Score{Mean: 1, Worst: 1}
	}
	n := float64(ww * wh)
	type sums struct{ x, y, xx, yy, xy int64 }
	cols := make([]sums, w)
	add := func(row int, sign int64) {
		for c := range w {
			a, b := int64(x[row*w+c]), int64(y[row*w+c])
			s := &cols[c]
			s.x += sign * a
			s.y += sign * b
			s.xx += sign * a * a
			s.yy += sign * b * b
			s.xy += sign * a * b
		}
	}
	for row := range wh {
		add(row, 1)
	}
	tilesX := (w-ww)/ssimTile + 1
	tiles := make([]float64, tilesX*((h-wh)/ssimTile+1))
	counts := make([]int, len(tiles))
	total, count := 0.0, 0
	for top := 0; ; top++ {
		var win sums
		for c := range ww {
			win.x += cols[c].x
			win.y += cols[c].y
			win.xx += cols[c].xx
			win.yy += cols[c].yy
			win.xy += cols[c].xy
		}
		rowTiles := (top / ssimTile) * tilesX
		for left := 0; ; left++ {
			mx, my := float64(win.x)/n, float64(win.y)/n
			vx := float64(win.xx)/n - mx*mx
			vy := float64(win.yy)/n - my*my
			cov := float64(win.xy)/n - mx*my
			s := (2*mx*my + ssimC1) * (2*cov + ssimC2) / ((mx*mx + my*my + ssimC1) * (vx + vy + ssimC2))
			total += s
			count++
			t := rowTiles + left/ssimTile
			tiles[t] += s
			counts[t]++
			if left+ww >= w {
				break
			}
			out, in := cols[left], cols[left+ww]
			win.x += in.x - out.x
			win.y += in.y - out.y
			win.xx += in.xx - out.xx
			win.yy += in.yy - out.yy
			win.xy += in.xy - out.xy
		}
		if top+wh >= h {
			break
		}
		add(top, -1)
		add(top+wh, 1)
	}
	score := Score{Mean: total / float64(count), Worst: 1}
	for i, sum := range tiles {
		if counts[i] > 0 && sum/float64(counts[i]) < score.Worst {
			score.Worst = sum / float64(counts[i])
			score.At = image.Point{X: (i % tilesX) * ssimTile, Y: (i / tilesX) * ssimTile}
		}
	}
	return score
}

func opaque(m *image.NRGBA) bool {
	w := m.Rect.Dx()
	for row := range m.Rect.Dy() {
		pix := m.Pix[row*m.Stride : row*m.Stride+4*w]
		for i := 3; i < len(pix); i += 4 {
			if pix[i] != 0xFF {
				return false
			}
		}
	}
	return true
}

// toNRGBA is the image as 8-bit non-premultiplied RGBA with its origin at 0, 0. 16-bit samples
// keep their high byte.
func toNRGBA(m image.Image) *image.NRGBA {
	if n, ok := m.(*image.NRGBA); ok && n.Rect.Min == (image.Point{}) {
		return n
	}
	b := m.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	switch src := m.(type) {
	case *image.YCbCr, *image.Gray, *image.RGBA:
		if isOpaqueType(src) {
			rgba := &image.RGBA{Pix: out.Pix, Stride: out.Stride, Rect: out.Rect}
			draw.Draw(rgba, rgba.Rect, src, b.Min, draw.Src)
			return out
		}
	case *image.Paletted:
		palette := make([]color.NRGBA, len(src.Palette))
		for i, c := range src.Palette {
			palette[i] = color.NRGBAModel.Convert(c).(color.NRGBA)
		}
		for row := range b.Dy() {
			idx := src.Pix[row*src.Stride : row*src.Stride+b.Dx()]
			dst := out.Pix[row*out.Stride:]
			for i, p := range idx {
				c := color.NRGBA{}
				if int(p) < len(palette) {
					c = palette[p]
				}
				dst[4*i], dst[4*i+1], dst[4*i+2], dst[4*i+3] = c.R, c.G, c.B, c.A
			}
		}
		return out
	}
	for yy := range b.Dy() {
		for xx := range b.Dx() {
			out.SetNRGBA(xx, yy, color.NRGBAModel.Convert(m.At(b.Min.X+xx, b.Min.Y+yy)).(color.NRGBA))
		}
	}
	return out
}

// isOpaqueType is a decoder type whose pixels are all opaque, so premultiplied and straight alpha
// agree and draw's fast paths give the exact values. Go's PNG decoder returns *image.RGBA only for
// images without alpha.
func isOpaqueType(m image.Image) bool {
	switch src := m.(type) {
	case *image.YCbCr, *image.Gray:
		return true
	case *image.RGBA:
		return src.Opaque()
	}
	return false
}
