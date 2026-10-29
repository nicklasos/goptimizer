// Command ssimcal runs the lossy pipelines on a corpus of images and prints the SSIM of each
// result, the number the lossy check compares with min_ssim, so the threshold can be set from
// real images. The corpus is a directory of subdirectories, one per kind of image. With
// -generate it first writes the synthetic gradients that band under quantization. Results go to
// -out, next to crops of the region where each result differs most, to look at side by side.
//
//	go run ./cmd/ssimcal -corpus /tmp/corpus -out /tmp/ssim -generate
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io/fs"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"app/internal/folders"
	"app/internal/images"
	"app/internal/optimizer"
	"app/internal/tools"
	"app/internal/verify"
)

type result struct {
	path       string
	kind       string
	format     images.Format
	before     int
	after      int
	ssim       verify.Score
	luma       verify.Score
	ycc        verify.Score
	tileAt     image.Point
	metadata   string
	color      float64
	err        string
	resultPath string
}

func main() {
	corpus := flag.String("corpus", "", "directory of image directories")
	out := flag.String("out", "", "directory for the results and crops")
	generate := flag.Bool("generate", false, "write the synthetic gradients into <corpus>/generated first")
	compare := flag.String("compare", "", "instead, score each file named after the flags against this original")
	mode := flag.String("mode", "lossy", "the pipelines to run: lossy or lossless")
	flag.Parse()
	if *compare != "" {
		scoreAgainst(*compare, flag.Args())
		return
	}
	if *corpus == "" || *out == "" {
		log.Fatal("-corpus and -out are required")
	}
	if *generate {
		if err := writeGenerated(filepath.Join(*corpus, "generated")); err != nil {
			log.Fatal(err)
		}
	}
	ctx := context.Background()
	report := tools.Check(ctx, tools.System{})
	profile := folders.DefaultProfile()
	profile.Mode = folders.Mode(*mode)
	work := filepath.Join(*out, "work")
	if err := os.MkdirAll(work, 0o700); err != nil {
		log.Fatal(err)
	}

	var results []result
	err := filepath.WalkDir(*corpus, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		format, ok := images.FormatOf(path)
		if !ok {
			return nil
		}
		rel, _ := filepath.Rel(*corpus, path)
		r := result{path: rel, kind: filepath.Dir(rel), format: format}
		original, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		r.before = len(original)
		steps, ok := optimizer.Pipeline(format, profile, report)
		if !ok {
			return fmt.Errorf("a tool for %s is missing", format)
		}
		optimized, err := optimizer.RunPipeline(ctx, work, steps, profile, false, time.Minute, filepath.Ext(path), original)
		if err != nil {
			r.err = err.Error()
			results = append(results, r)
			return nil
		}
		r.after = len(optimized)
		r.metadata = verify.MetadataChange(original, optimized, format)
		if format != images.FormatGIF {
			if a, _, err := image.Decode(bytes.NewReader(original)); err == nil {
				if b, _, err := image.Decode(bytes.NewReader(optimized)); err == nil && a.Bounds().Size() == b.Bounds().Size() {
					r.color = verify.ColorShift(a, b)
				}
			}
		}
		r.ssim, err = verify.Similarity(original, optimized, format)
		if err != nil {
			r.err = err.Error()
		}
		if format != images.FormatGIF && err == nil {
			r.luma, r.ycc, _ = alternatives(original, optimized)
			r.tileAt = r.ssim.At
		}
		r.resultPath = filepath.Join(*out, "results", rel)
		if err := os.MkdirAll(filepath.Dir(r.resultPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(r.resultPath, optimized, 0o644); err != nil {
			return err
		}
		if format != images.FormatGIF && r.err == "" {
			if err := writeCrop(filepath.Join(*out, "crops", strings.ReplaceAll(rel, "/", "_")+".png"), original, optimized, r.tileAt); err != nil {
				return err
			}
		}
		results = append(results, r)
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	sort.Slice(results, func(i, j int) bool { return results[i].ssim.Worst < results[j].ssim.Worst })
	fmt.Println("rgb_worst\trgb_mean\tluma_worst\tluma_mean\tycc_worst\tycc_mean\tcolor_shift\tsaving_pct\tbefore\tafter\tmetadata\tfile\terror")
	for _, r := range results {
		saving := 0.0
		if r.before > 0 && r.after > 0 {
			saving = 100 * float64(r.before-r.after) / float64(r.before)
		}
		fmt.Printf("%.4f\t%.4f\t%.4f\t%.4f\t%.4f\t%.4f\t%.2f\t%.1f\t%d\t%d\t%s\t%s\t%s\n", r.ssim.Worst, r.ssim.Mean, r.luma.Worst, r.luma.Mean, r.ycc.Worst, r.ycc.Mean, r.color, saving, r.before, r.after, r.metadata, r.path, r.err)
	}
	for _, p := range posterized() {
		fmt.Println(p)
	}
}

// tile is the side of the squares the crops are cut around.
const tile = 64

// alternatives are other ways to score the same pair, to compare with the one verify uses: SSIM of
// luma alone, and of Y at full resolution with Cb and Cr at half, as JPEG and the eye treat
// color. at is the worst luma tile, where the crop is cut.
func alternatives(original, result []byte) (verify.Score, verify.Score, image.Point) {
	a, _, errA := image.Decode(bytes.NewReader(original))
	b, _, errB := image.Decode(bytes.NewReader(result))
	if errA != nil || errB != nil {
		return verify.Score{}, verify.Score{}, image.Point{}
	}
	w, h := a.Bounds().Dx(), a.Bounds().Dy()
	ya, ca1, ca2 := ycc(a)
	yb, cb1, cb2 := ycc(b)
	lumaScore := verify.SSIMPlanes(ya, yb, w, h)
	s1 := verify.SSIMPlanes(ca1, cb1, w/2, h/2)
	s2 := verify.SSIMPlanes(ca2, cb2, w/2, h/2)
	yccScore := verify.Score{Mean: min(lumaScore.Mean, s1.Mean, s2.Mean), Worst: min(lumaScore.Worst, s1.Worst, s2.Worst)}
	worst, at := 2.0, image.Point{}
	for ty := 0; ty+tile <= h; ty += tile {
		for tx := 0; tx+tile <= w; tx += tile {
			pa, pb := make([]uint8, tile*tile), make([]uint8, tile*tile)
			for y := range tile {
				copy(pa[y*tile:], ya[(ty+y)*w+tx:(ty+y)*w+tx+tile])
				copy(pb[y*tile:], yb[(ty+y)*w+tx:(ty+y)*w+tx+tile])
			}
			if s := verify.SSIMPlanes(pa, pb, tile, tile).Mean; s < worst {
				worst, at = s, image.Point{tx, ty}
			}
		}
	}
	return lumaScore, yccScore, at
}

// ycc is BT.601 full-range Y of the image composited on black, and Cb and Cr averaged over 2x2.
func ycc(m image.Image) ([]uint8, []uint8, []uint8) {
	b := m.Bounds()
	w, h := b.Dx(), b.Dy()
	y := make([]uint8, w*h)
	cbF, crF := make([]float64, (w/2)*(h/2)), make([]float64, (w/2)*(h/2))
	for row := range h {
		for col := range w {
			c := color.NRGBAModel.Convert(m.At(b.Min.X+col, b.Min.Y+row)).(color.NRGBA)
			a := float64(c.A) / 255
			r, g, bl := a*float64(c.R), a*float64(c.G), a*float64(c.B)
			y[row*w+col] = uint8(math.Round(0.299*r + 0.587*g + 0.114*bl))
			if row/2 < h/2 && col/2 < w/2 {
				i := (row/2)*(w/2) + col/2
				cbF[i] += (128 - 0.168736*r - 0.331264*g + 0.5*bl) / 4
				crF[i] += (128 + 0.5*r - 0.418688*g - 0.081312*bl) / 4
			}
		}
	}
	cb, cr := make([]uint8, len(cbF)), make([]uint8, len(crF))
	for i := range cbF {
		cb[i], cr[i] = uint8(math.Round(cbF[i])), uint8(math.Round(crF[i]))
	}
	return y, cb, cr
}

// writeCrop saves 256x256 pixels around at from both images side by side, original on the left.
func writeCrop(path string, original, result []byte, at image.Point) error {
	a, _, err := image.Decode(bytes.NewReader(original))
	if err != nil {
		return err
	}
	b, _, err := image.Decode(bytes.NewReader(result))
	if err != nil {
		return err
	}
	const side = 192
	at = at.Sub(image.Point{X: (side - tile) / 2, Y: (side - tile) / 2})
	at.X, at.Y = max(0, at.X), max(0, at.Y)
	r := image.Rect(at.X, at.Y, at.X+side, at.Y+side).Add(a.Bounds().Min).Intersect(a.Bounds())
	out := image.NewNRGBA(image.Rect(0, 0, 2*r.Dx()+8, r.Dy()))
	draw.Draw(out, out.Rect, &image.Uniform{color.NRGBA{255, 0, 255, 255}}, image.Point{}, draw.Src)
	draw.Draw(out, image.Rect(0, 0, r.Dx(), r.Dy()), a, r.Min, draw.Over)
	draw.Draw(out, image.Rect(r.Dx()+8, 0, 2*r.Dx()+8, r.Dy()), b, r.Min, draw.Over)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, out)
}

// writeGenerated writes gradients, the content where quantization bands: wide areas with few
// distinct values.
func writeGenerated(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	gray := func(w, h int, from, to float64) image.Image {
		m := image.NewNRGBA(image.Rect(0, 0, w, h))
		for y := range h {
			for x := range w {
				v := uint8(math.Round(from + (to-from)*float64(x)/float64(w-1)))
				m.SetNRGBA(x, y, color.NRGBA{v, v, v, 255})
			}
		}
		return m
	}
	sky := image.NewNRGBA(image.Rect(0, 0, 1200, 600))
	for y := range 600 {
		for x := range 1200 {
			t := float64(y) / 599
			s := float64(x) / 1199
			sky.SetNRGBA(x, y, color.NRGBA{uint8(40 + 180*t + 10*s), uint8(90 + 140*t), uint8(200 + 50*t - 15*s), 255})
		}
	}
	radial := image.NewNRGBA(image.Rect(0, 0, 800, 800))
	for y := range 800 {
		for x := range 800 {
			d := math.Hypot(float64(x-400), float64(y-400)) / 566
			radial.SetNRGBA(x, y, color.NRGBA{uint8(250 - 200*d), uint8(120 + 60*d), uint8(60 + 150*d), 255})
		}
	}
	alpha := image.NewNRGBA(image.Rect(0, 0, 800, 200))
	for y := range 200 {
		for x := range 800 {
			alpha.SetNRGBA(x, y, color.NRGBA{30, uint8(100 + y/2), 220, uint8(255 * x / 799)})
		}
	}
	files := map[string]image.Image{
		"gradient-gray.png":   gray(1024, 256, 0, 255),
		"gradient-dark.png":   gray(1024, 256, 16, 48),
		"gradient-sky.png":    sky,
		"gradient-radial.png": radial,
		"gradient-alpha.png":  alpha,
		"gradient-sky.jpg":    sky,
		"gradient-dark.jpg":   gray(1024, 256, 16, 48),
	}
	for name, m := range files {
		var buf bytes.Buffer
		var err error
		if strings.HasSuffix(name, ".jpg") {
			err = jpeg.Encode(&buf, m, &jpeg.Options{Quality: 95})
		} else {
			err = png.Encode(&buf, m)
		}
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, name), buf.Bytes(), 0o644); err != nil {
			return err
		}
	}
	var anim gif.GIF
	palette := make(color.Palette, 0, 256)
	for i := range 256 {
		palette = append(palette, color.RGBA{uint8(i), uint8(255 - i), uint8(i / 2), 255})
	}
	for i := range 6 {
		frame := image.NewPaletted(image.Rect(0, 0, 320, 120), palette)
		for y := range 120 {
			for x := range 320 {
				frame.SetColorIndex(x, y, uint8((x+30*i)%256))
			}
		}
		anim.Image = append(anim.Image, frame)
		anim.Delay = append(anim.Delay, 10)
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, &anim); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "gradient-anim.gif"), buf.Bytes(), 0o644)
}

