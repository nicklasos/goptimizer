package unit

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"app/internal/db"
	"app/internal/middleware"
	"app/internal/users"
)

func TestUsersPolicy_Lockout(t *testing.T) {
	actor := middleware.Actor{ID: 1, IsAdmin: true}
	self := db.User{ID: 1, IsAdmin: true}
	otherAdmin := db.User{ID: 2, IsAdmin: true}
	plain := db.User{ID: 3}

	cases := []struct {
		name       string
		target     db.User
		deleting   bool
		adminAfter bool
		admins     int64
		want       error
	}{
		{"an admin edits themselves and stays admin", self, false, true, 1, nil},
		{"an admin cannot remove their own admin flag", self, false, false, 2, users.ErrCannotDemoteSelf},
		{"an admin cannot delete themselves", self, true, false, 2, users.ErrCannotDeleteSelf},
		{"an admin demotes another admin", otherAdmin, false, false, 2, nil},
		{"an admin deletes another admin", otherAdmin, true, false, 2, nil},
		{"the last admin cannot be demoted", otherAdmin, false, false, 1, users.ErrLastAdmin},
		{"the last admin cannot be deleted", otherAdmin, true, false, 1, users.ErrLastAdmin},
		{"the last admin can be edited", otherAdmin, false, true, 1, nil},
		{"an admin promotes a user", plain, false, true, 1, nil},
		{"an admin deletes a user", plain, true, false, 1, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, users.Lockout(actor, tc.target, tc.deleting, tc.adminAfter, tc.admins))
		})
	}
}
