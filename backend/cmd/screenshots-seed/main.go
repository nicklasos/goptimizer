// Command screenshots-seed fills the throwaway database of scripts/screenshots/shoot.mjs with the
// accounts, folders, scans, images and daily stats the README screenshots show. It writes through the
// factories, which do not check that a folder's path exists, so the pictures can show /var/www
// paths this machine lacks. The API there runs no scheduler, so no scan changes what is seeded.
package main

import (
	"fmt"
	"math/rand/v2"
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

type imageGroup struct {
	status images.Status
	reason string
	count  int
}

// A folder's optimized images are spread over its last spreadDays days, skipping every
// quietEvery-th day, and each result is about resultPercent of its original's size.
type folderPlan struct {
	folder        *db.Folder
	paths         func(i int) (dir, name string)
	optimized     int
	spreadDays    int
	quietEvery    int
	resultPercent int
	others        []imageGroup
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
	seedScans(create, now, shop, blog, docs, landing)

	products := []string{"oak-table", "linen-chair", "wool-throw", "ceramic-vase", "desk-lamp", "rattan-basket", "glass-carafe", "cotton-rug"}
	posts := []string{"spring-menu", "team-offsite", "release-notes", "behind-the-scenes", "recipe-card", "event-poster"}
	guides := []string{"install", "configure", "deploy", "monitor", "upgrade"}
	plans := []folderPlan{
		{
			folder: shop,
			paths: func(i int) (string, string) {
				ext := []string{".jpg", ".jpg", ".png", ".jpg", ".gif"}[i%5]
				return fmt.Sprintf("products/2025/%02d", 1+i%12), fmt.Sprintf("%s-%d%s", products[i%len(products)], 1000+i, ext)
			},
			optimized: 1150, spreadDays: 30, quietEvery: 9, resultPercent: 41,
			others: []imageGroup{
				{images.StatusKept, "no_saving: 182044 bytes to 179311", 14},
				{images.StatusKept, "ssim_below_threshold: 0.7934, below 0.8500, at 384,128; lossless: no_saving: 96112 bytes to 95870", 9},
				{images.StatusKept, "already_optimized", 7},
				{images.StatusKept, "metadata_changed: the iCCP profile changed", 4},
				{images.StatusKept, "color_changed: a block's average color moved 14.2 levels; lossless: no_saving: 51200 bytes to 50980", 3},
				{images.StatusSkipped, "apng", 3},
				{images.StatusSkipped, "extra_data: a motion photo's video after the end of the JPEG", 2},
				{images.StatusSkipped, "too_many_pixels", 1},
				{images.StatusSkipped, "symlink", 2},
				{images.StatusFailed, "tool_failed: pngquant: signal: killed", 2},
				{images.StatusFailed, "tool_failed: jpegoptim: exit status 1: Not a JPEG file: starts with 0x89 0x50", 2},
				{images.StatusPending, "too_new", 18},
				{images.StatusPending, "", 12},
				{images.StatusOptimizing, "", 2},
				{images.StatusGone, "", 6},
			},
		},
		{
			folder: blog,
			paths: func(i int) (string, string) {
				return fmt.Sprintf("2025/%02d", 1+i%10), fmt.Sprintf("%s-%d.jpg", posts[i%len(posts)], i)
			},
			optimized: 312, spreadDays: 2, resultPercent: 57,
			others: []imageGroup{{images.StatusOptimizing, "", 3}, {images.StatusPending, "", 545}},
		},
		{
			folder: docs,
			paths: func(i int) (string, string) {
				return "guides/" + guides[i%len(guides)], fmt.Sprintf("step-%02d.png", i)
			},
			optimized: 298, spreadDays: 12, quietEvery: 3, resultPercent: 79,
			others: []imageGroup{{images.StatusKept, "no_saving: 40211 bytes to 39877", 11}, {images.StatusKept, "pixels_changed: first at 0,12", 3}},
		},
		{
			folder: landing,
			paths: func(i int) (string, string) {
				return "img", fmt.Sprintf("hero-%d.png", i)
			},
			others: []imageGroup{{images.StatusPending, "", 96}},
		},
	}
	r := rand.New(rand.NewPCG(5, 2025))
	for _, plan := range plans {
		seedImages(create, r, plan, now)
	}
}

func seedImages(create *factory.Factory, r *rand.Rand, plan folderPlan, now time.Time) {
	type day struct {
		at                   time.Time
		count, before, after int64
	}
	byDay := map[string]*day{}
	i := 0
	for ; i < plan.optimized; i++ {
		before := int64(40_000 + r.IntN(900_000))
		after := before * int64(plan.resultPercent-12+r.IntN(24)) / 100
		ago := r.IntN(plan.spreadDays)
		for plan.quietEvery > 0 && ago%plan.quietEvery == 1 {
			ago = r.IntN(plan.spreadDays)
		}
		at := now.Add(-time.Duration(ago)*24*time.Hour - time.Duration(r.IntN(36_000))*time.Second)
		dir, name := plan.paths(i)
		create.Image(plan.folder, factory.At(dir, name), factory.AsOnDisk(after, 1), factory.Optimized(before), factory.UpdatedAt(at))
		key := images.Day(at)
		if byDay[key] == nil {
			byDay[key] = &day{at: at}
		}
		byDay[key].count++
		byDay[key].before += before
		byDay[key].after += after
	}
	for _, group := range plan.others {
		for range group.count {
			dir, name := plan.paths(i)
			i++
			at := now.Add(-time.Duration(r.IntN(20*86_400)) * time.Second)
			if group.status == images.StatusPending || group.status == images.StatusOptimizing {
				at = now.Add(-time.Duration(r.IntN(3_600)) * time.Second)
			}
			opts := []factory.ImageOption{factory.At(dir, name), factory.AsOnDisk(int64(30_000+r.IntN(600_000)), 1), factory.UpdatedAt(at)}
			switch group.status {
			case images.StatusFailed:
				retryAt := time.Time{}
				attempts := int64(images.MaxAttempts)
				if r.IntN(2) == 0 {
					retryAt, attempts = now.Add(time.Duration(10+r.IntN(40))*time.Minute), 1
				}
				opts = append(opts, factory.Failed(attempts, retryAt))
			default:
				opts = append(opts, factory.WithImageStatus(group.status))
			}
			if group.reason != "" {
				opts = append(opts, factory.WithReason(group.reason))
			}
			create.Image(plan.folder, opts...)
		}
	}
	for _, d := range byDay {
		create.DailyStat(plan.folder, d.at, d.count, d.before, d.after)
	}
}

func seedScans(create *factory.Factory, now time.Time, shop, blog, docs, landing *db.Folder) {
	succeeded, full := factory.WithScanStatus(scans.StatusSucceeded), factory.WithScanKind(scans.KindFull)
	for h := 47; h >= 1; h-- {
		start := now.Add(-time.Duration(h)*time.Hour - 12*time.Minute)
		counts := scans.Counts{DirsSeen: 214, DirsListed: int64(h % 4), FilesSeen: int64(3 * (h % 4)), FilesNew: int64(2 * (h % 4))}
		switch {
		case h == 30:
			create.Scan(shop, factory.WithScanStatus(scans.StatusFailed), factory.StartedAt(start), factory.Took(800*time.Millisecond),
				factory.WithScanError("open /var/www/shop.example.com/public/uploads/products/2025/07: permission denied"))
		case h%24 == 0:
			create.Scan(shop, full, succeeded, factory.StartedAt(start), factory.Took(41*time.Second),
				factory.WithScanCounts(scans.Counts{DirsSeen: 214, DirsListed: 214, FilesSeen: 1274, FilesChanged: 3}))
		default:
			create.Scan(shop, succeeded, factory.StartedAt(start), factory.Took(time.Duration(40+h*7)*time.Millisecond), factory.WithScanCounts(counts))
		}
	}
	create.Scan(shop, succeeded, factory.WithTrigger(scans.TriggerManual), factory.StartedAt(now.Add(-12*time.Minute)), factory.Took(210*time.Millisecond),
		factory.WithScanCounts(scans.Counts{DirsSeen: 214, DirsListed: 4, FilesSeen: 318, FilesNew: 27}))

	create.Scan(blog, full, succeeded, factory.StartedAt(now.Add(-26*time.Hour)), factory.Took(3*time.Second),
		factory.WithScanCounts(scans.Counts{DirsSeen: 18, DirsListed: 18, FilesSeen: 312, FilesNew: 312}))
	create.Scan(blog, full, factory.WithScanStatus(scans.StatusRunning), factory.StartedAt(now.Add(-time.Minute)),
		factory.WithScanCounts(scans.Counts{DirsSeen: 31, DirsListed: 22, FilesSeen: 860, FilesNew: 548}))

	for d := 6; d >= 0; d-- {
		counts := scans.Counts{DirsSeen: 6, DirsListed: 6, FilesSeen: 312}
		if d == 6 {
			counts.FilesNew = 312
		}
		create.Scan(docs, full, succeeded, factory.StartedAt(now.Add(-time.Duration(d)*24*time.Hour-3*time.Hour)), factory.Took(1200*time.Millisecond),
			factory.WithScanCounts(counts))
	}
	create.Scan(landing, full, succeeded, factory.StartedAt(now.Add(-40*time.Minute)), factory.Took(90*time.Millisecond),
		factory.WithScanCounts(scans.Counts{DirsSeen: 1, DirsListed: 1, FilesSeen: 96, FilesNew: 96}))
}
