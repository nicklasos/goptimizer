package verify

import (
	"bytes"
	"cmp"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"slices"

	"app/internal/images"
)

// pngColorChunks change how a browser shows a PNG, or at what size it prints. A result must keep
// each one the original has, with the same content. iCCP is compared by its decompressed profile,
// because recompressing the chunk is fine.
var pngColorChunks = []string{"cICP", "iCCP", "sRGB", "gAMA", "cHRM", "pHYs"}

// pngNewChunks may not appear in a result when the original lacks them: an added profile can
// change every color.
var pngNewChunks = []string{"cICP", "iCCP"}

// metadataChange names the first rendering metadata the result does not carry like the original,
// or "" when there is none.
func metadataChange(original, result []byte, format images.Format) string {
	switch format {
	case images.FormatPNG:
		return pngMetadataChange(original, result)
	case images.FormatJPEG:
		a, b := jpegMetadata(original), jpegMetadata(result)
		if !bytes.Equal(a.icc, b.icc) {
			return "ICC profile"
		}
		if a.orientation != b.orientation {
			return fmt.Sprintf("EXIF orientation %d became %d", a.orientation, b.orientation)
		}
	}
	return ""
}

func pngMetadataChange(original, result []byte) string {
	a, b := pngMetadata(original), pngMetadata(result)
	for _, kind := range pngColorChunks {
		before, had := a.chunks[kind]
		after, has := b.chunks[kind]
		switch {
		case had && !has:
			return kind + " dropped"
		case had && !bytes.Equal(before, after):
			return kind + " changed"
		case !had && has && slices.Contains(pngNewChunks, kind):
			return kind + " added"
		}
	}
	if a.orientation != b.orientation {
		return fmt.Sprintf("eXIf orientation %d became %d", a.orientation, b.orientation)
	}
	return ""
}

type pngMeta struct {
	chunks      map[string][]byte
	orientation int
}

func pngMetadata(data []byte) pngMeta {
	m := pngMeta{chunks: map[string][]byte{}, orientation: 1}
	for _, c := range pngChunks(data) {
		switch {
		case c.kind == "iCCP":
			m.chunks[c.kind] = iccpProfile(c.data)
		case c.kind == "eXIf":
			m.orientation = exifOrientation(c.data)
		case slices.Contains(pngColorChunks, c.kind):
			m.chunks[c.kind] = c.data
		}
	}
	return m
}

// iccpProfile is the profile of an iCCP chunk: a name, a NUL, the compression method and the
// zlib stream. An unreadable chunk compares by its raw bytes.
func iccpProfile(data []byte) []byte {
	name := bytes.IndexByte(data, 0)
	if name < 0 || name+2 > len(data) {
		return data
	}
	r, err := zlib.NewReader(bytes.NewReader(data[name+2:]))
	if err != nil {
		return data
	}
	profile, err := io.ReadAll(r)
	if err != nil {
		return data
	}
	return profile
}

type jpegMeta struct {
	icc         []byte
	orientation int
}

// jpegMetadata reads the markers before the first scan: the ICC profile, which APP2 splits into
// numbered pieces, and the EXIF orientation. No orientation tag means 1, upright.
func jpegMetadata(data []byte) jpegMeta {
	m := jpegMeta{orientation: 1}
	type piece struct {
		seq  byte
		data []byte
	}
	var pieces []piece
	for i := 2; i+4 <= len(data) && data[i] == 0xFF; {
		marker := data[i+1]
		if marker == 0xFF {
			i++
			continue
		}
		if marker == 0xD9 || marker == 0xDA {
			break
		}
		n := int(binary.BigEndian.Uint16(data[i+2:]))
		if n < 2 || i+2+n > len(data) {
			break
		}
		segment := data[i+4 : i+2+n]
		switch {
		case marker == 0xE1 && bytes.HasPrefix(segment, []byte("Exif\x00\x00")):
			m.orientation = exifOrientation(segment[6:])
		case marker == 0xE2 && bytes.HasPrefix(segment, []byte("ICC_PROFILE\x00")) && len(segment) >= 14:
			pieces = append(pieces, piece{seq: segment[12], data: segment[14:]})
		}
		i += 2 + n
	}
	slices.SortStableFunc(pieces, func(a, b piece) int { return cmp.Compare(a.seq, b.seq) })
	for _, p := range pieces {
		m.icc = append(m.icc, p.data...)
	}
	return m
}

// exifOrientation reads tag 0x0112 of IFD0 in a TIFF structure. Anything unreadable is 1.
func exifOrientation(tiff []byte) int {
	if len(tiff) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	ifd := int(order.Uint32(tiff[4:]))
	if ifd < 8 || ifd+2 > len(tiff) {
		return 1
	}
	count := int(order.Uint16(tiff[ifd:]))
	for e := 0; e < count; e++ {
		entry := ifd + 2 + 12*e
		if entry+12 > len(tiff) {
			return 1
		}
		if order.Uint16(tiff[entry:]) == 0x0112 {
			return int(order.Uint16(tiff[entry+8:]))
		}
	}
	return 1
}
