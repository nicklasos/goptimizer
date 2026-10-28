package integration

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"app/internal/factory"
	"app/internal/plans"
	"app/internal/runs"
	"app/tests/testkit"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// serverURL is a database server for the tests that dump a real database, or a skip. The tests
// create a database of their own on it and drop it when they end.
func serverURL(t *testing.T, env string) *url.URL {
	t.Helper()
	raw := os.Getenv(env)
	if raw == "" {
		t.Skipf("%s is not set: set it to a server the test may create and drop databases on to dump a real database", env)
	}
	u, err := url.Parse(raw)
	require.NoError(t, err, env)
	return u
}

func throwawayName() string {
	b := make([]byte, 4)
	rand.Read(b)
	return "gogobackup_test_" + hex.EncodeToString(b)
}

func port(t *testing.T, u *url.URL, fallback int) int {
	t.Helper()
	if u.Port() == "" {
		return fallback
	}
	n, err := strconv.Atoi(u.Port())
	require.NoError(t, err)
	return n
}

func TestRuns_PostgresDump(t *testing.T) {
	server := serverURL(t, "TEST_POSTGRES_URL")
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, server.String())
	require.NoError(t, err)
	t.Cleanup(func() { admin.Close(context.Background()) })
	name := throwawayName()
	_, err = admin.Exec(ctx, "CREATE DATABASE "+name)
	require.NoError(t, err)
	t.Cleanup(func() { admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)") })

	target := *server
	target.Path = "/" + name
	conn, err := pgx.Connect(ctx, target.String())
	require.NoError(t, err)
	_, err = conn.Exec(ctx, "CREATE TABLE invoices (id serial PRIMARY KEY, customer text NOT NULL); INSERT INTO invoices (customer) VALUES ('Acme Corp'), ('Globex Ltd')")
	require.NoError(t, err)
	conn.Close(ctx)

	app := testkit.New(t)
	app.UseRealTool("pg_dump")
	user := app.Create.User(factory.Admin())
	password, _ := server.User.Password()
	sslMode := server.Query().Get("sslmode")
	if sslMode == "" {
		sslMode = "disable"
	}
	config := plans.PostgresConfig{Host: server.Hostname(), Port: port(t, server, 5432), User: server.User.Username(), Database: name, SSLMode: sslMode}
	dir := t.TempDir()
	plan := app.Create.Plan(factory.PostgresSource(config, password), factory.WithStorages(app.Create.Storage(factory.LocalStorage(dir))))

	run := runPlan(t, app, user, plan)

	require.Equal(t, runs.StatusSucceeded, run.Status, run.Log)
	require.Len(t, run.Archives, 1)
	archive := filepath.Join(dir, run.Archives[0].ObjectKey)
	assert.Regexp(t, `\.dump$`, archive)
	pgRestore, err := exec.LookPath("pg_restore")
	require.NoError(t, err, "pg_restore reads the archive back")
	list, err := exec.Command(pgRestore, "--list", archive).CombinedOutput()
	require.NoError(t, err, string(list))
	assert.Contains(t, string(list), "TABLE DATA public invoices")
	data, err := exec.Command(pgRestore, "--data-only", "--table=invoices", "--file=-", archive).CombinedOutput()
	require.NoError(t, err, string(data))
	assert.Contains(t, string(data), "Acme Corp")
	assert.Contains(t, string(data), "Globex Ltd")
	t.Logf("pg_dump archive %s, %d bytes; pg_restore --list:\n%s", run.Archives[0].ObjectKey, run.Archives[0].SizeBytes, list)
	t.Logf("run log:\n%s", run.Log)

	t.Run("an older pg_dump than the server fails clearly", func(t *testing.T) {
		real, err := app.Tools.Find(app.Ctx, "pg_dump")
		require.NoError(t, err)
		app.Tools.Install("pg_dump", real.Path, "pg_dump (PostgreSQL) 9.6.24")
		failed := runPlan(t, app, user, plan)

		require.Equal(t, runs.StatusFailed, failed.Status)
		assert.Contains(t, *failed.Error, "pg_dump 9 is older than the server")
	})
}

