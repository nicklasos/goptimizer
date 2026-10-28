package factory

import (
	"app/internal/db"

	"golang.org/x/crypto/bcrypt"
)

type userAttrs struct {
	email    string
	name     string
	password string
	isAdmin  bool
	verified bool
}

type UserOption func(*userAttrs)

func WithEmail(email string) UserOption       { return func(a *userAttrs) { a.email = email } }
func WithName(name string) UserOption         { return func(a *userAttrs) { a.name = name } }
func WithPassword(password string) UserOption { return func(a *userAttrs) { a.password = password } }

// Admin makes the account an admin: create.User(factory.Admin()).
func Admin() UserOption { return func(a *userAttrs) { a.isAdmin = true } }

// Unverified leaves the email unconfirmed, as it is right after self-registration.
func Unverified() UserOption { return func(a *userAttrs) { a.verified = false } }

// User creates a user who is not an admin, with a verified email and DefaultPassword.
func (f *Factory) User(opts ...UserOption) *db.User {
	f.t.Helper()

	attrs := userAttrs{
		email:    "user-" + unique() + "@example.com",
		name:     "Test User",
		password: DefaultPassword,
		verified: true,
	}
	for _, opt := range opts {
		opt(&attrs)
	}

	// The cheapest cost: these passwords protect nothing, and a suite creates hundreds of users
	hash, err := bcrypt.GenerateFromPassword([]byte(attrs.password), bcrypt.MinCost)
	if err != nil {
		f.t.Fatalf("factory.User: hash password: %v", err)
	}

	user, err := f.queries.FactoryCreateUser(ctx, db.FactoryCreateUserParams{
		Email:    attrs.email,
		Name:     attrs.name,
		Password: string(hash),
		IsAdmin:  attrs.isAdmin,
		Verified: attrs.verified,
	})
	if err != nil {
		f.t.Fatalf("factory.User: %v", err)
	}
	return &user
}
