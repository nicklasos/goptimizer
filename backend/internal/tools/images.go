package tools

import (
	"context"
	"errors"
	"slices"
	"sync"
)

// Need is one kind of work the optimizer does: a format, and for PNG the mode.
type Need string

const (
	NeedPNGLossy    Need = "png_lossy"
	NeedPNGLossless Need = "png_lossless"
	NeedJPEG        Need = "jpeg"
	NeedGIF         Need = "gif"
)

// Program is an image tool. Purpose is in English; the admin panel translates it by Name.
type Program struct {
	Name    string
	Purpose string
	// Apt is the Ubuntu package, empty when Ubuntu has none. Brew is the macOS formula.
	Apt  string
	Brew string
}

// Programs is every image tool, in the order the settings page lists them.
var Programs = []Program{
	{Name: "pngquant", Purpose: "Lossy PNG: reduces the colors to a palette", Apt: "pngquant", Brew: "pngquant"},
	// Not in Ubuntu's archive in any release (Launchpad, October 2026); optipng stands in.
	{Name: "oxipng", Purpose: "Lossless PNG recompression", Brew: "oxipng"},
	{Name: "optipng", Purpose: "Lossless PNG recompression, when oxipng is missing", Apt: "optipng", Brew: "optipng"},
	{Name: "jpegoptim", Purpose: "JPEG: caps the quality and strips comments", Apt: "jpegoptim", Brew: "jpegoptim"},
	{Name: "gifsicle", Purpose: "GIF optimization, lossy or lossless", Apt: "gifsicle", Brew: "gifsicle"},
}

// NeedSteps is what each need runs, in order. A step names the programs that can do it,
// preferred first, and is covered when any of them is installed.
var NeedSteps = []struct {
	Need  Need
	Steps [][]string
}{
	{NeedPNGLossy, [][]string{{"pngquant"}, {"oxipng", "optipng"}}},
	{NeedPNGLossless, [][]string{{"oxipng", "optipng"}}},
	{NeedJPEG, [][]string{{"jpegoptim"}}},
	{NeedGIF, [][]string{{"gifsicle"}}},
}

type ProgramStatus struct {
	Name    string `json:"name"`
	Purpose string `json:"purpose"`
	UsedFor []Need `json:"used_for" enums:"png_lossy,png_lossless,jpeg,gif"`
	// Alternatives can do this program's work when it is missing. Empty means it is required.
	Alternatives []string `json:"alternatives"`
	Installed    bool     `json:"installed"`
	Path         string   `json:"path"`
	Version      string   `json:"version"`
	Apt          string   `json:"apt"`
	Brew         string   `json:"brew"`
}

type NeedStatus struct {
	Need      Need `json:"need" enums:"png_lossy,png_lossless,jpeg,gif"`
	Satisfied bool `json:"satisfied"`
	// Missing is the preferred program of each step nothing installed can do.
	Missing []string `json:"missing"`
}

// Report is what is installed and what work it covers.
type Report struct {
	Programs []ProgramStatus `json:"programs"`
	Needs    []NeedStatus    `json:"needs"`
}

// Check looks every program up at once. A program on PATH whose version could not be read
// counts as installed, with an empty version.
func Check(ctx context.Context, finder Finder) Report {
	found := make([]Tool, len(Programs))
	missing := make([]bool, len(Programs))
	var wg sync.WaitGroup
	for i, program := range Programs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tool, err := finder.Find(ctx, program.Name)
			found[i] = tool
			missing[i] = errors.Is(err, ErrNotFound)
		}()
	}
	wg.Wait()

	installed := map[string]bool{}
	report := Report{Programs: make([]ProgramStatus, len(Programs))}
	for i, program := range Programs {
		installed[program.Name] = !missing[i]
		report.Programs[i] = ProgramStatus{
			Name:         program.Name,
			Purpose:      program.Purpose,
			UsedFor:      usedFor(program.Name),
			Alternatives: alternatives(program.Name),
			Installed:    !missing[i],
			Path:         found[i].Path,
			Version:      found[i].Version,
			Apt:          program.Apt,
			Brew:         program.Brew,
		}
	}

	for _, need := range NeedSteps {
		status := NeedStatus{Need: need.Need, Missing: []string{}}
		for _, step := range need.Steps {
			if !slices.ContainsFunc(step, func(name string) bool { return installed[name] }) {
				status.Missing = append(status.Missing, step[0])
			}
		}
		status.Satisfied = len(status.Missing) == 0
		report.Needs = append(report.Needs, status)
	}
	return report
}

func usedFor(name string) []Need {
	needs := []Need{}
	for _, need := range NeedSteps {
		for _, step := range need.Steps {
			if slices.Contains(step, name) && !slices.Contains(needs, need.Need) {
				needs = append(needs, need.Need)
			}
		}
	}
	return needs
}

func alternatives(name string) []string {
	others := []string{}
	for _, need := range NeedSteps {
		for _, step := range need.Steps {
			if !slices.Contains(step, name) {
				continue
			}
			for _, other := range step {
				if other != name && !slices.Contains(others, other) {
					others = append(others, other)
				}
			}
		}
	}
	return others
}
