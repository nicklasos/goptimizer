package commands

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"app/cmd/cli/internal"
	"app/internal/db"
	"app/internal/errs"
	"app/internal/plans"
	"app/internal/secretbox"
	"app/internal/selfbackup"
	"app/internal/storages"
	"app/internal/tools"

	"github.com/go-playground/validator/v10"
)

// RunSelfBackup makes sure a plan backs up this service's own database. Deploys run it on
// every release; Ansible reads "Created" in the output as a change.
func RunSelfBackup(app *internal.CLIApp, args []string) {
	fs := flag.NewFlagSet("self-backup", flag.ExitOnError)
	dir := fs.String("dir", "", "Directory of the local storage, created if missing (required)")
	at := fs.String("time", "03:15", "Daily run time, HH:MM in UTC")
	maxAgeDays := fs.Int("max-age-days", 30, "Keep every archive this many days (0 keeps them all)")
	keepWeekly := fs.Int("keep-weekly", 8, "After that, keep one archive a week for this many weeks")
	keepMonthly := fs.Int("keep-monthly", 12, "After that, keep one archive a month for this many months")
	fs.Usage = func() {
		fmt.Println("Usage: make cli -- self-backup --dir DIR [--time HH:MM] [--max-age-days N] [--keep-weekly N] [--keep-monthly N]")
		fmt.Println()
		fmt.Printf("Make sure the plan %q backs up DATABASE_PATH to a local storage in DIR.\n", selfbackup.PlanName)
		fmt.Println("A plan that already backs up DATABASE_PATH is left as it is, whatever its name; the options only shape a new one.")
		fmt.Println()
		fmt.Println("Options:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse flags: %v\n", err)
		os.Exit(1)
	}
	if strings.TrimSpace(*dir) == "" {
		fmt.Fprintln(os.Stderr, "Error: --dir is required")
		fs.Usage()
		os.Exit(1)
	}

	if err := app.Config.ValidateEncryptionKey(); err != nil {
		fmt.Fprintf(os.Stderr, "Self-backup failed: %v\n", err)
		os.Exit(1)
	}
	secrets, err := secretbox.New(app.Config.EncryptionKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Self-backup failed: %v\n", err)
		os.Exit(1)
	}

	result, err := selfbackup.Ensure(context.Background(), app.Queries,
		storages.NewStorageService(app.Queries, secrets),
		plans.NewPlanService(app.Queries, db.NewTxRunner(app.Database), secrets, tools.System{}),
		selfbackup.Options{
			Database:    app.Config.DatabasePath,
			Dir:         *dir,
			Time:        *at,
			MaxAgeDays:  *maxAgeDays,
			KeepWeekly:  *keepWeekly,
			KeepMonthly: *keepMonthly,
		})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Self-backup failed: %s\n", describe(err))
		os.Exit(1)
	}

	plan := result.Plan
	if !result.PlanCreated {
		fmt.Printf("Found plan %q (id %d) backing up this database.\n", plan.Name, plan.ID)
		printPlan(plan)
		fmt.Println("Nothing was changed.")
		return
	}
	if result.StorageCreated {
		fmt.Printf("Created storage %q (id %d) at %s\n", result.Storage.Name, result.Storage.ID, *dir)
	} else {
		fmt.Printf("Found storage %q (id %d) at %s\n", result.Storage.Name, result.Storage.ID, *dir)
	}
	fmt.Printf("Created plan %q (id %d)\n", plan.Name, plan.ID)
	printPlan(plan)
}

func printPlan(plan *plans.Plan) {
	var names []string
	for _, s := range plan.Storages {
		names = append(names, fmt.Sprintf("%s (%s)", s.Name, s.Kind))
	}
	fmt.Printf("  enabled    %t\n", plan.Enabled)
	fmt.Printf("  schedules  %s (UTC)\n", plan.Schedules)
	fmt.Printf("  retention  max_age_days=%d keep_weekly=%d keep_monthly=%d\n", plan.MaxAgeDays, plan.KeepWeekly, plan.KeepMonthly)
	fmt.Printf("  storages   %s\n", strings.Join(names, ", "))
}

// describe spells out a validation failure field by field, the way the API would answer it.
func describe(err error) string {
	var fields errs.Fields
	var invalid validator.ValidationErrors
	if errors.As(err, &fields) || errors.As(err, &invalid) {
		var lines []string
		for field, keys := range errs.FormatValidationError(err).Errors {
			lines = append(lines, field+": "+strings.Join(keys, ", "))
		}
		return "invalid options: " + strings.Join(lines, "; ")
	}
	return err.Error()
}
