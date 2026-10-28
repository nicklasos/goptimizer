package unit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"app/internal/db"
	"app/internal/errs"
	"app/internal/factory"
	"app/internal/plans"
	"app/internal/runs"
	"app/tests/testkit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The admin panel runs the same cases (features/plans/restore.test.ts).
func TestRestoreTargetIsOwn(t *testing.T) {
	raw, err := os.ReadFile("testdata/own_target.json")
	require.NoError(t, err)
	var cases []struct {
		Name   string           `json:"name"`
		Kind   plans.SourceKind `json:"kind"`
		Source json.RawMessage  `json:"source"`
		Target json.RawMessage  `json:"target"`
		Own    bool             `json:"own"`
		Phrase string           `json:"phrase"`
	}
	require.NoError(t, json.Unmarshal(raw, &cases))
	require.NotEmpty(t, cases)
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			phrase, own, err := plans.RestoreTargetIsOwn(tc.Kind, tc.Source, tc.Target)
			require.NoError(t, err)
			assert.Equal(t, tc.Own, own)
			assert.Equal(t, tc.Phrase, phrase)
		})
	}
}

func planOf(t *testing.T, kind plans.SourceKind, config any) *db.Plan {
	t.Helper()
	raw, err := json.Marshal(config)
	require.NoError(t, err)
	return &db.Plan{Name: "Shop", SourceKind: string(kind), SourceConfig: string(raw)}
}

func target(t *testing.T, parsed *plans.ParsedTarget, into any) {
	t.Helper()
	require.NoError(t, json.Unmarshal(parsed.Config, into))
}

func TestParseRestoreTarget_Postgres(t *testing.T) {
	plan := planOf(t, plans.SourcePostgres, plans.PostgresConfig{Host: "db.internal", Port: 5432, User: "app", Database: "shop", SSLMode: "require"})
	env := plans.TargetEnv{}

	t.Run("a copy needs no confirmation and is created by default", func(t *testing.T) {
		parsed, err := plans.ParseRestoreTarget(plan, &plans.RestoreRequest{Postgres: &plans.PostgresTargetInput{Database: "shop_copy"}}, env)
		require.NoError(t, err)
		assert.False(t, parsed.Own)
		assert.Nil(t, parsed.Secret, "a blank password is the plan's")
		var got plans.PostgresTarget
		target(t, parsed, &got)
		assert.Equal(t, plans.PostgresTarget{
			PostgresConfig: plans.PostgresConfig{Host: "db.internal", Port: 5432, User: "app", Database: "shop_copy", SSLMode: "require"},
			CreateDatabase: true,
		}, got, "blank connection fields are the plan's")
	})

	t.Run("the plan's own database needs its name", func(t *testing.T) {
		req := &plans.RestoreRequest{Postgres: &plans.PostgresTargetInput{Database: "shop"}}
		_, err := plans.ParseRestoreTarget(plan, req, env)
		assert.Equal(t, errs.Fields{"confirm": {errs.ErrKeyRestoresConfirm}}, err)

		req.Confirm = "Shop"
		_, err = plans.ParseRestoreTarget(plan, req, env)
		assert.Error(t, err, "the name exactly")

		req.Confirm = "shop"
		parsed, err := plans.ParseRestoreTarget(plan, req, env)
		require.NoError(t, err)
		assert.True(t, parsed.Own)
		var got plans.PostgresTarget
		target(t, parsed, &got)
		assert.False(t, got.CreateDatabase, "the plan's database exists")
	})

	t.Run("the plan's server under another name is still its own", func(t *testing.T) {
		local := planOf(t, plans.SourcePostgres, plans.PostgresConfig{Host: "localhost", Port: 5432, User: "app", Database: "shop", SSLMode: "disable"})
		_, err := plans.ParseRestoreTarget(local, &plans.RestoreRequest{Postgres: &plans.PostgresTargetInput{Host: "127.0.0.1", Database: "shop"}}, env)
		assert.Equal(t, errs.Fields{"confirm": {errs.ErrKeyRestoresConfirm}}, err)
	})

	t.Run("another server with a password of its own", func(t *testing.T) {
		no := false
		parsed, err := plans.ParseRestoreTarget(plan, &plans.RestoreRequest{Postgres: &plans.PostgresTargetInput{
			Host: "staging.internal", Port: 6432, User: "restore", Database: "shop", Password: "s3cret", CreateDatabase: &no,
		}}, env)
		require.NoError(t, err)
		assert.False(t, parsed.Own)
		assert.JSONEq(t, `{"password":"s3cret"}`, string(parsed.Secret))
		var got plans.PostgresTarget
		target(t, parsed, &got)
		assert.Equal(t, "staging.internal", got.Host)
		assert.Equal(t, 6432, got.Port)
		assert.False(t, got.CreateDatabase)
	})

	t.Run("only the plan's kind", func(t *testing.T) {
		_, err := plans.ParseRestoreTarget(plan, &plans.RestoreRequest{SQLite: &plans.SQLiteTargetInput{Path: "/tmp/x.db"}}, env)
		assert.Equal(t, errs.Fields{"postgres": {"validation.postgres.required"}, "sqlite": {"validation.sqlite.invalid"}}, err)
	})
}