// posterized is the SSIM of gradients cut to fewer levels, banding made on purpose, to see where
// visible bands sit on the scale.
func posterized() []string {
	var lines []string
	for _, levels := range []int{64, 32, 16, 8} {
		for _, g := range []struct {
			name     string
			from, to float64
		}{{"gray 0-255", 0, 255}, {"dark 16-48", 16, 48}} {
			const w, h = 1024, 64
			x, y := make([]uint8, w*h), make([]uint8, w*h)
			for row := range h {
				for col := range w {
					v := g.from + (g.to-g.from)*float64(col)/float64(w-1)
					step := 256 / float64(levels)
					x[row*w+col] = uint8(math.Round(v))
					y[row*w+col] = uint8(math.Min(255, math.Floor(v/step)*step+step/2))
				}
			}
			score := verify.SSIMPlanes(x, y, w, h)
			lines = append(lines, fmt.Sprintf("posterized\t%s\t%d levels\tmean %.4f\tworst tile %.4f", g.name, levels, score.Mean, score.Worst))
		}
	}
	return lines
}

// scoreAgainst prints the score of each result against one original.
func scoreAgainst(originalPath string, results []string) {
	original, err := os.ReadFile(originalPath)
	if err != nil {
		log.Fatal(err)
	}
	format, _ := verify.Sniff(original)
	for _, path := range results {
		result, err := os.ReadFile(path)
		if err != nil {
			log.Fatal(err)
		}
		score, err := verify.Similarity(original, result, format)
		luma, ycc, _ := alternatives(original, result)
		shift := 0.0
		if a, _, err := image.Decode(bytes.NewReader(original)); err == nil {
			if b, _, err := image.Decode(bytes.NewReader(result)); err == nil && a.Bounds().Size() == b.Bounds().Size() {
				shift = verify.ColorShift(a, b)
			}
		}
		fmt.Printf("%s\t%d bytes\tworst tile %.4f at %v\tmean %.4f\tluma worst %.4f\tycc worst %.4f\tcolor shift %.2f\t%v\n", path, len(result), score.Worst, score.At, score.Mean, luma.Worst, ycc.Worst, shift, err)
	}
}
