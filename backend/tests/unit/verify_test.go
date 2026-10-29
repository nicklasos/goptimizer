package unit

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"

	"app/internal/images"
	"app/internal/verify"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var lossless = verify.Check{MinSavingPercent: 3}
var lossy = verify.Check{Lossy: true, MinSSIM: 0.85, MinSavingPercent: 3}

func gradient(w, h int) *image.NRGBA {
	m := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			m.SetNRGBA(x, y, color.NRGBA{uint8(x * 255 / w), uint8(y * 255 / h), 128, 255})
		}
	}
	return m
}

func pngOf(t *testing.T, m image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&buf, m))
	return buf.Bytes()
}

func jpegOf(t *testing.T, m image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, m, &jpeg.Options{Quality: 90}))
	return buf.Bytes()
}

// withChunks puts chunks right after IHDR, as encoders write the color chunks.
func withChunks(data []byte, chunks ...[]byte) []byte {
	out := append([]byte{}, data[:33]...)
	for _, c := range chunks {
		out = append(out, c...)
	}
	return append(out, data[33:]...)
}

func chunk(kind string, body []byte) []byte {
	c := binary.BigEndian.AppendUint32(nil, uint32(len(body)))
	c = append(c, kind...)
	c = append(c, body...)
	return binary.BigEndian.AppendUint32(c, crc32.ChecksumIEEE(c[4:]))
}

func iccp(profile string) []byte {
	var z bytes.Buffer
	w := zlib.NewWriter(&z)
	_, _ = w.Write([]byte(profile))
	_ = w.Close()
	return chunk("iCCP", append([]byte("icc\x00\x00"), z.Bytes()...))
}

func padding() []byte { return chunk("tEXt", []byte("Comment\x00"+strings.Repeat("x", 4000))) }

// withSegments puts JPEG segments right after SOI.
func withSegments(data []byte, segments ...[]byte) []byte {
	out := append([]byte{}, data[:2]...)
	for _, s := range segments {
		out = append(out, s...)
	}
	return append(out, data[2:]...)
}

func segment(marker byte, body []byte) []byte {
	return append([]byte{0xFF, marker, byte((len(body) + 2) >> 8), byte(len(body) + 2)}, body...)
}

func exifOrientation(o uint16) []byte {
	tiff := []byte("MM\x00\x2a\x00\x00\x00\x08\x00\x01\x01\x12\x00\x03\x00\x00\x00\x01")
	tiff = binary.BigEndian.AppendUint16(tiff, o)
	tiff = append(tiff, 0, 0, 0, 0, 0, 0)
	return segment(0xE1, append([]byte("Exif\x00\x00"), tiff...))
}

func iccSegment(profile string) []byte {
	return segment(0xE2, append([]byte("ICC_PROFILE\x00\x01\x01"), profile...))
}

func comSegment() []byte { return segment(0xFE, []byte(strings.Repeat("c", 4000))) }