func TestParseRestoreTarget_MySQL(t *testing.T) {
	plan := planOf(t, plans.SourceMySQL, plans.MySQLConfig{Host: "localhost", Port: 3306, User: "app", Database: "blog"})

	_, err := plans.ParseRestoreTarget(plan, &plans.RestoreRequest{MySQL: &plans.MySQLTargetInput{Database: "blog"}}, plans.TargetEnv{})
	assert.Equal(t, errs.Fields{"confirm": {errs.ErrKeyRestoresConfirm}}, err)
	_, err = plans.ParseRestoreTarget(plan, &plans.RestoreRequest{MySQL: &plans.MySQLTargetInput{Database: "blog"}, Confirm: "blog"}, plans.TargetEnv{})
	assert.NoError(t, err)

	parsed, err := plans.ParseRestoreTarget(plan, &plans.RestoreRequest{MySQL: &plans.MySQLTargetInput{Database: "blog_copy"}}, plans.TargetEnv{})
	require.NoError(t, err)
	var got plans.MySQLTarget
	target(t, parsed, &got)
	assert.Equal(t, plans.MySQLTarget{MySQLConfig: plans.MySQLConfig{Host: "localhost", Port: 3306, User: "app", Database: "blog_copy"}, CreateDatabase: true}, got)
}

func TestParseRestoreTarget_SQLite(t *testing.T) {
	plan := planOf(t, plans.SourceSQLite, plans.SQLiteConfig{Path: "/srv/app/app.db"})
	env := plans.TargetEnv{DatabasePath: "/srv/gogobackup/data/gogobackup.db"}

	_, err := plans.ParseRestoreTarget(plan, &plans.RestoreRequest{SQLite: &plans.SQLiteTargetInput{Path: "/srv/app/./app.db"}}, env)
	assert.Equal(t, errs.Fields{"confirm": {errs.ErrKeyRestoresConfirm}}, err, "the same file, written differently")
	parsed, err := plans.ParseRestoreTarget(plan, &plans.RestoreRequest{SQLite: &plans.SQLiteTargetInput{Path: "/srv/app/app.db"}, Confirm: "/srv/app/app.db"}, env)
	require.NoError(t, err)
	assert.True(t, parsed.Own)

	parsed, err = plans.ParseRestoreTarget(plan, &plans.RestoreRequest{SQLite: &plans.SQLiteTargetInput{Path: "/srv/app/copy.db"}}, env)
	require.NoError(t, err)
	assert.False(t, parsed.Own)
	assert.JSONEq(t, `{"path":"/srv/app/copy.db"}`, string(parsed.Config))

	_, err = plans.ParseRestoreTarget(plan, &plans.RestoreRequest{SQLite: &plans.SQLiteTargetInput{Path: "copy.db"}}, env)
	assert.Equal(t, errs.FieldError("sqlite.path", "invalid"), err)
	_, err = plans.ParseRestoreTarget(plan, &plans.RestoreRequest{SQLite: &plans.SQLiteTargetInput{Path: env.DatabasePath}}, env)
	assert.Equal(t, errs.Fields{"sqlite.path": {errs.ErrKeyRestoresOwnDatabase}}, err, "never this service's own database")
}

