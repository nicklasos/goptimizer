package optimizer

import (
	"fmt"

	"app/internal/folders"
	"app/internal/images"
	"app/internal/tools"
)

// program is how the optimizer calls one image tool on the work copy, which the tool rewrites in
// place. unchanged are exit codes that mean the tool left the file as it was, such as pngquant's
// when quantizing would not meet the quality range or would not be smaller.
type program struct {
	args      func(p folders.Profile, file string) []string
	unchanged []int
}

// programs has an entry for every program in tools.NeedSteps.
var programs = map[string]program{
	"pngquant": {
		args: func(p folders.Profile, file string) []string {
			return []string{"--quality", fmt.Sprintf("%d-%d", p.PNGQualityMin, p.PNGQualityMax), "--skip-if-larger", "--force", "--output", file, "--", file}
		},
		unchanged: []int{98, 99},
	},
	// --strip safe would drop gAMA, cHRM and eXIf, which change how a PNG shows or which way up.
	"oxipng": {args: func(_ folders.Profile, file string) []string {
		return []string{"-o", "3", "--keep", "display,gAMA,cHRM,eXIf", "--quiet", "--", file}
	}},
	"optipng": {args: func(_ folders.Profile, file string) []string {
		return []string{"-o2", "-quiet", "--", file}
	}},
	"jpegoptim": {args: func(p folders.Profile, file string) []string {
		args := []string{"--all-progressive", "--strip-com", "--strip-xmp", "--strip-iptc", "--quiet"}
		if p.Mode == folders.ModeLossy {
			args = append(args, fmt.Sprintf("--max=%d", p.JPEGMaxQuality))
		}
		return append(args, "--", file)
	}},
	"gifsicle": {args: func(p folders.Profile, file string) []string {
		args := []string{"-O3", "--batch"}
		if p.Mode == folders.ModeLossy {
			args = append(args, fmt.Sprintf("--lossy=%d", p.GIFLossy))
		}
		return append(args, file)
	}},
}

// Step is one program to run, found on this server.
type Step struct {
	name string
	path string
	program
}

// Pipeline is what runs on an image of the format under the profile: for each step of its need,
// the first installed program. False when a step has none, and the image waits.
func Pipeline(format images.Format, profile folders.Profile, report tools.Report) ([]Step, bool) {
	need := profile.Need(format)
	for _, n := range tools.NeedSteps {
		if n.Need != need {
			continue
		}
		var steps []Step
		for _, alternatives := range n.Steps {
			found := false
			for _, name := range alternatives {
				if path, ok := report.Path(name); ok {
					steps = append(steps, Step{name: name, path: path, program: programs[name]})
					found = true
					break
				}
			}
			if !found {
				return nil, false
			}
		}
		return steps, true
	}
	return nil, false
}
