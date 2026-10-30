// Package verify decides whether an optimized image may replace its original. The result must be
// smaller by the folder's minimum saving, decode with Go's own decoders in the same format and
// geometry, keep the metadata browsers render with, and have the same pixels (lossless) or an
// SSIM at or above the folder's threshold (lossy). It also reads what an original is before any
// tool runs: its real format, whether it is animated, and how many pixels a decode would allocate.
package verify

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image/jpeg"
	"image/png"

	"app/internal/images"
)

var (
	pngMagic  = []byte("\x89PNG\r\n\x1a\n")
	jpegMagic = []byte{0xFF, 0xD8, 0xFF}
)

// Sniff is the format the content has, by its first bytes.
func Sniff(data []byte) (images.Format, bool) {
	switch {
	case bytes.HasPrefix(data, pngMagic):
		return images.FormatPNG, true
	case bytes.HasPrefix(data, jpegMagic):
		return images.FormatJPEG, true
	case bytes.HasPrefix(data, []byte("GIF87a")), bytes.HasPrefix(data, []byte("GIF89a")):
		return images.FormatGIF, true
	}
	return "", false
}

// IsAPNG reports an animated PNG: an acTL chunk before the first IDAT.
func IsAPNG(data []byte) bool {
	for _, c := range pngChunks(data) {
		switch c.kind {
		case "acTL":
			return true
		case "IDAT":
			return false
		}
	}
	return false
}

var errMalformed = errors.New("malformed image")

// Pixels is how many pixels decoding the image allocates, read from its headers without decoding:
// width times height, and for a GIF the sum over its frames.
func Pixels(data []byte, format images.Format) (int64, error) {
	switch format {
	case images.FormatPNG:
		cfg, err := png.DecodeConfig(bytes.NewReader(data))
		return int64(cfg.Width) * int64(cfg.Height), err
	case images.FormatJPEG:
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
		return int64(cfg.Width) * int64(cfg.Height), err
	case images.FormatGIF:
		g, err := walkGIF(data)
		if err != nil {
			return 0, err
		}
		total := int64(g.width) * int64(g.height)
		for _, f := range g.frames {
			total += int64(f.width) * int64(f.height)
		}
		return total, nil
	}
	return 0, errMalformed
}

type chunk struct {
	kind string
	data []byte
}

// pngChunks lists the chunks after the signature, stopping at the first one that does not fit.
func pngChunks(data []byte) []chunk {
	var chunks []chunk
	for i := len(pngMagic); i+12 <= len(data); {
		n := int(binary.BigEndian.Uint32(data[i:]))
		if n < 0 || i+12+n > len(data) {
			break
		}
		kind := string(data[i+4 : i+8])
		chunks = append(chunks, chunk{kind: kind, data: data[i+8 : i+8+n]})
		if kind == "IEND" {
			break
		}
		i += 12 + n
	}
	return chunks
}

// gifInfo is a GIF's structure, read without decoding any frame.
type gifInfo struct {
	width, height int
	frames        []gifFrame
	// end is the position after the trailer.
	end int
}

type gifFrame struct{ width, height int }

func walkGIF(data []byte) (gifInfo, error) {
	if len(data) < 13 {
		return gifInfo{}, errMalformed
	}
	g := gifInfo{width: int(binary.LittleEndian.Uint16(data[6:])), height: int(binary.LittleEndian.Uint16(data[8:]))}
	i := 13
	if flags := data[10]; flags&0x80 != 0 {
		i += 3 << (flags&7 + 1)
	}
	skipBlocks := func() bool {
		for i < len(data) {
			n := int(data[i])
			i++
			if n == 0 {
				return true
			}
			i += n
		}
		return false
	}
	for i < len(data) {
		switch data[i] {
		case 0x3B:
			g.end = i + 1
			return g, nil
		case 0x21:
			i += 2
			if !skipBlocks() {
				return g, errMalformed
			}
		case 0x2C:
			if i+10 > len(data) {
				return g, errMalformed
			}
			g.frames = append(g.frames, gifFrame{
				width:  int(binary.LittleEndian.Uint16(data[i+5:])),
				height: int(binary.LittleEndian.Uint16(data[i+7:])),
			})
			flags := data[i+9]
			i += 10
			if flags&0x80 != 0 {
				i += 3 << (flags&7 + 1)
			}
			i++
			if !skipBlocks() {
				return g, errMalformed
			}
		default:
			return g, errMalformed
		}
	}
	return g, errMalformed
}

// Strip is the image without the data it carries beyond what a browser shows: bytes after the end
// of the image (a motion photo's video, an Ultra HDR gain map, the second picture of an MPF file,
// or anything appended to it), a JPEG's MPF index and JUMBF boxes, and a PNG's caBX chunks (both
// content credentials). The tools are given the stripped image, so none of it survives
// optimizing. A file whose structure cannot be walked comes back as it is.
func Strip(data []byte, format images.Format) []byte {
	switch format {
	case images.FormatPNG:
		return pngStrip(data)
	case images.FormatJPEG:
		return jpegStrip(data)
	case images.FormatGIF:
		if g, err := walkGIF(data); err == nil {
			return data[:g.end]
		}
	}
	return data
}

func pngStrip(data []byte) []byte {
	out := append([]byte{}, pngMagic...)
	for _, c := range pngChunks(data) {
		if c.kind == "caBX" {
			continue
		}
		out = binary.BigEndian.AppendUint32(out, uint32(len(c.data)))
		out = append(out, c.kind...)
		out = append(out, c.data...)
		out = binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(append([]byte(c.kind), c.data...)))
		if c.kind == "IEND" {
			return out
		}
	}
	return data
}

// jpegStrip walks the segments and the entropy-coded data of every scan to the end of the primary
// image, since an appended video or gain map can contain an end marker of its own.
func jpegStrip(data []byte) []byte {
	out := append([]byte{}, data[:min(2, len(data))]...)
	i := 2
	for i+2 <= len(data) {
		if data[i] != 0xFF {
			return data
		}
		marker := data[i+1]
		switch {
		case marker == 0xFF:
			i++
			continue
		case marker == 0xD9:
			return append(out, 0xFF, 0xD9)
		case marker >= 0xD0 && marker <= 0xD7, marker == 0x01:
			out = append(out, data[i:i+2]...)
			i += 2
			continue
		}
		if i+4 > len(data) {
			return data
		}
		n := int(binary.BigEndian.Uint16(data[i+2:]))
		if n < 2 || i+2+n > len(data) {
			return data
		}
		segment := data[i+4 : i+2+n]
		mpf := marker == 0xE2 && bytes.HasPrefix(segment, []byte("MPF\x00"))
		if !mpf && marker != 0xEB {
			out = append(out, data[i:i+2+n]...)
		}
		i += 2 + n
		if marker == 0xDA {
			end := skipEntropyCoded(data, i)
			out = append(out, data[i:end]...)
			i = end
		}
	}
	return data
}

// skipEntropyCoded returns the position of the first marker after a scan's data: 0xFF followed by
// anything but a stuffed 0x00, a fill byte or a restart marker.
func skipEntropyCoded(data []byte, i int) int {
	for i+1 < len(data) {
		if data[i] == 0xFF {
			next := data[i+1]
			if next != 0x00 && next != 0xFF && (next < 0xD0 || next > 0xD7) {
				return i
			}
		}
		i++
	}
	return len(data)
}
