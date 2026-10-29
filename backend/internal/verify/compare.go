package verify

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"slices"

	"app/internal/images"
)

// Reason is why a result may not replace its original. The original is then kept as it is.
type Reason string

const (
	ReasonNoSaving          Reason = "no_saving"
	ReasonFormatChanged     Reason = "format_changed"
	ReasonUndecodable       Reason = "result_undecodable"
	ReasonDimensionsChanged Reason = "dimensions_changed"
	ReasonFramesChanged     Reason = "frames_changed"
	ReasonAnimationChanged  Reason = "animation_changed"
	ReasonPixelsChanged     Reason = "pixels_changed"
	ReasonLowSSIM           Reason = "ssim_below_threshold"
	ReasonColorChanged      Reason = "color_changed"
	ReasonMetadataChanged   Reason = "metadata_changed"
)

type Failure struct {
	Reason Reason
	Detail string
}

func (f *Failure) Error() string {
	if f.Detail == "" {
		return string(f.Reason)
	}
	return string(f.Reason) + ": " + f.Detail
}

func fail(reason Reason, format string, args ...any) *Failure {
	return &Failure{Reason: reason, Detail: fmt.Sprintf(format, args...)}
}

// maxColorShift is how far, in 8-bit levels, the average color of a 32-pixel block may move in a
// lossy result. On the calibration corpus no JPEG re-encode at quality 82 moved one more than 2.6,
// and no palette result that passed SSIM more than 10.4 (docs/benchmarks/ssim.md).
const maxColorShift = 12

// Check is what a result is held to.
type Check struct {
	Lossy            bool
	MinSSIM          float64
	MinSavingPercent int
}

// Compare decides whether result may replace original, which has the format given and decoded
// within the pixel limit. Nil means it may. The cheap checks run first: size, format, metadata,
// then the headers, and only then a full decode of both.
func Compare(original, result []byte, format images.Format, c Check) *Failure {
	saved := int64(len(original)) - int64(len(result))
	if saved <= 0 || saved*100 < int64(c.MinSavingPercent)*int64(len(original)) {
		return fail(ReasonNoSaving, "%d bytes to %d", len(original), len(result))
	}
	if got, _ := Sniff(result); got != format {
		return fail(ReasonFormatChanged, "%s became %q", format, got)
	}
	if change := metadataChange(original, result, format); change != "" {
		return &Failure{Reason: ReasonMetadataChanged, Detail: change}
	}
	if format == images.FormatGIF {
		return compareGIF(original, result, c)
	}
	return compareStill(original, result, format, c)
}

func compareStill(original, result []byte, format images.Format, c Check) *Failure {
	decodeConfig, decode := png.DecodeConfig, png.Decode
	if format == images.FormatJPEG {
		decodeConfig, decode = jpeg.DecodeConfig, jpeg.Decode
	}
	a, err := decodeConfig(bytes.NewReader(original))
	if err != nil {
		return fail(ReasonUndecodable, "the original: %v", err)
	}
	b, err := decodeConfig(bytes.NewReader(result))
	if err != nil {
		return fail(ReasonUndecodable, "%v", err)
	}
	if a.Width != b.Width || a.Height != b.Height {
		return fail(ReasonDimensionsChanged, "%dx%d became %dx%d", a.Width, a.Height, b.Width, b.Height)
	}
	before, err := decode(bytes.NewReader(original))
	if err != nil {
		return fail(ReasonUndecodable, "the original: %v", err)
	}
	after, err := decode(bytes.NewReader(result))
	if err != nil {
		return fail(ReasonUndecodable, "%v", err)
	}
	return comparePixels(format, before, after, c)
}

// score is the SSIM the lossy check uses for the format: per color channel, with alpha, for the
// palette formats, whose quantization shifts tints and bands gradients; luma for JPEG.
func score(format images.Format, a, b image.Image) Score {
	if format == images.FormatJPEG {
		return SSIMLuma(a, b)
	}
	return SSIM(a, b)
}