func TestRuns_MySQLDump(t *testing.T) {
	server := serverURL(t, "TEST_MYSQL_URL")
	password, _ := server.User.Password()
	cfg := mysql.NewConfig()
	cfg.User = server.User.Username()
	cfg.Passwd = password
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(server.Hostname(), strconv.Itoa(port(t, server, 3306)))
	cfg.MultiStatements = true
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	require.NoError(t, err)
	t.Cleanup(func() { admin.Close() })
	name := throwawayName()
	_, err = admin.Exec("CREATE DATABASE " + name)
	require.NoError(t, err)
	t.Cleanup(func() { admin.Exec("DROP DATABASE IF EXISTS " + name) })
	_, err = admin.Exec("CREATE TABLE " + name + ".orders (id INT AUTO_INCREMENT PRIMARY KEY, item VARCHAR(50), photo BLOB); " +
		"INSERT INTO " + name + ".orders (item, photo) VALUES ('Blue widget', x'00ff10'), ('Red gadget', NULL)")
	require.NoError(t, err)

	app := testkit.New(t)
	app.UseRealTool("mysqldump")
	user := app.Create.User(factory.Admin())
	config := plans.MySQLConfig{Host: server.Hostname(), Port: port(t, server, 3306), User: server.User.Username(), Database: name}
	dir := t.TempDir()
	plan := app.Create.Plan(factory.MySQLSource(config, password), factory.WithStorages(app.Create.Storage(factory.LocalStorage(dir))))

	run := runPlan(t, app, user, plan)

	require.Equal(t, runs.StatusSucceeded, run.Status, run.Log)
	require.Len(t, run.Archives, 1)
	data, err := os.ReadFile(filepath.Join(dir, run.Archives[0].ObjectKey))
	require.NoError(t, err)
	dump := string(gunzip(t, data))
	assert.Contains(t, dump, "CREATE TABLE `orders`")
	assert.Contains(t, dump, "INSERT INTO `orders`")
	assert.Contains(t, dump, "'Blue widget',0x00FF10")
	assert.Contains(t, dump, "'Red gadget',NULL")
	if password != "" {
		assert.NotContains(t, run.Log, password, "the password stays out of the log")
	}
	t.Logf("mysqldump archive %s, %d bytes compressed, %d bytes of SQL", run.Archives[0].ObjectKey, run.Archives[0].SizeBytes, len(dump))
	t.Logf("run log:\n%s", run.Log)
}

func pgRows(t *testing.T, server url.URL, database string) []string {
	t.Helper()
	server.Path = "/" + database
	conn, err := pgx.Connect(context.Background(), server.String())
	require.NoError(t, err)
	defer conn.Close(context.Background())
	rows, err := conn.Query(context.Background(), "SELECT customer FROM invoices ORDER BY id")
	require.NoError(t, err)
	customers, err := pgx.CollectRows(rows, pgx.RowTo[string])
	require.NoError(t, err)
	return customers
}

