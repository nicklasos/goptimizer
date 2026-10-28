# Error Handling Guide

Simple guide for using the `errs` package for consistent error handling.

## Quick Start

### 1. Define Errors in Services

```go
package users

import "app/internal/errs"

var (
    ErrUserNotFound = errs.NewNotFoundError(errs.ErrKeyUsersNotFound, "User not found")
)
```

### 2. Return Errors from Services

```go
func (s *UserService) Get(ctx context.Context, id int64) (*db.User, error) {
    user, err := s.queries.GetUserByID(ctx, id)
    if errors.Is(err, sql.ErrNoRows) {
        return nil, ErrUserNotFound
    }
    if err != nil {
        return nil, errs.WrapDatabaseError(err)
    }
    return &user, nil
}
```

### 3. Handle Errors in Handlers

```go
func (h *Handler) GetUser(c *gin.Context) {
    user, err := h.service.Get(c.Request.Context(), id)
    if err != nil {
        errs.RespondWithError(c, err)
        return
    }
    c.JSON(http.StatusOK, UserDataResponse{Data: userResponseFromDB(*user)})
}
```

## Error Types

```go
// Not Found (404)
errs.NewNotFoundError(key, message)

// Bad Request (400)
errs.NewBadRequestError(key, message)

// Unauthorized (401)
errs.NewUnauthorizedError(key, message)

// Forbidden (403)
errs.NewForbiddenError(key, message)

// Internal Error (500)
errs.NewInternalError(key, message)

// Any other status, such as 409 for users.last_admin
errs.NewDomainError(key, message, http.StatusConflict)
```

## Error Keys

Add error keys in `internal/errs/keys.go`:

```go
const (
    ErrKeyUsersNotFound = "users.not_found"
)
```

Keys are grouped by module (`auth.*`, `users.*`, `validation.*`). Clients translate them, so treat a key as part of the API contract.

Validation failures have their own shape and always return 400:

```json
{
  "message": "The given data was invalid.",
  "error_key": "validation.failed",
  "errors": { "email": ["validation.email.required"] }
}
```

A service reports such a failure itself with `errs.Fields`, which `RespondWithError` answers in the same shape:

```go
return nil, errs.FieldError("email", "invalid")              // validation.email.invalid
return nil, errs.Fields{"email": {errs.ErrKeyAuthUserExists}}
```

A field inside a nested block is named by its path, such as `address.city`, both from binding and from `errs.Fields`.

## Handler Helpers

```go
// Validation errors
errs.RespondWithValidationError(c, err)

// Quick responses
errs.RespondWithUnauthorized(c, "Unauthorized")
errs.RespondWithBadRequest(c, key, message)
```

## Wrapping Errors

```go
// Prefer WrapDatabaseError for sqlc errors (sql.ErrNoRows → 404, else 500)
if err != nil {
    return nil, errs.WrapDatabaseError(err)
}

// Or wrap with an explicit key/message
if err != nil {
    return nil, errs.WrapInternal(key, "Failed to create", err)
}

// Map unique constraint violations to stable domain errors
if domainErr := errs.DomainErrorFromUniqueViolation(err); domainErr != nil {
    return nil, ErrUserAlreadyExists
}
```

## Response Format

All errors return:

```json
{
  "error_key": "users.not_found",
  "message": "User not found",
  "status": 404,
  "timestamp": "2024-01-01T00:00:00Z"
}
```

## Reference

`internal/users/user_service.go` and `internal/users/handler.go` use most of the above.