func TestParseRestoreTarget_Files(t *testing.T) {
	plan := planOf(t, plans.SourceFiles, plans.FilesConfig{Paths: []string{"/srv/app/uploads"}, Excludes: []string{}})
	env := plans.TargetEnv{}

	_, err := plans.ParseRestoreTarget(plan, &plans.RestoreRequest{Files: &plans.FilesTargetInput{OriginalLocations: true}}, env)
	assert.Equal(t, errs.Fields{"confirm": {errs.ErrKeyRestoresConfirm}}, err)
	parsed, err := plans.ParseRestoreTarget(plan, &plans.RestoreRequest{Files: &plans.FilesTargetInput{OriginalLocations: true}, Confirm: plans.OriginalLocations}, env)
	require.NoError(t, err)
	assert.True(t, parsed.Own)

	parsed, err = plans.ParseRestoreTarget(plan, &plans.RestoreRequest{Files: &plans.FilesTargetInput{Directory: "/var/restores/uploads/"}}, env)
	require.NoError(t, err)
	assert.False(t, parsed.Own)
	assert.JSONEq(t, `{"directory":"/var/restores/uploads","original_locations":false}`, string(parsed.Config))

	for _, dir := range []string{"", "restores", "/"} {
		_, err = plans.ParseRestoreTarget(plan, &plans.RestoreRequest{Files: &plans.FilesTargetInput{Directory: dir}}, env)
		assert.Error(t, err, "%q", dir)
	}
}

func TestDefaultRestoreTarget(t *testing.T) {
	env := plans.TargetEnv{WorkDir: "/srv/gogobackup/work", Now: time.Date(2026, 10, 17, 9, 30, 0, 0, time.UTC)}

	files, err := plans.DefaultRestoreTarget(planOf(t, plans.SourceFiles, plans.FilesConfig{Paths: []string{"/srv/up"}}), env)
	require.NoError(t, err)
	assert.Equal(t, &plans.FilesTarget{Directory: "/srv/gogobackup/work/restores/shop-20261017T093000Z"}, files.Files)

	pg, err := plans.DefaultRestoreTarget(planOf(t, plans.SourcePostgres, plans.PostgresConfig{Host: "h", Port: 5432, User: "u", Database: "shop", SSLMode: "disable"}), env)
	require.NoError(t, err)
	assert.Equal(t, "shop", pg.Postgres.Database)
	assert.Equal(t, plans.SourcePostgres, pg.SourceKind)
}

func TestExtractLayout_IntoDirectory(t *testing.T) {
	layout := plans.IntoDirectory("/restore")

	_, rel, err := layout.Place("srv/app/uploads/a.txt")
	require.NoError(t, err)
	assert.Equal(t, filepath.FromSlash("srv/app/uploads/a.txt"), rel)
	_, rel, err = layout.Place("srv/app/uploads/")
	require.NoError(t, err)
	assert.Equal(t, filepath.FromSlash("srv/app/uploads"), rel)

	for _, name := range []string{"../etc/passwd", "srv/../../etc/passwd", "srv/app/..", "/etc/passwd", "", "srv\\..\\etc"} {
		_, _, err := layout.Place(name)
		assert.Error(t, err, "%q is refused", name)
	}

	assert.True(t, layout.LinkAllowed("srv/app/current", "releases/5"))
	assert.True(t, layout.LinkAllowed("srv/app/current", "../app/releases/5"))
	assert.True(t, layout.LinkAllowed("srv/app/current", "/restore/srv/app/releases/5"))
	assert.False(t, layout.LinkAllowed("srv/app/current", "../../../etc"), "out of the target")
	assert.False(t, layout.LinkAllowed("srv/app/current", "/etc"), "absolute, outside the target")
	assert.False(t, layout.LinkAllowed("srv/app/current", "/srv/app/releases/5"), "the original location is outside the target")
	assert.False(t, layout.LinkAllowed("../escape", "a"), "the link itself climbs out")
}

func TestExtractLayout_ToOriginalLocations(t *testing.T) {
	layout := plans.ToOriginalLocations([]string{"/srv/app/uploads", "/etc/app.conf"})

	region, rel, err := layout.Place("srv/app/uploads/a/b.txt")
	require.NoError(t, err)
	assert.Equal(t, 0, region)
	assert.Equal(t, filepath.FromSlash("uploads/a/b.txt"), rel, "under the region's root, the plan path's parent")
	region, rel, err = layout.Place("etc/app.conf")
	require.NoError(t, err)
	assert.Equal(t, 1, region)
	assert.Equal(t, "app.conf", rel)

	for _, name := range []string{"etc/passwd", "srv/app/uploads-other/x", "srv/app/uploads/../secret", "srv/app"} {
		_, _, err := layout.Place(name)
		assert.Error(t, err, "%q is outside the plan's paths", name)
	}

	assert.True(t, layout.LinkAllowed("srv/app/uploads/latest", "/srv/app/uploads/2026/x.png"))
	assert.True(t, layout.LinkAllowed("srv/app/uploads/latest", "2026/x.png"))
	assert.False(t, layout.LinkAllowed("srv/app/uploads/latest", "../config.php"))
	assert.False(t, layout.LinkAllowed("etc/app.conf", "/etc/shadow"))
}

