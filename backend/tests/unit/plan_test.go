package unit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"app/internal/plans"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The same cases run against the admin panel's port of NextRun (frontend/src/features/plans).
func TestNextRun(t *testing.T) {
	raw, err := os.ReadFile("testdata/next_run.json")
	require.NoError(t, err)
	var cases []struct {
		Name      string           `json:"name"`
		Schedules []plans.Schedule `json:"schedules"`
		After     time.Time        `json:"after"`
		Next      *time.Time       `json:"next"`
	}
	require.NoError(t, json.Unmarshal(raw, &cases))
	require.NotEmpty(t, cases)

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			next, ok := plans.NextRun(tc.Schedules, tc.After)
			if tc.Next == nil {
				assert.False(t, ok)
				return
			}
			require.True(t, ok)
			assert.Equal(t, tc.Next.UTC(), next)
		})
	}
}

func TestTimeOfDay(t *testing.T) {
	for input, want := range map[string]int{"00:00": 0, "03:00": 180, "18:30": 1110, "23:59": 1439} {
		at, ok := plans.ParseTimeOfDay(input)
		assert.True(t, ok, input)
		assert.Equal(t, plans.TimeOfDay(want), at, input)
		assert.Equal(t, input, at.String())
	}
	for _, input := range []string{"", "3:00", "24:00", "12:60", "12:00:00", "noon", " 03:00"} {
		_, ok := plans.ParseTimeOfDay(input)
		assert.False(t, ok, input)
	}
}

func TestPgDumpCompatible(t *testing.T) {
	cases := []struct {
		version string
		server  int
		problem string
	}{
		{"pg_dump (PostgreSQL) 17.2", 17, ""},
		{"pg_dump (PostgreSQL) 17.2", 15, ""},
		{"pg_dump (PostgreSQL) 16.4 (Debian 16.4-1.pgdg120+2)", 16, ""},
		{"pg_dump (PostgreSQL) 15.8", 16, "pg_dump 15 is older than the server (PostgreSQL 16)"},
		{"pg_dump (PostgreSQL) 9.6.24", 10, "pg_dump 9 is older than the server (PostgreSQL 10)"},
		{"pg_dump: unknown", 16, "could not read the version of pg_dump"},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s against %d", tc.version, tc.server), func(t *testing.T) {
			err := plans.PgDumpCompatible(tc.version, tc.server)
			if tc.problem == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.problem)
		})
	}
}

func TestConnectMessage(t *testing.T) {
	cases := map[string]struct {
		err  error
		want string
	}{
		"postgres answered": {
			fmt.Errorf("failed to connect: %w", &pgconn.PgError{Code: "28P01", Message: `password authentication failed for user "backup"`}),
			`password authentication failed for user "backup"`,
		},
		"mysql answered": {
			&mysql.MySQLError{Number: 1045, Message: "Access denied for user 'backup'@'localhost' (using password: YES)"},
			"Access denied for user 'backup'@'localhost' (using password: YES)",
		},
		"no answer in time":   {fmt.Errorf("dial: %w", context.DeadlineExceeded), "no answer within 10 seconds"},
		"network error as is": {errors.New("dial tcp 127.0.0.1:1: connect: connection refused"), "dial tcp 127.0.0.1:1: connect: connection refused"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, plans.ConnectMessage(tc.err))
		})
	}
}
