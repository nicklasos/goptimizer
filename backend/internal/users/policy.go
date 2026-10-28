package users

import (
	"app/internal/db"
	"app/internal/middleware"
)

// Lockout keeps someone able to manage users: an admin cannot delete themselves or
// remove their own admin flag, and no change may leave zero admins. adminAfter is the
// target's flag once the change is written (false for a delete); admins is the count before it.
// The frontend mirrors the self rules in features/users/policy.ts.
func Lockout(actor middleware.Actor, target db.User, deleting, adminAfter bool, admins int64) error {
	switch {
	case target.ID == actor.ID && deleting:
		return ErrCannotDeleteSelf
	case target.ID == actor.ID && !adminAfter:
		return ErrCannotDemoteSelf
	case target.IsAdmin && (deleting || !adminAfter) && admins <= 1:
		return ErrLastAdmin
	}
	return nil
}
