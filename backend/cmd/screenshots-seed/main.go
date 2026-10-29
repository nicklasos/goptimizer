// Command screenshots-seed fills the throwaway database of scripts/screenshots/shoot.mjs with the
// accounts, folders, scans and images the README screenshots show. It writes through the
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
	for folder, n := range map[*db.Folder]int{shop: 1204, blog: 860, docs: 312, landing: 96} {
		for range n {
			create.Image(folder)
		}
	}
}
