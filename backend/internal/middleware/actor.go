package middleware

import "github.com/gin-gonic/gin"

// Actor is who is making the request: the input of every policy function.
// Handlers get it from CurrentActor and pass it to the service.
type Actor struct {
	ID      int64
	IsAdmin bool
}

// CurrentActor returns the authenticated user. When there is none it answers 401
// itself and returns false, so a handler only has to return.
func CurrentActor(c *gin.Context) (Actor, bool) {
	userID, ok := CurrentUserID(c)
	if !ok {
		return Actor{}, false
	}
	return Actor{ID: userID, IsAdmin: c.GetBool("user_is_admin")}, true
}
