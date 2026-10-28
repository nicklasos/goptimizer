package integration

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"testing"
	"time"

	"app/internal/db"
	"app/internal/errs"
	"app/internal/factory"
	"app/tests/testkit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

// Rows written by raw SQL store CURRENT_TIMESTAMP's format and rows written by Go store the
// driver's; every "has it expired" query must agree on both.
func TestSQLite_TimestampsCompareTheSameWhoeverWroteThem(t *testing.T) {
	resetPassword := func(app *testkit.App, token string) *testkit.Response {
		return app.POST("/api/v1/auth/reset-password", fmt.Sprintf(`{"token": %q, "password": "brand-new-password"}`, token))
	}
	insertResetToken := func(app *testkit.App, userID int64, token, expiresAt string) {
		_, err := app.DB.ExecContext(app.Ctx,
			"INSERT INTO auth_tokens (user_id, purpose, token_hash, expires_at) VALUES (?, 'password_reset', ?, datetime('now', ?))",
			userID, sha256Hex(token), expiresAt)
		require.NoError(t, err)
	}

	t.Run("an emailed token written by SQL is consumable until it expires", func(t *testing.T) {
		app := testkit.New(t)
		user := app.Create.User()
		insertResetToken(app, user.ID, "fresh-token", "+1 hour")

		resetPassword(app, "fresh-token").AssertOK()
		login(app, user.Email, "brand-new-password").AssertOK()
	})

	t.Run("an emailed token written by SQL is rejected after it expires", func(t *testing.T) {
		app := testkit.New(t)
		user := app.Create.User()
		insertResetToken(app, user.ID, "stale-token", "-1 hour")

		resetPassword(app, "stale-token").AssertError(http.StatusBadRequest, errs.ErrKeyAuthInvalidEmailToken)
		login(app, user.Email, "password123").AssertOK()
	})

	t.Run("a refresh token written by Go is accepted until it expires, in any time zone", func(t *testing.T) {
		app := testkit.New(t)
		user := app.Create.User()
		ahead := time.FixedZone("UTC+5", 5*60*60)
		behind := time.FixedZone("UTC-5", -5*60*60)

		store := func(raw string, expiresAt time.Time) {
			_, err := app.Queries.CreateRefreshToken(app.Ctx, db.CreateRefreshTokenParams{
				UserID: user.ID, Token: sha256Hex(raw), ExpiresAt: expiresAt,
			})
			require.NoError(t, err)
		}
		refresh := func(raw string) int {
			return app.POST("/api/v1/auth/refresh", fmt.Sprintf(`{"refresh_token": %q}`, raw)).StatusCode
		}

		store("expired-ahead", time.Now().Add(-time.Minute).In(ahead))
		store("expired-behind", time.Now().Add(-time.Minute).In(behind))
		store("valid-behind", time.Now().Add(time.Minute).In(behind))

		assert.Equal(t, http.StatusUnauthorized, refresh("expired-ahead"))
		assert.Equal(t, http.StatusUnauthorized, refresh("expired-behind"))
		assert.Equal(t, http.StatusOK, refresh("valid-behind"))
	})

	t.Run("the cleanup job deletes exactly the expired refresh tokens", func(t *testing.T) {
		app := testkit.New(t)
		user := app.Create.User()
		for raw, expiresAt := range map[string]time.Time{
			"old": time.Now().Add(-time.Hour),
			"new": time.Now().Add(time.Hour),
		} {
			_, err := app.Queries.CreateRefreshToken(app.Ctx, db.CreateRefreshTokenParams{
				UserID: user.ID, Token: sha256Hex(raw), ExpiresAt: expiresAt,
			})
			require.NoError(t, err)
		}

		require.NoError(t, app.Queries.DeleteExpiredRefreshTokens(app.Ctx))
		app.AssertDatabaseMissing("refresh_tokens", testkit.Row{"token": sha256Hex("old")})
		app.AssertDatabaseHas("refresh_tokens", testkit.Row{"token": sha256Hex("new")})
	})
}

// SQLite only enforces foreign keys on a connection that turned them on, so this fails if
// any connection the pool opens misses the pragma.
func TestSQLite_ForeignKeysHoldOnEveryConnection(t *testing.T) {
	app := testkit.New(t)
	storage := app.Create.Storage()
	kept := app.Create.Plan(factory.WithStorages(storage))
	removed := app.Create.Plan(factory.WithStorages(storage, app.Create.Storage()))

	app.DB.SetMaxIdleConns(0)
	require.NoError(t, app.Queries.DeletePlan(app.Ctx, removed.ID))
	app.AssertDatabaseCount("plan_storages", 0, testkit.Row{"plan_id": removed.ID})
	app.AssertDatabaseCount("plan_storages", 1, testkit.Row{"plan_id": kept.ID})

	err := app.Queries.DeleteStorage(app.Ctx, storage.ID)
	assert.True(t, errs.IsForeignKeyViolation(err), "a storage a plan uses is kept: %v", err)

	_, err = app.DB.ExecContext(app.Ctx, "INSERT INTO plan_storages (plan_id, storage_id) VALUES (?, ?)", removed.ID, storage.ID)
	assert.Error(t, err, "a link to a missing plan is refused")
}
