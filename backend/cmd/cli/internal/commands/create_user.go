package commands

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"app/cmd/cli/internal"
	appinternal "app/internal"
	"app/internal/db"
	"app/internal/errs"

	"golang.org/x/crypto/bcrypt"
)

// RunCreateUser creates a user in the database.
func RunCreateUser(app *internal.CLIApp, args []string) {
	fs := flag.NewFlagSet("create-user", flag.ExitOnError)
	email := fs.String("email", "", "User email (required)")
	name := fs.String("name", "", "User name (defaults to the email local part)")
	password := fs.String("password", "", "User password, min 8 characters (required)")
	isAdmin := fs.Bool("admin", false, "Make the user an admin, who can manage users")
	fs.Usage = func() {
		fmt.Println("Usage: make cli -- create-user --email EMAIL --password PASSWORD [--name NAME] [--admin]")
		fmt.Println()
		fmt.Println("Create a user. Without --admin the user is not an admin, so pass it for the first account.")
		fmt.Println()
		fmt.Println("Options:")
		fs.PrintDefaults()
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  make cli -- create-user --email admin@example.com --password password123 --admin")
		fmt.Println("  make cli -- create-user --email ada@example.com --name Ada --password password123")
	}

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse flags: %v\n", err)
		os.Exit(1)
	}

	*email = appinternal.NormalizeEmail(*email)
	*name = strings.TrimSpace(*name)

	if *email == "" || *password == "" {
		fmt.Fprintln(os.Stderr, "Error: --email and --password are required")
		fs.Usage()
		os.Exit(1)
	}
	if len(*password) < 8 {
		fmt.Fprintln(os.Stderr, "Error: --password must be at least 8 characters")
		os.Exit(1)
	}
	if *name == "" {
		*name = strings.SplitN(*email, "@", 2)[0]
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to hash password: %v\n", err)
		os.Exit(1)
	}

	user, err := app.Queries.CreateUser(context.Background(), db.CreateUserParams{
		Email:    *email,
		Name:     *name,
		Password: string(hashedPassword),
		IsAdmin:  *isAdmin,

		EmailVerified: true,
	})
	if err != nil {
		if domainErr := errs.DomainErrorFromUniqueViolation(err); domainErr != nil {
			fmt.Fprintf(os.Stderr, "User with this email already exists: %s\n", *email)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Failed to create user: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created user id=%d email=%s name=%s admin=%t\n", user.ID, user.Email, user.Name, user.IsAdmin)
}
