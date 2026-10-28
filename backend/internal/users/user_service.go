package users

import (
	"context"
	"fmt"
	"net/http"

	"app/internal"
	"app/internal/db"
	"app/internal/errs"
	"app/internal/middleware"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserAlreadyExists = errs.NewBadRequestError(errs.ErrKeyAuthUserExists, "User with this email already exists")
	ErrUserNotFound      = errs.NewNotFoundError(errs.ErrKeyUsersNotFound, "User not found")
	ErrCannotDeleteSelf  = errs.NewBadRequestError(errs.ErrKeyUsersCannotDeleteSelf, "Cannot delete yourself")
	ErrCannotDemoteSelf  = errs.NewBadRequestError(errs.ErrKeyUsersCannotDemoteSelf, "Cannot remove your own admin rights")
	ErrLastAdmin         = errs.NewDomainError(errs.ErrKeyUsersLastAdmin, "At least one admin must remain", http.StatusConflict)
)

type UserService struct {
	queries *db.Queries
	tx      *db.TxRunner
}

type PaginatedUsers struct {
	Data  []db.User
	Total int64
}

func NewUserService(queries *db.Queries, tx *db.TxRunner) *UserService {
	return &UserService{queries: queries, tx: tx}
}

func (s *UserService) List(ctx context.Context, page, pageSize int32) (*PaginatedUsers, error) {
	total, err := s.queries.CountUsers(ctx)
	if err != nil {
		return nil, errs.WrapDatabaseError(err)
	}

	users, err := s.queries.ListUsers(ctx, db.ListUsersParams{
		PageLimit:  int64(pageSize),
		PageOffset: int64(page-1) * int64(pageSize),
	})
	if err != nil {
		return nil, errs.WrapDatabaseError(err)
	}

	return &PaginatedUsers{Data: users, Total: total}, nil
}

func (s *UserService) Create(ctx context.Context, req CreateUserRequest) (*db.User, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user, err := s.queries.CreateUser(ctx, db.CreateUserParams{
		Email:    internal.NormalizeEmail(req.Email),
		Name:     req.Name,
		Password: string(hashedPassword),
		IsAdmin:  req.IsAdmin,
		// The admin creating the account vouches for the address
		EmailVerified: true,
	})
	if err != nil {
		if domainErr := errs.DomainErrorFromUniqueViolation(err); domainErr != nil {
			return nil, ErrUserAlreadyExists
		}
		return nil, errs.WrapDatabaseError(err)
	}
	return &user, nil
}

// Update changes name, email and the admin flag. The lockout check and the writes share
// one transaction, so two admins demoting each other at once cannot leave zero admins.
func (s *UserService) Update(ctx context.Context, actor middleware.Actor, id int64, req UpdateUserRequest) (*db.User, error) {
	var user db.User
	err := s.tx.WithTx(ctx, func(q *db.Queries) error {
		if err := checkLockout(ctx, q, actor, id, false, req.IsAdmin); err != nil {
			return err
		}
		if err := q.SetUserAdmin(ctx, db.SetUserAdminParams{ID: id, IsAdmin: req.IsAdmin}); err != nil {
			return errs.WrapDatabaseError(err)
		}
		var err error
		user, err = q.UpdateUserProfile(ctx, db.UpdateUserProfileParams{
			ID:           id,
			Email:        internal.NormalizeEmail(req.Email),
			Name:         req.Name,
			KeepVerified: true,
		})
		if err != nil {
			if domainErr := errs.DomainErrorFromUniqueViolation(err); domainErr != nil {
				return ErrUserAlreadyExists
			}
			return errs.WrapDatabaseError(err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// SetPassword replaces the password and revokes the user's refresh tokens.
func (s *UserService) SetPassword(ctx context.Context, id int64, password string) error {
	if _, err := s.queries.GetUserByID(ctx, id); err != nil {
		return ErrUserNotFound
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	return s.tx.WithTx(ctx, func(q *db.Queries) error {
		if err := q.UpdateUserPassword(ctx, db.UpdateUserPasswordParams{
			ID:       id,
			Password: string(hashedPassword),
		}); err != nil {
			return errs.WrapDatabaseError(err)
		}
		return errs.WrapDatabaseError(q.RevokeAllUserRefreshTokens(ctx, id))
	})
}

func (s *UserService) Delete(ctx context.Context, actor middleware.Actor, id int64) error {
	return s.tx.WithTx(ctx, func(q *db.Queries) error {
		if err := checkLockout(ctx, q, actor, id, true, false); err != nil {
			return err
		}
		return errs.WrapDatabaseError(q.DeleteUser(ctx, id))
	})
}

func checkLockout(ctx context.Context, q *db.Queries, actor middleware.Actor, id int64, deleting, adminAfter bool) error {
	target, err := q.GetUserByID(ctx, id)
	if err != nil {
		return ErrUserNotFound
	}
	admins, err := q.CountAdmins(ctx)
	if err != nil {
		return errs.WrapDatabaseError(err)
	}
	return Lockout(actor, target, deleting, adminAfter, admins)
}