func TestCompare_Refusals(t *testing.T) {
	base := gradient(64, 48)
	original := pngOf(t, base)
	padded := withChunks(original, padding())
	changedPixel := image.NewNRGBA(base.Rect)
	copy(changedPixel.Pix, base.Pix)
	changedPixel.SetNRGBA(10, 10, color.NRGBA{0, 0, 0, 255})
	smaller := gradient(64, 47)
	banded := image.NewNRGBA(base.Rect)
	for i := range banded.Pix {
		banded.Pix[i] = base.Pix[i] &^ 63
		if i%4 == 3 {
			banded.Pix[i] = 255
		}
	}

	jpegBase := jpegOf(t, gradient(64, 48))
	icc := strings.Repeat("profile", 50)

	for _, tc := range []struct {
		name     string
		original []byte
		result   []byte
		format   images.Format
		check    verify.Check
		want     verify.Reason
	}{
		{"not smaller", original, original, images.FormatPNG, lossless, verify.ReasonNoSaving},
		{"smaller by less than the minimum", padded, padded[:len(padded)-50], images.FormatPNG, lossless, verify.ReasonNoSaving},
		{"another format", padded, []byte("GIF89a, not a PNG"), images.FormatPNG, lossless, verify.ReasonFormatChanged},
		{"a PNG that does not decode", padded, append(append([]byte{}, original[:40]...), 0, 1, 2), images.FormatPNG, lossless, verify.ReasonUndecodable},
		{"other dimensions", padded, pngOf(t, smaller), images.FormatPNG, lossless, verify.ReasonDimensionsChanged},
		{"one pixel changed in lossless", padded, pngOf(t, changedPixel), images.FormatPNG, lossless, verify.ReasonPixelsChanged},
		{"banding in lossy", padded, pngOf(t, banded), images.FormatPNG, lossy, verify.ReasonLowSSIM},
		{"iCCP dropped", withChunks(original, iccp(icc), padding()), original, images.FormatPNG, lossless, verify.ReasonMetadataChanged},
		{"iCCP changed", withChunks(original, iccp(icc), padding()), withChunks(original, iccp("other")), images.FormatPNG, lossless, verify.ReasonMetadataChanged},
		{"gAMA dropped", withChunks(original, chunk("gAMA", []byte{0, 0, 0xB1, 0x8F}), padding()), original, images.FormatPNG, lossless, verify.ReasonMetadataChanged},
		{"cHRM changed", withChunks(original, chunk("cHRM", make([]byte, 32)), padding()), withChunks(original, chunk("cHRM", bytes.Repeat([]byte{1}, 32))), images.FormatPNG, lossless, verify.ReasonMetadataChanged},
		{"pHYs dropped", withChunks(original, chunk("pHYs", []byte{0, 0, 0x0B, 0x13, 0, 0, 0x0B, 0x13, 1}), padding()), original, images.FormatPNG, lossless, verify.ReasonMetadataChanged},
		{"sRGB dropped", withChunks(original, chunk("sRGB", []byte{0}), padding()), original, images.FormatPNG, lossless, verify.ReasonMetadataChanged},
		{"iCCP added", padded, withChunks(original, iccp(icc)), images.FormatPNG, lossless, verify.ReasonMetadataChanged},
		{"JPEG ICC profile dropped", withSegments(jpegBase, iccSegment(icc), comSegment()), jpegBase, images.FormatJPEG, lossless, verify.ReasonMetadataChanged},
		{"JPEG orientation dropped", withSegments(jpegBase, exifOrientation(6), comSegment()), jpegBase, images.FormatJPEG, lossless, verify.ReasonMetadataChanged},
		{"JPEG orientation changed", withSegments(jpegBase, exifOrientation(6), comSegment()), withSegments(jpegBase, exifOrientation(1)), images.FormatJPEG, lossless, verify.ReasonMetadataChanged},
	} {
		t.Run(tc.name, func(t *testing.T) {
			failure := verify.Compare(tc.original, tc.result, tc.format, tc.check)
			require.NotNil(t, failure)
			assert.Equal(t, tc.want, failure.Reason, failure.Error())
		})
	}
}

func TestCompare_Accepts(t *testing.T) {
	base := gradient(64, 48)
	original := pngOf(t, base)
	padded := withChunks(original, padding())
	icc := strings.Repeat("profile", 50)
	jpegBase := jpegOf(t, gradient(64, 48))

	palette := color.Palette{}
	gray := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := range 32 {
		for x := range 32 {
			c := color.NRGBA{uint8(8 * x), uint8(8 * x), uint8(8 * x), 255}
			gray.SetNRGBA(x, y, c)
			if y == 0 {
				palette = append(palette, c)
			}
		}
	}
	paletted := image.NewPaletted(gray.Rect, palette)
	draw.Draw(paletted, gray.Rect, gray, image.Point{}, draw.Src)

	for _, tc := range []struct {
		name     string
		original []byte
		result   []byte
		format   images.Format
		check    verify.Check
	}{
		{"the same pixels, smaller", padded, original, images.FormatPNG, lossless},
		{"truecolor to a palette of the same colors", withChunks(pngOf(t, gray), padding()), pngOf(t, paletted), images.FormatPNG, lossless},
		{"the same metadata, recompressed", withChunks(original, iccp(icc), chunk("gAMA", []byte{0, 0, 0xB1, 0x8F}), padding()), withChunks(original, iccp(icc), chunk("gAMA", []byte{0, 0, 0xB1, 0x8F})), images.FormatPNG, lossless},
		{"an sRGB chunk added to an untagged PNG", padded, withChunks(original, chunk("sRGB", []byte{0})), images.FormatPNG, lossless},
		{"a JPEG keeping its profile and orientation", withSegments(jpegBase, exifOrientation(6), iccSegment(icc), comSegment()), withSegments(jpegBase, exifOrientation(6), iccSegment(icc)), images.FormatJPEG, lossless},
	} {
		t.Run(tc.name, func(t *testing.T) {
			failure := verify.Compare(tc.original, tc.result, tc.format, tc.check)
			assert.Nil(t, failure, "%v", failure)
		})
	}
}

