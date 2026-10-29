// Command scanbench measures the scanner on a synthetic tree, by default 1,000,000 files in
// 10,110 directories nested three deep, 90% of them images:
//
//	go run ./cmd/scanbench                       # the numbers in docs/benchmarks/scanner.md
//	go run ./cmd/scanbench -runs 1 -cpuprofile cpu.out -profile quick
//
// It times (a) the first full scan of a fresh database, then rounds of (b) a quick scan with no
// change, (b2) the same again at once, (b3) the same after the idle wait that comes before (c),
// (c) a quick scan after adding 100 images in 10 directories and deleting 50 in 5 others, and
// (d) a full scan with no change. (b2) is the control for (b): on macOS a full scan evicts the
// directories from the kernel's vnode cache, which (b) then pays for and (b2) does not. (b3) is
// the control for (c), which also starts from idle. Rows written are SQLite's total_changes() on the only connection, so they
// include every insert, update and delete the scan made.
//
// It is a program rather than a go test benchmark because the phases share one tree and one
// database, with changes between them, which b.N repetition does not express.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/pprof"
	"runtime/trace"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"app/internal/db"
	"app/internal/logger"
	"app/internal/scanner"
	"app/internal/scans"
	"app/migrations"

	"github.com/pressly/goose/v3"
)

var (
	top       = flag.Int("top", 10, "directories at the first level")
	mid       = flag.Int("mid", 10, "directories inside each of those")
	leaves    = flag.Int("leaves", 100, "directories inside each of those, which hold the files")
	perLeaf   = flag.Int("files", 100, "files in each leaf directory; every tenth is not an image")
	runs      = flag.Int("runs", 5, "rounds of (b), (c) and (d)")
	parent    = flag.String("dir", "", "where to make the tree and the database (default: the system temp directory)")
	keep      = flag.Bool("keep", false, "keep the tree and the database")
	profile   = flag.String("profile", "", "phase to profile or trace: first, quick, again, idle, change or full")
	profileTo = flag.String("cpuprofile", "", "file for the CPU profile of that phase")
	traceTo   = flag.String("trace", "", "file for the execution trace of that phase (go tool trace -pprof=syscall)")
)

// settle is longer than the scanner's settle time, so no directory is stored with mtime 0 and the
// scans that follow measure the same work every round.
const settle = 2100 * time.Millisecond

type sample struct {
	wall, cpu time.Duration
	rows      int64
	counts    scans.Counts
}

type bench struct {
	ctx     context.Context
	queries *db.Queries
	scanner *scanner.Scanner
	folder  int64
	root    string
}

func main() {
	flag.Parse()
	work, err := os.MkdirTemp(*parent, "scanbench-")
	check(err)
	if !*keep {
		defer os.RemoveAll(work)
	}
	root := filepath.Join(work, "tree")

	start := time.Now()
	files, images, dirs := generate(root)
	log.Printf("generated %d files (%d images) in %d directories in %s", files, images, dirs, time.Since(start).Round(time.Second))

	conn, err := db.Open(filepath.Join(work, "bench.db"))
	check(err)
	defer conn.Close()
	// One connection, so total_changes() counts every write of the scan.
	conn.SetMaxOpenConns(1)
	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())
	check(goose.SetDialect(migrations.Dialect))
	check(goose.Up(conn, "."))

	queries := db.New(conn)
	resolved, err := filepath.EvalSymlinks(root)
	check(err)
	folder, err := queries.CreateFolder(context.Background(), db.CreateFolderParams{
		Name: "scanbench", Kind: "local", Path: resolved, ScanIntervalMinutes: 60, FullScanIntervalHours: 24, Profile: "{}",
	})
	check(err)
	appLogger, err := logger.New(logger.Config{Level: "warn", Format: "text", Output: "stderr"})
	check(err)
	b := &bench{ctx: context.Background(), queries: queries, scanner: scanner.New(queries, db.NewTxRunner(conn), appLogger), folder: folder.ID, root: root}

	time.Sleep(settle)
	first := b.measure("first", scans.KindFull)
	var quick, again, idle, change, full []sample
	for round := range *runs {
		quick = append(quick, b.measure("quick", scans.KindQuick))
		again = append(again, b.measure("again", scans.KindQuick))
		time.Sleep(settle)
		idle = append(idle, b.measure("idle", scans.KindQuick))
		b.mutate(round)
		time.Sleep(settle)
		change = append(change, b.measure("change", scans.KindQuick))
		full = append(full, b.measure("full", scans.KindFull))
	}

	var listCounts []time.Duration
	for range *runs {
		t := time.Now()
		_, err := queries.CountImagesByStatus(b.ctx)
		check(err)
		listCounts = append(listCounts, time.Since(t))
	}
	counts, err := queries.CountImagesOfFolderByStatus(b.ctx, b.folder)
	check(err)

	fmt.Printf("tree: %d files, %d images, %d directories\n\n", files, images, dirs)
	fmt.Println("| phase | runs | wall median | wall range | each run | CPU median | CPU/wall | rows written | dirs seen / listed | files new / changed / gone |")
	fmt.Println("|---|---|---|---|---|---|---|---|---|---|")
	report("(a) first full scan", []sample{first})
	report("(b) quick, no change", quick)
	report("(b2) quick again at once", again)
	report("(b3) quick after 2.1 s idle", idle)
	report("(c) quick, +100 / -50 images", change)
	report("(d) full, no change", full)
	fmt.Printf("\nCountImagesByStatus (the folder list's image_counts): median %s, range %s to %s, %d runs\n",
		median(listCounts), slices.Min(listCounts), slices.Max(listCounts), len(listCounts))
	fmt.Printf("images by status at the end: %v\n", counts)
}

