package errs

import (
	"errors"
	"net/http"

	"database/sql"
)

// WrapDatabaseError wraps common database errors into DomainErrors.
// sql.ErrNoRows becomes a 404; other errors become 500 with full error logged.
func WrapDatabaseError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return NewNotFoundError(ErrKeyNotFound, "resource not found")
	}
	return WrapInternal(ErrKeyInternalError, "database error", err)
}

// IsNotFound checks if error is a not found error (status 404)
func IsNotFound(err error) bool {
	domainErr := ExtractDomainError(err)
	return domainErr != nil && domainErr.Status == http.StatusNotFound
}