func comparePixels(format images.Format, before, after image.Image, c Check) *Failure {
	if before.Bounds().Size() != after.Bounds().Size() {
		return fail(ReasonDimensionsChanged, "%v became %v", before.Bounds().Size(), after.Bounds().Size())
	}
	if !c.Lossy {
		if x, y, differ := firstDifference(before, after); differ {
			return fail(ReasonPixelsChanged, "first at %d,%d", x, y)
		}
		return nil
	}
	if s := score(format, before, after); s.Worst < c.MinSSIM {
		return fail(ReasonLowSSIM, "%.4f, below %.4f, at %d,%d", s.Worst, c.MinSSIM, s.At.X, s.At.Y)
	}
	if shift := ColorShift(before, after); shift > maxColorShift {
		return fail(ReasonColorChanged, "a block's average color moved %.1f levels", shift)
	}
	return nil
}

// firstDifference compares every pixel as 16-bit non-premultiplied RGBA, so the same colors from
// a palette and from truecolor are equal. Like every color model with alpha, it makes a fully
// transparent pixel equal to any other, whatever color hides under it; no browser shows that
// color.
func firstDifference(a, b image.Image) (int, int, bool) {
	ba, bb := a.Bounds(), b.Bounds()
	if ya, ok := a.(*image.YCbCr); ok {
		if yb, ok := b.(*image.YCbCr); ok && ya.SubsampleRatio == yb.SubsampleRatio && ba.Min == bb.Min {
			if sameYCbCr(ya, yb) {
				return 0, 0, false
			}
		}
	}
	for y := range ba.Dy() {
		for x := range ba.Dx() {
			ca := color.NRGBA64Model.Convert(a.At(ba.Min.X+x, ba.Min.Y+y))
			cb := color.NRGBA64Model.Convert(b.At(bb.Min.X+x, bb.Min.Y+y))
			if ca != cb {
				return x, y, true
			}
		}
	}
	return 0, 0, false
}

func sameYCbCr(a, b *image.YCbCr) bool {
	r := a.Rect
	for y := r.Min.Y; y < r.Max.Y; y++ {
		ya, yb := a.YOffset(r.Min.X, y), b.YOffset(r.Min.X, y)
		if !bytes.Equal(a.Y[ya:ya+r.Dx()], b.Y[yb:yb+r.Dx()]) {
			return false
		}
		for x := r.Min.X; x < r.Max.X; x++ {
			ca, cb := a.COffset(x, y), b.COffset(x, y)
			if a.Cb[ca] != b.Cb[cb] || a.Cr[ca] != b.Cr[cb] {
				return false
			}
		}
	}
	return true
}

// Similarity is the SSIM the lossy check holds a result to: of the decoded images, or for a GIF
// the lowest over its composed frames. For measuring, such as calibrating min_ssim.
func Similarity(original, result []byte, format images.Format) (Score, error) {
	if format != images.FormatGIF {
		decode := png.Decode
		if format == images.FormatJPEG {
			decode = jpeg.Decode
		}
		a, err := decode(bytes.NewReader(original))
		if err != nil {
			return Score{}, err
		}
		b, err := decode(bytes.NewReader(result))
		if err != nil {
			return Score{}, err
		}
		if a.Bounds().Size() != b.Bounds().Size() {
			return Score{}, fmt.Errorf("dimensions changed")
		}
		return score(format, a, b), nil
	}
	a, err := gif.DecodeAll(bytes.NewReader(original))
	if err != nil {
		return Score{}, err
	}
	b, err := gif.DecodeAll(bytes.NewReader(result))
	if err != nil {
		return Score{}, err
	}
	if len(a.Image) != len(b.Image) || a.Config.Width != b.Config.Width || a.Config.Height != b.Config.Height {
		return Score{}, fmt.Errorf("frames changed")
	}
	x, y := newCanvas(a), newCanvas(b)
	lowest := Score{Mean: 1, Worst: 1}
	for range a.Image {
		s := SSIM(x.next(), y.next())
		lowest.Mean = min(lowest.Mean, s.Mean)
		if s.Worst < lowest.Worst {
			lowest.Worst, lowest.At = s.Worst, s.At
		}
	}
	return lowest, nil
}

