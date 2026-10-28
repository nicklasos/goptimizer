// Command screenshots-seed fills the throwaway database of scripts/screenshots/shoot.mjs with the
// accounts and folders the README screenshots show. It writes through the factories, which do not
// check that a folder's path exists, so the pictures can show /var/www paths this machine lacks.
package main

import (
	"fmt"
	"os"

	"app/internal/db"
	"app/internal/factory"
	"app/internal/folders"
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

	create.Folder(factory.WithFolderName("Shop uploads"), factory.WithFolderPath("/var/www/shop.example.com/public/uploads"), factory.WithProfile(withCache))
	create.Folder(factory.WithFolderName("Blog media"), factory.WithFolderPath("/var/www/blog.example.com/wp-content/uploads"))
	create.Folder(factory.WithFolderName("Docs screenshots"), factory.WithFolderPath("/var/www/docs.example.com/static/img"), factory.WithProfile(lossless))
	create.Folder(factory.WithFolderName("Landing pages"), factory.WithFolderPath("/var/www/landing.example.com/assets"))
	create.Folder(factory.WithFolderName("Staging shop"), factory.WithFolderPath("/var/www/staging.shop.example.com/public/uploads"), factory.Paused())
}