func TestRestores_Postgres(t *testing.T) {
	server := serverURL(t, "TEST_POSTGRES_URL")
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, server.String())
	require.NoError(t, err)
	t.Cleanup(func() { admin.Close(context.Background()) })
	name := throwawayName()
	copyName := name + "_copy"
	_, err = admin.Exec(ctx, "CREATE DATABASE "+name)
	require.NoError(t, err)
	t.Cleanup(func() {
		admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
		admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+copyName+" WITH (FORCE)")
	})
	source := *server
	source.Path = "/" + name
	conn, err := pgx.Connect(ctx, source.String())
	require.NoError(t, err)
	_, err = conn.Exec(ctx, "CREATE TABLE invoices (id serial PRIMARY KEY, customer text NOT NULL); INSERT INTO invoices (customer) VALUES ('Acme Corp'), ('Globex Ltd')")
	require.NoError(t, err)
	conn.Close(ctx)

	app := testkit.New(t)
	app.UseRealTool("pg_dump")
	app.UseRealTool("pg_restore")
	user := app.Create.User(factory.Admin())
	password, _ := server.User.Password()
	sslMode := server.Query().Get("sslmode")
	if sslMode == "" {
		sslMode = "disable"
	}
	config := plans.PostgresConfig{Host: server.Hostname(), Port: port(t, server, 5432), User: server.User.Username(), Database: name, SSLMode: sslMode}
	plan := app.Create.Plan(factory.PostgresSource(config, password), factory.WithStorages(app.Create.Storage(factory.LocalStorage(t.TempDir()))))
	backup := runPlan(t, app, user, plan)
	require.Equal(t, runs.StatusSucceeded, backup.Status, backup.Log)
	archive := backup.Archives[0]

	copied := restore(t, app, user, archive.ID, plans.RestoreRequest{Postgres: &plans.PostgresTargetInput{Database: copyName}})

	require.Equal(t, runs.StatusSucceeded, copied.Status, copied.Log)
	assert.True(t, copied.Target.Postgres.CreateDatabase)
	assert.Contains(t, copied.Log, "Created the database "+copyName)
	assert.Contains(t, copied.Log, "with 2 parallel jobs")
	assert.Contains(t, copied.Log, "pg_restore: entering main parallel loop")
	assert.NotContains(t, copied.Log, "pg_restore: pg_restore:")
	assert.Equal(t, []string{"Acme Corp", "Globex Ltd"}, pgRows(t, *server, copyName))
	if password != "" {
		assert.NotContains(t, copied.Log, password)
	}
	t.Logf("restore into a new database:\n%s", copied.Log)

	conn, err = pgx.Connect(ctx, source.String())
	require.NoError(t, err)
	_, err = conn.Exec(ctx, "UPDATE invoices SET customer = 'Changed' WHERE id = 1; INSERT INTO invoices (customer) VALUES ('After the backup')")
	require.NoError(t, err)
	conn.Close(ctx)

	body := plans.RestoreRequest{Postgres: &plans.PostgresTargetInput{Database: name}}
	app.As(user).POST("/api/v1/archives/%d/restore", body, archive.ID).AssertFieldError("confirm", "restores.confirm_mismatch")
	body.Confirm = name
	over := restore(t, app, user, archive.ID, body)

	require.Equal(t, runs.StatusSucceeded, over.Status, over.Log)
	assert.False(t, over.Target.Postgres.CreateDatabase)
	assert.Equal(t, []string{"Acme Corp", "Globex Ltd"}, pgRows(t, *server, name), "the original is back as it was backed up")
	t.Logf("restore over the original:\n%s", over.Log)

	t.Run("a missing database without create_database fails clearly", func(t *testing.T) {
		no := false
		failed := restore(t, app, user, archive.ID, plans.RestoreRequest{Postgres: &plans.PostgresTargetInput{Database: name + "_missing", CreateDatabase: &no}})
		require.Equal(t, runs.StatusFailed, failed.Status)
		assert.Contains(t, *failed.Error, "does not exist")
	})

	t.Run("an older pg_restore than the archive fails clearly", func(t *testing.T) {
		real, err := app.Tools.Find(app.Ctx, "pg_restore")
		require.NoError(t, err)
		app.Tools.Install("pg_restore", real.Path, "pg_restore (PostgreSQL) 12.0")
		t.Cleanup(func() { app.Tools.Install("pg_restore", real.Path, real.Version) })
		failed := restore(t, app, user, archive.ID, plans.RestoreRequest{Postgres: &plans.PostgresTargetInput{Database: copyName}})
		require.Equal(t, runs.StatusFailed, failed.Status)
		assert.Contains(t, *failed.Error, "pg_restore 12 is older than the pg_dump")
	})
}

func mysqlOrders(t *testing.T, admin *sql.DB, database string) []string {
	t.Helper()
	rows, err := admin.Query("SELECT item FROM " + database + ".orders ORDER BY id")
	require.NoError(t, err)
	defer rows.Close()
	var items []string
	for rows.Next() {
		var item string
		require.NoError(t, rows.Scan(&item))
		items = append(items, item)
	}
	require.NoError(t, rows.Err())
	return items
}