func (b *bench) measure(phase string, kind scans.Kind) sample {
	before, err := b.queries.TotalChanges(b.ctx)
	check(err)
	if phase == *profile && *profileTo != "" {
		f, err := os.Create(*profileTo)
		check(err)
		check(pprof.StartCPUProfile(f))
		defer func() { pprof.StopCPUProfile(); f.Close() }()
	}
	if phase == *profile && *traceTo != "" {
		f, err := os.Create(*traceTo)
		check(err)
		check(trace.Start(f))
		defer func() { trace.Stop(); f.Close() }()
	}
	cpu := cpuTime()
	start := time.Now()
	counts, err := b.scanner.Scan(b.ctx, b.folder, kind, nil)
	wall := time.Since(start)
	cpu = cpuTime() - cpu
	check(err)
	after, err := b.queries.TotalChanges(b.ctx)
	check(err)
	return sample{wall: wall, cpu: cpu, rows: after - before, counts: counts}
}

// mutate adds 10 images to each of 10 leaf directories and deletes 10 from each of 5 others,
// different ones every round.
func (b *bench) mutate(round int) {
	leaf := func(i int) string {
		i += round * 15
		return filepath.Join(b.root, fmt.Sprintf("t%02d/m%02d/l%03d", i/(*mid**leaves)%*top, i/(*leaves)%*mid, i%*leaves))
	}
	for d := range 10 {
		for f := range 10 {
			check(os.WriteFile(filepath.Join(leaf(d), fmt.Sprintf("new-%d-%d.png", round, f)), nil, 0o644))
		}
	}
	for d := 10; d < 15; d++ {
		for _, f := range []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 10} {
			check(os.Remove(filepath.Join(leaf(d), fileName(f))))
		}
	}
}

// generate makes the tree, one leaf directory per job on every core, and returns how many
// files, images and directories it holds, the root included.
func generate(root string) (files, images, dirs int) {
	jobs := make(chan string)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for dir := range jobs {
				check(os.MkdirAll(dir, 0o755))
				for f := range *perLeaf {
					check(os.WriteFile(filepath.Join(dir, fileName(f)), nil, 0o644))
				}
			}
		})
	}
	for t := range *top {
		for m := range *mid {
			for l := range *leaves {
				jobs <- filepath.Join(root, fmt.Sprintf("t%02d/m%02d/l%03d", t, m, l))
			}
		}
	}
	close(jobs)
	wg.Wait()
	n := *top * *mid * *leaves
	return n * *perLeaf, n * (*perLeaf - *perLeaf/10), 1 + *top + *top**mid + n
}

// fileName is the f-th file of a leaf: png, jpg and gif in turn, and every tenth a text file.
func fileName(f int) string {
	if f%10 == 9 {
		return fmt.Sprintf("f%03d.txt", f)
	}
	return fmt.Sprintf("f%03d.%s", f, []string{"png", "jpg", "gif"}[f%3])
}

func cpuTime() time.Duration {
	var usage syscall.Rusage
	check(syscall.Getrusage(syscall.RUSAGE_SELF, &usage))
	return time.Duration(usage.Utime.Nano() + usage.Stime.Nano())
}

func report(phase string, samples []sample) {
	walls := make([]time.Duration, len(samples))
	cpus := make([]time.Duration, len(samples))
	rows := make([]int64, len(samples))
	each := make([]string, len(samples))
	for i, s := range samples {
		walls[i], cpus[i], rows[i] = s.wall, s.cpu, s.rows
		each[i] = s.wall.Round(time.Millisecond).String()
	}
	slices.Sort(rows)
	c := samples[len(samples)-1].counts
	fmt.Printf("| %s | %d | %s | %s to %s | %s | %s | %.2f | %v | %d / %d | %d / %d / %d |\n",
		phase, len(samples), median(walls).Round(time.Millisecond), slices.Min(walls).Round(time.Millisecond), slices.Max(walls).Round(time.Millisecond),
		strings.Join(each, ", "), median(cpus).Round(time.Millisecond), float64(median(cpus))/float64(median(walls)),
		slices.Compact(rows), c.DirsSeen, c.DirsListed, c.FilesNew, c.FilesChanged, c.FilesGone)
}

func median(values []time.Duration) time.Duration {
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	return sorted[len(sorted)/2]
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
