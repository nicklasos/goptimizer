package unit

import (
	"strings"
	"testing"
	"time"

	"app/internal/plans"
	"app/internal/runs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPgDumpCommand(t *testing.T) {
	args, env := plans.PgDumpCommand(plans.PostgresConfig{
		Host: "db.internal", Port: 5433, User: "app", Database: "shop", SSLMode: "require",
	}, "s3cret pass")

	assert.Equal(t, []string{
		"--format=custom", "--no-owner", "--no-acl", "--no-password",
		"--host=db.internal", "--port=5433", "--username=app", "--dbname=shop",
	}, args)
	assert.Equal(t, []string{"PGPASSWORD=s3cret pass", "PGSSLMODE=require", "PGCONNECT_TIMEOUT=10"}, env)
	assert.NotContains(t, strings.Join(args, " "), "s3cret", "the password is never an argument: ps would show it")
}

func TestMySQLDefaultsFile(t *testing.T) {
	config := plans.MySQLConfig{Host: "127.0.0.1", Port: 3306, User: "shop", Database: "shop"}
	cases := []struct {
		password string
		line     string
	}{
		{"plain", `password="plain"`},
		{"with # hash", `password="with # hash"`},
		{`has "double"`, `password='has "double"'`},
		{`back\slash`, `password="back\\slash"`},
		{"tab\tand\nnewline", `password="tab\tand\nnewline"`},
		{"", `password=""`},
	}
	for _, tc := range cases {
		t.Run(tc.password, func(t *testing.T) {
			content, err := plans.MySQLDefaultsFile(config, tc.password)
			require.NoError(t, err)
			assert.Equal(t, "[client]\nprotocol=TCP\nhost=\"127.0.0.1\"\nport=\"3306\"\nuser=\"shop\"\n"+tc.line+"\n", content)
		})
	}

	_, err := plans.MySQLDefaultsFile(config, `both ' and "`)
	assert.ErrorContains(t, err, "cannot be written to an option file")
}

func TestMySQLDumpArgs(t *testing.T) {
	args := plans.MySQLDumpArgs("/tmp/x.cnf", plans.MySQLConfig{Database: "shop"})
	require.NotEmpty(t, args)
	assert.Equal(t, "--defaults-extra-file=/tmp/x.cnf", args[0], "mysqldump reads it only as the first option")
	assert.Equal(t, "shop", args[len(args)-1])
	assert.Subset(t, args, []string{"--single-transaction", "--routines", "--triggers", "--events", "--hex-blob"})
}

func TestObjectKey(t *testing.T) {
	at := time.Date(2026, 10, 17, 3, 0, 5, 0, time.UTC)
	assert.Equal(t, "myproject-dev/myproject-dev-20261017T030005Z-42.dump", runs.ObjectKey("MyProject Dev", 42, at, "dump"))
	assert.Equal(t, "shop-db/shop-db-20261017T030005Z-7.sql.gz", runs.ObjectKey("  Shop / DB!  ", 7, at.In(time.FixedZone("x", 3600)), "sql.gz"))
	assert.Equal(t, "plan/plan-20261017T030005Z-1.tar.gz", runs.ObjectKey("Бекап", 1, at, "tar.gz"), "a name with no ASCII letters still makes a key")
}