func TestRestores_MySQL(t *testing.T) {
	server := serverURL(t, "TEST_MYSQL_URL")
	password, _ := server.User.Password()
	cfg := mysql.NewConfig()
	cfg.User = server.User.Username()
	cfg.Passwd = password
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(server.Hostname(), strconv.Itoa(port(t, server, 3306)))
	cfg.MultiStatements = true
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	require.NoError(t, err)
	t.Cleanup(func() { admin.Close() })
	name := throwawayName()
	copyName := name + "_copy"
	_, err = admin.Exec("CREATE DATABASE " + name)
	require.NoError(t, err)
	t.Cleanup(func() {
		admin.Exec("DROP DATABASE IF EXISTS " + name)
		admin.Exec("DROP DATABASE IF EXISTS " + copyName)
	})
	_, err = admin.Exec("CREATE TABLE " + name + ".orders (id INT AUTO_INCREMENT PRIMARY KEY, item VARCHAR(50), photo BLOB); " +
		"INSERT INTO " + name + ".orders (item, photo) VALUES ('Blue widget', x'00ff10'), ('Red gadget', NULL)")
	require.NoError(t, err)

	app := testkit.New(t)
	app.UseRealTool("mysqldump")
	app.UseRealTool("mysql")
	user := app.Create.User(factory.Admin())
	config := plans.MySQLConfig{Host: server.Hostname(), Port: port(t, server, 3306), User: server.User.Username(), Database: name}
	storageDir := t.TempDir()
	plan := app.Create.Plan(factory.MySQLSource(config, password), factory.WithStorages(app.Create.Storage(factory.LocalStorage(storageDir))))
	backup := runPlan(t, app, user, plan)
	require.Equal(t, runs.StatusSucceeded, backup.Status, backup.Log)
	archive := backup.Archives[0]

	copied := restore(t, app, user, archive.ID, plans.RestoreRequest{MySQL: &plans.MySQLTargetInput{Database: copyName}})

	require.Equal(t, runs.StatusSucceeded, copied.Status, copied.Log)
	assert.Contains(t, copied.Log, "Created the database "+copyName)
	assert.Equal(t, []string{"Blue widget", "Red gadget"}, mysqlOrders(t, admin, copyName))
	var photo []byte
	require.NoError(t, admin.QueryRow("SELECT photo FROM "+copyName+".orders WHERE id = 1").Scan(&photo))
	assert.Equal(t, []byte{0x00, 0xff, 0x10}, photo)
	if password != "" {
		assert.NotContains(t, copied.Log, password)
	}
	t.Logf("restore into a new database:\n%s", copied.Log)

	_, err = admin.Exec("UPDATE " + name + ".orders SET item = 'Changed' WHERE id = 1; INSERT INTO " + name + ".orders (item) VALUES ('After the backup')")
	require.NoError(t, err)
	body := plans.RestoreRequest{MySQL: &plans.MySQLTargetInput{Database: name}}
	app.As(user).POST("/api/v1/archives/%d/restore", body, archive.ID).AssertFieldError("confirm", "restores.confirm_mismatch")
	body.Confirm = name
	over := restore(t, app, user, archive.ID, body)

	require.Equal(t, runs.StatusSucceeded, over.Status, over.Log)
	assert.Equal(t, []string{"Blue widget", "Red gadget"}, mysqlOrders(t, admin, name), "the original is back as it was backed up")
	t.Logf("restore over the original:\n%s", over.Log)

	_, err = admin.Exec("UPDATE " + name + ".orders SET item = 'Changed again' WHERE id = 1")
	require.NoError(t, err)
	object := filepath.Join(storageDir, archive.ObjectKey)
	data, err := os.ReadFile(object)
	require.NoError(t, err)
	// The gzip trailer: every statement still decompresses, so only the checksum can tell.
	data[len(data)-6] ^= 0xff
	require.NoError(t, os.WriteFile(object, data, 0o644))

	damaged := restore(t, app, user, archive.ID, body)

	require.Equal(t, runs.StatusFailed, damaged.Status, damaged.Log)
	assert.Equal(t, []string{"Changed again", "Red gadget"}, mysqlOrders(t, admin, name), "a damaged archive never reaches mysql")
}
