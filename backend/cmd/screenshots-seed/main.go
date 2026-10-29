// Command screenshots-seed fills the throwaway database of scripts/screenshots/shoot.mjs with the
// accounts, folders, scans, images and daily stats the README screenshots show. It writes through the
// factories, which do not check that a folder's path exists, so the pictures can show /var/www
// paths this machine lacks. The API there runs no scheduler, so no scan changes what is seeded.
package main

import (
	"fmt"
	"os"
	"time"

	"app/internal/db"
	"app/internal/factory"
	"app/internal/folders"
	"app/internal/images"
	"app/internal/scans"
)

type reporter struct{}

func (reporter) Helper() {}
func (reporter) Fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "screenshots-seed: "+format+"\n", args...)
	os.Exit(1)
}

func main() {
	conn, err := db.Open(os.Getenv("DATABASE_PATH"))
	if err != nil {
		reporter{}.Fatalf("%v", err)
	}
	defer conn.Close()
	create := factory.New(reporter{}, conn)

	create.User(factory.WithEmail("admin@example.com"), factory.WithName("Server Admin"), factory.Admin())
	create.User(factory.WithEmail("devops@example.com"), factory.WithName("DevOps Team"), factory.Admin())
	create.User(factory.WithEmail("shop@example.com"), factory.WithName("Shop Developer"))
	create.User(factory.WithEmail("blog@example.com"), factory.WithName("Blog Editor"))
	create.User(factory.WithEmail("intern@example.com"), factory.WithName("New Intern"), factory.Unverified())

	lossless := folders.DefaultProfile()
	lossless.Mode = folders.ModeLossless
	withCache := folders.DefaultProfile()
	withCache.Excludes = []string{"cache/**", "*.thumb.png"}

	shop := create.Folder(factory.WithFolderName("Shop uploads"), factory.WithFolderPath("/var/www/shop.example.com/public/uploads"), factory.WithProfile(withCache))
	blog := create.Folder(factory.WithFolderName("Blog media"), factory.WithFolderPath("/var/www/blog.example.com/wp-content/uploads"))
	docs := create.Folder(factory.WithFolderName("Docs screenshots"), factory.WithFolderPath("/var/www/docs.example.com/static/img"), factory.WithProfile(lossless))
	landing := create.Folder(factory.WithFolderName("Landing pages"), factory.WithFolderPath("/var/www/landing.example.com/assets"))
	create.Folder(factory.WithFolderName("Staging shop"), factory.WithFolderPath("/var/www/staging.shop.example.com/public/uploads"), factory.Paused())

	now := time.Now()
	full, succeeded := factory.WithScanKind(scans.KindFull), factory.WithScanStatus(scans.StatusSucceeded)
	create.Scan(shop, succeeded, factory.StartedAt(now.Add(-12*time.Minute)), factory.WithScanCounts(scans.Counts{DirsListed: 4, FilesSeen: 318, FilesNew: 27}))
	create.Scan(blog, full, factory.WithScanStatus(scans.StatusRunning), factory.StartedAt(now.Add(-time.Minute)), factory.WithScanCounts(scans.Counts{FilesSeen: 860, FilesNew: 860}))
	create.Scan(docs, full, succeeded, factory.StartedAt(now.Add(-3*time.Hour)), factory.WithScanCounts(scans.Counts{FilesSeen: 312, FilesNew: 312}))
	create.Scan(landing, succeeded, factory.StartedAt(now.Add(-40*time.Minute)))
	// Each folder's images by status, and what its optimizations saved, spread over the days before
	// today so the totals add up as the optimizer would have written them.
	for _, f := range []struct {
		folder                     *db.Folder
		optimized, kept, failed, n int
		before, after              int64
	}{
		{shop, 1150, 40, 4, 1204, 412_000, 168_000},
		{blog, 312, 0, 0, 860, 31_000, 17_800},
		{docs, 298, 14, 0, 312, 298_000, 236_000},
		{landing, 0, 0, 0, 96, 0, 0},
	} {
		for i := range f.n {
			switch {
			case i < f.optimized:
				create.Image(f.folder, factory.Optimized(f.before), factory.AsOnDisk(f.after, 1))
			case i < f.optimized+f.kept:
				create.Image(f.folder, factory.WithImageStatus(images.StatusKept), factory.WithReason("no_saving"))
			case i < f.optimized+f.kept+f.failed:
				create.Image(f.folder, factory.Failed(3, time.Time{}), factory.WithReason("tool_failed: pngquant: signal: killed"))
			default:
				create.Image(f.folder)
			}
		}
		for day := range 6 {
			count := int64(f.optimized / 6)
			if day == 0 {
				count += int64(f.optimized % 6)
			}
			if count > 0 {
				create.DailyStat(f.folder, now.AddDate(0, 0, -day), count, count*f.before, count*f.after)
			}
		}
	}
}