func TestParsePgArchiveList(t *testing.T) {
	list := ";\n; Archive created at 2026-10-17 10:00:00 UTC\n;     dbname: shop\n;     Format: CUSTOM\n" +
		";     Dumped from database version: 16.4\n;     Dumped by pg_dump version: 17.2\n;\n"
	archive, err := plans.ParsePgArchiveList(list)
	require.NoError(t, err)
	assert.Equal(t, plans.PgArchive{DumpedBy: 17, DumpedFrom: 16}, archive)

	_, err = plans.ParsePgArchiveList("pg_restore: error: not an archive")
	assert.Error(t, err)
}

func TestPgRestoreCompatible(t *testing.T) {
	archive := plans.PgArchive{DumpedBy: 17, DumpedFrom: 16}
	assert.NoError(t, plans.PgRestoreCompatible("pg_restore (PostgreSQL) 17.2", archive, 16))
	assert.NoError(t, plans.PgRestoreCompatible("pg_restore (PostgreSQL) 18.0", archive, 17), "a newer pg_restore and server")

	err := plans.PgRestoreCompatible("pg_restore (PostgreSQL) 16.4", archive, 16)
	assert.EqualError(t, err, "pg_restore 16 is older than the pg_dump 17 that made the archive and cannot read it: install pg_restore 17 or newer")
	err = plans.PgRestoreCompatible("pg_restore (PostgreSQL) 17.2", archive, 15)
	assert.EqualError(t, err, "the archive comes from PostgreSQL 16 and the target server runs PostgreSQL 15: restore into a server of version 16 or newer")
	assert.Error(t, plans.PgRestoreCompatible("garbage", archive, 16))
}

func TestPgRestoreCommand(t *testing.T) {
	args, env := plans.PgRestoreCommand(plans.PostgresConfig{Host: "db", Port: 5432, User: "app", Database: "shop_copy", SSLMode: "require"}, "s3cret", 4, "/work/archive")
	assert.Equal(t, []string{
		"--clean", "--if-exists", "--no-owner", "--no-acl", "--exit-on-error", "--verbose", "--jobs=4",
		"--no-password", "--host=db", "--port=5432", "--username=app", "--dbname=shop_copy", "/work/archive",
	}, args)
	assert.Contains(t, env, "PGPASSWORD=s3cret")
	assert.NotContains(t, args, "s3cret")
}

// A plan runs one job at a time: a run waits for a restore of its plan, and a restore for a run.
func TestClaim_OneJobPerPlan(t *testing.T) {
	app := testkit.New(t)
	storage := app.Create.Storage()
	busy := app.Create.Plan(factory.WithStorages(storage))
	other := app.Create.Plan(factory.WithStorages(storage))
	archive := app.Create.Archive(app.Create.Run(busy, factory.WithRunStatus(runs.StatusSucceeded)), storage)
	now := time.Now()

	running := app.Create.Restore(archive, factory.WithRestoreStatus(runs.StatusRunning))
	queuedRun := app.Create.Run(busy)
	otherRun := app.Create.Run(other)

	ok, err := runs.Claim(app.Ctx, app.Queries, queuedRun.ID, now)
	require.NoError(t, err)
	assert.False(t, ok, "a run waits while a restore of its plan runs")
	ok, err = runs.Claim(app.Ctx, app.Queries, otherRun.ID, now)
	require.NoError(t, err)
	assert.True(t, ok, "another plan is not held up")

	queuedRestore := app.Create.Restore(archive)
	ok, err = runs.ClaimRestore(app.Ctx, app.Queries, queuedRestore.ID, now)
	require.NoError(t, err)
	assert.False(t, ok, "two restores of one plan never overlap")

	jobs, err := app.Queries.ListClaimableJobs(app.Ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, jobs, "nothing of the busy plan is offered")

	_, err = runs.FinishRestore(app.Ctx, app.Queries, running.ID, nil, "", now)
	require.NoError(t, err)
	jobs, err = app.Queries.ListClaimableJobs(app.Ctx, 10)
	require.NoError(t, err)
	require.Len(t, jobs, 2)
	assert.ElementsMatch(t, []string{"run", "restore"}, []string{jobs[0].Kind, jobs[1].Kind}, "both are offered once the plan is free")

	ok, err = runs.Claim(app.Ctx, app.Queries, queuedRun.ID, now)
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = runs.ClaimRestore(app.Ctx, app.Queries, queuedRestore.ID, now)
	require.NoError(t, err)
	assert.False(t, ok, "a restore waits while a run of its plan runs")
}
