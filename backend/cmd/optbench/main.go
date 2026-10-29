// Command optbench measures the optimizer's throughput: it copies a corpus of images into a fresh
// folder, scans it, and times the worker pool until nothing is left to claim, for each number of
// workers, the runs alternated. CPU counts this process and the tools it started.
//
//	go run ./cmd/optbench -corpus /tmp/corpus -workers 1,4,7 -runs 5
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"app/internal/db"
	"app/internal/folders"
	"app/internal/images"
	"app/internal/logger"
	"app/internal/optimizer"
	"app/internal/scanner"
	"app/internal/scans"
	"app/internal/tools"
	"app/migrations"

	"github.com/pressly/goose/v3"
)

var logLevel *string

type run struct {
	workers  int
	wall     time.Duration
	cpu      time.Duration
	toolsCPU time.Duration
	images   int
	statuses map[string]int64
	before   int64
	after    int64
}

func main() {
	corpus := flag.String("corpus", "", "directory of images, copied for each run")
	scratch := flag.String("work", filepath.Join(os.TempDir(), "optbench"), "scratch directory, emptied first")
	workerList := flag.String("workers", "1,4,7", "numbers of workers to compare")
	runs := flag.Int("runs", 5, "runs per number of workers, alternated")
	nice := flag.Bool("nice", true, "run the tools under nice, as the default settings do")
	logLevel = flag.String("log", "error", "log level; debug prints every image with how long it took")
	flag.Parse()
	if *corpus == "" {
		log.Fatal("-corpus is required")
	}
	var counts []int
	for _, w := range strings.Split(*workerList, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(w))
		if err != nil || n < 1 {
			log.Fatalf("bad -workers %q", *workerList)
		}
		counts = append(counts, n)
	}
	report := tools.Check(context.Background(), tools.System{})
	var results []run
	for round := range *runs {
		for _, workers := range counts {
			r := measure(*corpus, filepath.Join(*scratch, fmt.Sprintf("r%d-w%d", round, workers)), workers, *nice, report)
			results = append(results, r)
			fmt.Printf("round %d\tworkers %d\t%.1f s\t%.2f images/s\tcpu %.1f s, tools %.1f s\tcpu/wall %.2f\t%v\t%d -> %d bytes\n",
				round+1, workers, r.wall.Seconds(), float64(r.images)/r.wall.Seconds(), r.cpu.Seconds(), r.toolsCPU.Seconds(), r.cpu.Seconds()/r.wall.Seconds(), r.statuses, r.before, r.after)
		}
	}
	fmt.Println()
	fmt.Println("workers\truns\tmedian images/s\trange\tmedian cpu/wall")
	for _, workers := range counts {
		var rates, loads []float64
		for _, r := range results {
			if r.workers == workers {
				rates = append(rates, float64(r.images)/r.wall.Seconds())
				loads = append(loads, r.cpu.Seconds()/r.wall.Seconds())
			}
		}
		sort.Float64s(rates)
		sort.Float64s(loads)
		fmt.Printf("%d\t%d\t%.2f\t%.2f to %.2f\t%.2f\n", workers, len(rates), rates[len(rates)/2], rates[0], rates[len(rates)-1], loads[len(loads)/2])
	}
}

func measure(corpus, dir string, workers int, nice bool, report tools.Report) run {
	check(os.RemoveAll(dir))
	root := filepath.Join(dir, "images")
	before := copyTree(corpus, root)
	conn, err := db.Open(filepath.Join(dir, "bench.db"))
	check(err)
	defer conn.Close()
	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())
	check(goose.SetDialect(migrations.Dialect))
	check(goose.Up(conn, "."))
	queries, tx := db.New(conn), db.NewTxRunner(conn)
	ctx := context.Background()
	profile, err := json.Marshal(folders.DefaultProfile())
	check(err)
	folder, err := queries.CreateFolder(ctx, db.CreateFolderParams{
		Name: "bench", Kind: "local", Path: root, ScanIntervalMinutes: 60, FullScanIntervalHours: 24, Profile: string(profile),
	})
	check(err)
	_, err = queries.UpdateSettings(ctx, db.UpdateSettingsParams{Workers: int64(workers), ScanWorkers: 1, Nice: nice})
	check(err)
	log, err := logger.New(logger.Config{Level: *logLevel, Format: "text", Output: "stdout"})
	check(err)
	_, err = scanner.New(queries, tx, log).Scan(ctx, folder.ID, scans.KindFull, nil)
	check(err)
	pending := countStatuses(ctx, queries, folder.ID)[string(images.StatusPending)]

	reporter := tools.NewReporter(fixedFinder{report}, time.Hour)
	pool := optimizer.NewPool(queries, tx, log, reporter, filepath.Join(dir, "work"))
	cpuBefore, toolsBefore := cpuTime()
	started := time.Now()
	check(pool.Start())
	for {
		time.Sleep(100 * time.Millisecond)
		statuses := countStatuses(ctx, queries, folder.ID)
		if statuses[string(images.StatusPending)]+statuses[string(images.StatusOptimizing)] == 0 {
			break
		}
	}
	wall := time.Since(started)
	pool.Stop()
	cpuAfter, toolsAfter := cpuTime()
	r := run{workers: workers, wall: wall, cpu: cpuAfter - cpuBefore, toolsCPU: toolsAfter - toolsBefore, images: int(pending), statuses: countStatuses(ctx, queries, folder.ID), before: before, after: treeSize(root)}
	check(os.RemoveAll(dir))
	return r
}

// fixedFinder answers from a report taken once, so no run pays for looking the tools up.
type fixedFinder struct{ report tools.Report }

func (f fixedFinder) Find(_ context.Context, name string) (tools.Tool, error) {
	for _, p := range f.report.Programs {
		if p.Name == name && p.Installed {
			return tools.Tool{Name: name, Path: p.Path, Version: p.Version}, nil
		}
	}
	return tools.Tool{Name: name}, tools.ErrNotFound
}

func countStatuses(ctx context.Context, q *db.Queries, folderID int64) map[string]int64 {
	rows, err := q.CountImagesOfFolderByStatus(ctx, folderID)
	if err != nil && err != sql.ErrNoRows {
		check(err)
	}
	statuses := map[string]int64{}
	for _, r := range rows {
		statuses[r.Status] = r.Count
	}
	return statuses
}

// copyTree copies every file, with an mtime a day old so min_age_minutes lets it through.
func copyTree(from, to string) int64 {
	var total int64
	old := time.Now().Add(-24 * time.Hour)
	check(filepath.WalkDir(from, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, path)
		target := filepath.Join(to, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		total += int64(len(data))
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
		return os.Chtimes(target, old, old)
	}))
	return total
}

func treeSize(root string) int64 {
	var total int64
	check(filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			info, err := d.Info()
			if err == nil {
				total += info.Size()
			}
		}
		return err
	}))
	return total
}

// cpuTime is the user and system time of this process and its tools, and of the tools alone.
func cpuTime() (time.Duration, time.Duration) {
	var self, children syscall.Rusage
	check(syscall.Getrusage(syscall.RUSAGE_SELF, &self))
	check(syscall.Getrusage(syscall.RUSAGE_CHILDREN, &children))
	used := func(u syscall.Rusage) time.Duration {
		return time.Duration(u.Utime.Nano()) + time.Duration(u.Stime.Nano())
	}
	return used(self) + used(children), used(children)
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
