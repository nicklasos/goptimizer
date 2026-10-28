package integration

import (
	"fmt"
	"net/http"
	"testing"

	"app/internal/errs"
	"app/internal/factory"
	"app/internal/users"
	"app/tests/testkit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createUserBody(email string, isAdmin bool) string {
	return fmt.Sprintf(`{"email": %q, "name": "New User", "password": "password123", "is_admin": %t}`, email, isAdmin)
}

func TestUsersAPI_List(t *testing.T) {
	t.Run("admin lists every account with pagination meta", func(t *testing.T) {
		app := testkit.New(t)
		admin := app.Create.User(factory.Admin())
		app.Create.User()
		app.Create.User()

		resp := app.As(admin).GET("/api/v1/users?page=1&page_size=2").AssertOK()

		var response users.PaginatedUsersResponse
		resp.JSON(&response)
		assert.Len(t, response.Data, 2)
		assert.Equal(t, int64(3), response.Pagination.Total)
		assert.Equal(t, int32(2), response.Pagination.PerPage)

		app.As(admin).GET("/api/v1/users?page=2&page_size=2").AssertOK().
			AssertJSONCount("data", 1).
			AssertJSONPath("data.0.id", float64(admin.ID)).
			AssertJSONPath("data.0.is_admin", true)
	})

	t.Run("plain user and anonymous are rejected", func(t *testing.T) {
		app := testkit.New(t)
		user := app.Create.User()

		app.As(user).GET("/api/v1/users").AssertForbidden()
		app.GET("/api/v1/users").AssertUnauthorized()
	})
}

func TestUsersAPI_Create(t *testing.T) {
	t.Run("admin creates an admin and a user", func(t *testing.T) {
		app := testkit.New(t)
		admin := app.Create.User(factory.Admin())

		app.As(admin).POST("/api/v1/users", createUserBody("boss@example.com", true)).AssertOK().
			AssertJSONPath("data.email", "boss@example.com").
			AssertJSONPath("data.is_admin", true)
		app.As(admin).POST("/api/v1/users", createUserBody("plain@example.com", false)).AssertOK().
			AssertJSONPath("data.is_admin", false)

		app.AssertDatabaseHas("users", testkit.Row{"email": "boss@example.com", "is_admin": true})
		app.AssertDatabaseHas("users", testkit.Row{"email": "plain@example.com", "is_admin": false})
	})

	t.Run("plain user cannot create anyone", func(t *testing.T) {
		app := testkit.New(t)
		user := app.Create.User()

		app.As(user).POST("/api/v1/users", createUserBody("x@example.com", true)).AssertForbidden()
		app.AssertDatabaseMissing("users", testkit.Row{"email": "x@example.com"})
	})

	t.Run("rejects a short password and a duplicate email", func(t *testing.T) {
		app := testkit.New(t)
		admin := app.Create.User(factory.Admin())

		app.As(admin).POST("/api/v1/users", `{"email": "p@example.com", "name": "P", "password": "short", "is_admin": false}`).AssertInvalid("password")
		app.As(admin).POST("/api/v1/users", createUserBody(admin.Email, false)).AssertError(http.StatusBadRequest, errs.ErrKeyAuthUserExists)
	})
}

func TestUsersAPI_Update(t *testing.T) {
	t.Run("admin edits, promotes and demotes other accounts", func(t *testing.T) {
		app := testkit.New(t)
		admin := app.Create.User(factory.Admin())
		plain := app.Create.User()
		otherAdmin := app.Create.User(factory.Admin())

		app.As(admin).PUT("/api/v1/users/%d", `{"email": "renamed@example.com", "name": "Renamed", "is_admin": true}`, plain.ID).AssertOK().
			AssertJSONPath("data.email", "renamed@example.com").
			AssertJSONPath("data.name", "Renamed").
			AssertJSONPath("data.is_admin", true)

		app.As(admin).PUT("/api/v1/users/%d", `{"email": "demoted@example.com", "name": "Demoted", "is_admin": false}`, otherAdmin.ID).AssertOK().
			AssertJSONPath("data.is_admin", false)

		app.AssertDatabaseHas("users", testkit.Row{"id": plain.ID, "is_admin": true})
		app.AssertDatabaseHas("users", testkit.Row{"id": otherAdmin.ID, "is_admin": false})
	})

	t.Run("admin edits themselves but cannot remove their own admin flag", func(t *testing.T) {
		app := testkit.New(t)
		admin := app.Create.User(factory.Admin())
		app.Create.User(factory.Admin())

		app.As(admin).PUT("/api/v1/users/%d", `{"email": "me@example.com", "name": "Me", "is_admin": true}`, admin.ID).AssertOK()
		app.As(admin).PUT("/api/v1/users/%d", `{"email": "me@example.com", "name": "Me", "is_admin": false}`, admin.ID).
			AssertError(http.StatusBadRequest, errs.ErrKeyUsersCannotDemoteSelf)

		app.AssertDatabaseHas("users", testkit.Row{"id": admin.ID, "is_admin": true})
	})

	t.Run("returns 404 for an unknown user", func(t *testing.T) {
		app := testkit.New(t)
		admin := app.Create.User(factory.Admin())

		app.As(admin).PUT("/api/v1/users/2147483647", `{"email": "x@example.com", "name": "X", "is_admin": false}`).AssertError(http.StatusNotFound, errs.ErrKeyUsersNotFound)
	})
}