// MetadataChange names the first rendering metadata of the original the result does not carry
// the same, or "" when there is none.
func MetadataChange(original, result []byte, format images.Format) string {
	return metadataChange(original, result, format)
}

func compareGIF(original, result []byte, c Check) *Failure {
	a, err := walkGIF(original)
	if err != nil {
		return fail(ReasonUndecodable, "the original: %v", err)
	}
	b, err := walkGIF(result)
	if err != nil {
		return fail(ReasonUndecodable, "%v", err)
	}
	if a.width != b.width || a.height != b.height {
		return fail(ReasonDimensionsChanged, "%dx%d became %dx%d", a.width, a.height, b.width, b.height)
	}
	if len(a.frames) != len(b.frames) {
		return fail(ReasonFramesChanged, "%d frames became %d", len(a.frames), len(b.frames))
	}
	before, err := gif.DecodeAll(bytes.NewReader(original))
	if err != nil {
		return fail(ReasonUndecodable, "the original: %v", err)
	}
	after, err := gif.DecodeAll(bytes.NewReader(result))
	if err != nil {
		return fail(ReasonUndecodable, "%v", err)
	}
	if len(before.Image) != len(after.Image) {
		return fail(ReasonFramesChanged, "%d frames became %d", len(before.Image), len(after.Image))
	}
	for i, d := range before.Disposal {
		if d > gif.DisposalPrevious {
			return fail(ReasonAnimationChanged, "frame %d has disposal method %d, which browsers do not agree on", i+1, d)
		}
	}
	if before.LoopCount != after.LoopCount {
		return fail(ReasonAnimationChanged, "loop count %d became %d", before.LoopCount, after.LoopCount)
	}
	if !slices.Equal(before.Delay, after.Delay) {
		return fail(ReasonAnimationChanged, "frame delays changed")
	}
	x, y := newCanvas(before), newCanvas(after)
	for i := range before.Image {
		if f := comparePixels(images.FormatGIF, x.next(), y.next(), c); f != nil {
			f.Detail = fmt.Sprintf("frame %d: %s", i+1, f.Detail)
			return f
		}
	}
	return nil
}

// canvas plays a GIF frame by frame as a browser shows it: each frame drawn over what the previous
// one left, by its disposal method, from a transparent start. Optimizers crop frames to what
// changed and rely on that, so frames compare only after composing.
type canvas struct {
	g     *gif.GIF
	i     int
	shown *image.NRGBA
	saved *image.NRGBA
}

func newCanvas(g *gif.GIF) *canvas {
	r := image.Rect(0, 0, g.Config.Width, g.Config.Height)
	return &canvas{g: g, shown: image.NewNRGBA(r), saved: image.NewNRGBA(r)}
}

// next applies the previous frame's disposal, draws the next frame and returns the canvas.
func (c *canvas) next() *image.NRGBA {
	if c.i > 0 {
		prev := c.g.Image[c.i-1]
		switch c.disposal(c.i - 1) {
		case gif.DisposalBackground:
			erase(c.shown, prev.Rect)
		case gif.DisposalPrevious:
			copy(c.shown.Pix, c.saved.Pix)
		}
	}
	frame := c.g.Image[c.i]
	if c.disposal(c.i) == gif.DisposalPrevious {
		copy(c.saved.Pix, c.shown.Pix)
	}
	r := frame.Rect.Intersect(c.shown.Rect)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			idx := frame.Pix[frame.PixOffset(x, y)]
			if int(idx) >= len(frame.Palette) {
				continue
			}
			col := color.NRGBAModel.Convert(frame.Palette[idx]).(color.NRGBA)
			if col.A == 0 {
				continue
			}
			c.shown.SetNRGBA(x, y, col)
		}
	}
	c.i++
	return c.shown
}

func (c *canvas) disposal(i int) byte {
	if i < len(c.g.Disposal) {
		return c.g.Disposal[i]
	}
	return gif.DisposalNone
}

func erase(m *image.NRGBA, r image.Rectangle) {
	r = r.Intersect(m.Rect)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		row := m.Pix[m.PixOffset(r.Min.X, y):m.PixOffset(r.Max.X, y)]
		for i := range row {
			row[i] = 0
		}
	}
}
