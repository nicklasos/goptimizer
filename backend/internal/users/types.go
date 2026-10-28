package users

import "app/internal"

// CreateUserRequest creates a user, an admin when is_admin is true
type CreateUserRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Name     string `json:"name" binding:"required"`
	Password string `json:"password" binding:"required,min=8"`
	IsAdmin  bool   `json:"is_admin"`
}

// UpdateUserRequest updates name, email and the admin flag
type UpdateUserRequest struct {
	Email   string `json:"email" binding:"required,email"`
	Name    string `json:"name" binding:"required"`
	IsAdmin bool   `json:"is_admin"`
}

// SetPasswordRequest sets a new password for a user
type SetPasswordRequest struct {
	Password string `json:"password" binding:"required,min=8"`
}

// UserResponse is a user without the password
type UserResponse struct {
	ID            int64  `json:"id"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	IsAdmin       bool   `json:"is_admin"`
	EmailVerified bool   `json:"email_verified"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// UserDataResponse wraps a user
type UserDataResponse struct {
	Data UserResponse `json:"data"`
}

// PaginatedUsersResponse wraps a page of users
type PaginatedUsersResponse struct {
	Data       []UserResponse          `json:"data"`
	Pagination internal.PaginationMeta `json:"pagination"`
}