func TestUsersAPI_SetPassword(t *testing.T) {
	app := testkit.New(t)
	admin := app.Create.User(factory.Admin())
	target := app.Create.User(factory.Admin())

	var session struct {
		Data struct {
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	login(app, target.Email, "password123").JSON(&session)
	require.NotEmpty(t, session.Data.RefreshToken)

	app.As(admin).POST("/api/v1/users/%d/set-password", `{"password": "new-password-1"}`, target.ID).AssertOK()

	login(app, target.Email, "password123").AssertUnauthorized()
	login(app, target.Email, "new-password-1").AssertOK()

	app.POST("/api/v1/auth/refresh", fmt.Sprintf(`{"refresh_token": %q}`, session.Data.RefreshToken)).AssertUnauthorized()
}

func TestUsersAPI_Delete(t *testing.T) {
	app := testkit.New(t)
	admin := app.Create.User(factory.Admin())
	plain := app.Create.User()
	otherAdmin := app.Create.User(factory.Admin())

	app.As(admin).DELETE("/api/v1/users/%d", plain.ID).AssertOK()
	app.As(admin).DELETE("/api/v1/users/%d", otherAdmin.ID).AssertOK()
	app.AssertDatabaseMissing("users", testkit.Row{"id": plain.ID})
	app.AssertDatabaseMissing("users", testkit.Row{"id": otherAdmin.ID})

	app.As(admin).DELETE("/api/v1/users/%d", admin.ID).AssertError(http.StatusBadRequest, errs.ErrKeyUsersCannotDeleteSelf)
	app.AssertDatabaseHas("users", testkit.Row{"id": admin.ID})
}

// Two admins acting at once each see the other as an admin. The service runs their
// requests one after the other; the second must not leave zero admins.
func TestUserService_LastAdminSurvivesCrossedRequests(t *testing.T) {
	t.Run("demotions", func(t *testing.T) {
		app := testkit.New(t)
		a := app.Create.User(factory.Admin())
		b := app.Create.User(factory.Admin())
		service := users.NewUserService(app.Queries, app.TxRunner)

		_, err := service.Update(app.Ctx, testkit.Actor(a), b.ID, users.UpdateUserRequest{Email: b.Email, Name: b.Name, IsAdmin: false})
		require.NoError(t, err)
		_, err = service.Update(app.Ctx, testkit.Actor(b), a.ID, users.UpdateUserRequest{Email: a.Email, Name: a.Name, IsAdmin: false})

		assert.ErrorIs(t, err, users.ErrLastAdmin)
		app.AssertDatabaseHas("users", testkit.Row{"id": a.ID, "is_admin": true})
	})

	t.Run("deletions", func(t *testing.T) {
		app := testkit.New(t)
		a := app.Create.User(factory.Admin())
		b := app.Create.User(factory.Admin())
		service := users.NewUserService(app.Queries, app.TxRunner)

		require.NoError(t, service.Delete(app.Ctx, testkit.Actor(a), b.ID))
		err := service.Delete(app.Ctx, testkit.Actor(b), a.ID)

		assert.ErrorIs(t, err, users.ErrLastAdmin)
		app.AssertDatabaseHas("users", testkit.Row{"id": a.ID})
	})
}

func TestUsersAPI_DemotionIsImmediate(t *testing.T) {
	app := testkit.New(t)
	admin := app.Create.User(factory.Admin())
	other := app.Create.User(factory.Admin())
	app.As(other).GET("/api/v1/users").AssertOK()

	app.As(admin).PUT("/api/v1/users/%d", fmt.Sprintf(`{"email": %q, "name": "Other", "is_admin": false}`, other.Email), other.ID).AssertOK()

	app.As(other).GET("/api/v1/users").AssertForbidden()
	app.As(other).GET("/api/v1/folders").AssertOK()
}

func TestUsersAPI_DeletedUserTokenIsRejected(t *testing.T) {
	app := testkit.New(t)
	admin := app.Create.User(factory.Admin())
	target := app.Create.User(factory.Admin())
	app.As(target).GET("/api/v1/auth/me").AssertOK()

	app.As(admin).DELETE("/api/v1/users/%d", target.ID).AssertOK()

	app.As(target).GET("/api/v1/auth/me").AssertError(http.StatusUnauthorized, errs.ErrKeyAuthInvalidToken)
}
