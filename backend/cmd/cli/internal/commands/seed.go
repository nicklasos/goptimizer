package commands

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"app/cmd/cli/internal"
	"app/internal/factory"
	"app/internal/middleware"
	"app/internal/plans"
	"app/internal/secretbox"
)

const seedAdminEmail = "admin@example.com"

// seedReporter lets the factories, which report failures the way tests do, run from the CLI.
type seedReporter struct{}

func (seedReporter) Helper() {}
func (seedReporter) Fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Seeding failed: "+format+"\n", args...)
	os.Exit(1)
}

// RunSeed fills an empty development database with accounts and sample data.
// Add the records of a new module here, built with its factory.
func RunSeed(app *internal.CLIApp, args []string) {
	if app.Config.IsProduction() {
		fmt.Fprintln(os.Stderr, "Refusing to seed: APP_ENV is production")
		os.Exit(1)
	}

	ctx := context.Background()
	if _, err := app.Queries.GetUserByEmail(ctx, seedAdminEmail); err == nil {
		fmt.Printf("Already seeded: %s exists. Nothing was changed.\n", seedAdminEmail)
		return
	} else if !errors.Is(err, sql.ErrNoRows) {
		fmt.Fprintf(os.Stderr, "Seeding failed: %v\n", err)
		os.Exit(1)
	}

	secrets, err := secretbox.New(app.Config.EncryptionKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Seeding failed: %v\n", err)
		os.Exit(1)
	}
	create := factory.New(seedReporter{}, app.Database, secrets)

	create.User(factory.WithEmail(seedAdminEmail), factory.WithName("Super Admin"), factory.WithRoles(middleware.RoleSuperAdmin))
	create.User(factory.WithEmail("manager@example.com"), factory.WithName("Manager"), factory.WithRoles(middleware.RoleAdmin))

	create.User(factory.WithEmail("user@example.com"), factory.WithName("Demo User"))
	create.User(factory.WithEmail("unverified@example.com"), factory.WithName("New Signup"), factory.Unverified())

	backups, err := filepath.Abs(filepath.Join("data", "backups"))
	if err == nil {
		err = os.MkdirAll(backups, 0o750)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Seeding failed: %v\n", err)
		os.Exit(1)
	}
	storage := create.Storage(factory.WithStorageName("Local backups"), factory.LocalStorage(backups))

	database, err := filepath.Abs(app.Config.DatabasePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Seeding failed: %v\n", err)
		os.Exit(1)
	}
	create.Plan(
		factory.WithPlanName("Gogobackup database"),
		factory.SQLiteSource(database),
		factory.WithSchedules(plans.Schedule{Time: 3 * 60, Days: []plans.Weekday{}}),
		factory.WithRetention(30, 8, 12),
		factory.WithStorages(storage),
	)

	fmt.Println("Seeded. Every account has the password:", factory.DefaultPassword)
	fmt.Println("  super admin  ", seedAdminEmail)
	fmt.Println("  admin         manager@example.com")
	fmt.Println("  user          user@example.com")
	fmt.Println("  user          unverified@example.com (email not confirmed)")
	fmt.Println("Storage \"Local backups\" points at", backups)
	fmt.Println("Plan \"Gogobackup database\" backs up", database, "at 03:00 UTC every day")
}
