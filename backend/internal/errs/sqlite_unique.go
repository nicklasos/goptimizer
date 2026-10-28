package errs

import (
	"errors"
	"strings"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// DomainErrorFromUniqueViolation maps known unique constraints to client-facing errors.
func DomainErrorFromUniqueViolation(err error) *DomainError {
	if IsUniqueViolation(err, "users.email") || IsUniqueViolation(err, "idx_users_email_lower") {
		return NewBadRequestError(ErrKeyAuthUserExists, "User with this email already exists")
	}
	return nil
}

// IsUniqueViolation reports a unique constraint failure on a column ("refresh_tokens.token") or, for
// an expression index, on the index: SQLite names one or the other in the message.
func IsUniqueViolation(err error, column string) bool {
	var sqliteErr *sqlite.Error
	return errors.As(err, &sqliteErr) &&
		sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE &&
		strings.Contains(sqliteErr.Error(), column)
}

// IsForeignKeyViolation reports a write refused by a foreign key, such as deleting a row that
// another table still points at. SQLite reports ON DELETE RESTRICT with the trigger code, so
// this goes by the primary code and the message.
func IsForeignKeyViolation(err error) bool {
	var sqliteErr *sqlite.Error
	return errors.As(err, &sqliteErr) &&
		sqliteErr.Code()&0xff == sqlite3.SQLITE_CONSTRAINT &&
		strings.Contains(sqliteErr.Error(), "FOREIGN KEY constraint failed")
}