// animated builds a GIF from full frames, or with every frame after the first cropped to what
// changed and drawn over the previous one, as gifsicle -O3 writes it.
func animated(t *testing.T, frames []*image.Paletted, delays []int, loop int) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, gif.EncodeAll(&buf, &gif.GIF{Image: frames, Delay: delays, LoopCount: loop}))
	return buf.Bytes()
}

func TestCompare_GIF(t *testing.T) {
	palette := color.Palette{color.RGBA{0, 0, 0, 255}, color.RGBA{255, 255, 255, 255}, color.RGBA{220, 20, 20, 255}, color.RGBA{}}
	full := func(i int) *image.Paletted {
		f := image.NewPaletted(image.Rect(0, 0, 40, 30), palette)
		draw.Draw(f, f.Rect, &image.Uniform{palette[1]}, image.Point{}, draw.Src)
		draw.Draw(f, image.Rect(5*i, 5, 5*i+10, 15), &image.Uniform{palette[2]}, image.Point{}, draw.Src)
		return f
	}
	var frames []*image.Paletted
	for i := range 4 {
		frames = append(frames, full(i))
	}
	original := animated(t, frames, []int{10, 10, 10, 10}, 0)
	original = append(original[:len(original)-1], append(bytes.Repeat([]byte{0x21, 0xFE, 0x10}, 1), append(bytes.Repeat([]byte{'c'}, 16), 0, 0x3B)...)...)

	cropped := []*image.Paletted{frames[0]}
	for i := 1; i < 4; i++ {
		r := image.Rect(5*(i-1), 5, 5*i+10, 15)
		f := image.NewPaletted(r, palette)
		draw.Draw(f, r, frames[i], r.Min, draw.Src)
		cropped = append(cropped, f)
	}

	assert.Nil(t, verify.Compare(original, animated(t, cropped, []int{10, 10, 10, 10}, 0), images.FormatGIF, lossless), "cropped frames that compose to the same pictures")

	for _, tc := range []struct {
		name   string
		result []byte
		want   verify.Reason
	}{
		{"a frame fewer", animated(t, frames[:3], []int{10, 10, 10}, 0), verify.ReasonFramesChanged},
		{"another loop count", animated(t, frames, []int{10, 10, 10, 10}, 3), verify.ReasonAnimationChanged},
		{"other delays", animated(t, frames, []int{10, 20, 10, 10}, 0), verify.ReasonAnimationChanged},
		{"a frame drawn elsewhere", animated(t, append(append([]*image.Paletted{}, frames[:3]...), full(0)), []int{10, 10, 10, 10}, 0), verify.ReasonPixelsChanged},
	} {
		t.Run(tc.name, func(t *testing.T) {
			failure := verify.Compare(original, tc.result, images.FormatGIF, lossless)
			require.NotNil(t, failure)
			assert.Equal(t, tc.want, failure.Reason, failure.Error())
		})
	}
}

func TestSniffAndAPNG(t *testing.T) {
	original := pngOf(t, gradient(8, 8))
	format, ok := verify.Sniff(original)
	assert.True(t, ok)
	assert.Equal(t, images.FormatPNG, format)
	format, _ = verify.Sniff(jpegOf(t, gradient(8, 8)))
	assert.Equal(t, images.FormatJPEG, format)
	_, ok = verify.Sniff([]byte("<svg/>"))
	assert.False(t, ok)

	assert.False(t, verify.IsAPNG(original))
	assert.True(t, verify.IsAPNG(withChunks(original, chunk("acTL", []byte{0, 0, 0, 2, 0, 0, 0, 0}))))
}
