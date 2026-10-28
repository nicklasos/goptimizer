package middleware

import (
	"context"

	"app/internal/errs"

	"github.com/gin-gonic/gin"
)

// UserAdminLookup reports whether the authenticated user is an admin right now.
type UserAdminLookup interface {
	IsUserAdmin(ctx context.Context, userID int64) (bool, error)
}

// UserAuth is JWT verification plus the admin lookup. AuthService implements this.
type UserAuth interface {
	UserJWTVerifier
	UserAdminLookup
}

// RequireAdmin lets through only admins, as loaded from the database by UserAuthMiddleware.
func RequireAdmin(c *gin.Context) {
	if _, exists := c.Get("user_is_admin"); !exists {
		errs.RespondWithUnauthorized(c, "Unauthorized")
		c.Abort()
		return
	}
	if !c.GetBool("user_is_admin") {
		errs.RespondWithError(c, errs.NewForbiddenError(errs.ErrKeyForbidden, "Forbidden"))
		c.Abort()
		return
	}
	c.Next()
}
