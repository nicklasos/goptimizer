package plans

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"app/internal/runlog"
	"app/internal/tools"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	_ "modernc.org/sqlite"
)

// pgMaintenanceDatabase is where a missing database is created from: it exists on every server.
const pgMaintenanceDatabase = "postgres"

// pgUndefinedDatabase is Postgres's error code for a database that does not exist.
const pgUndefinedDatabase = "3D000"

// restorePostgres downloads the archive, since pg_restore reads a custom-format archive in
// parallel only from a file, then restores it with --jobs into the target database, which it
// creates first when allowed.
func restorePostgres(ctx context.Context, job RestoreJob, log *runlog.Log) error {
	var c PostgresTarget
	var s DatabaseSecret
	if err := unmarshalSource(job.Target, &c, job.Secret, &s); err != nil {
		return err
	}
	tool, err := dumpTool(ctx, job.Tools, "pg_restore", log)
	if err != nil {
		return err
	}
	server, err := preparePostgres(ctx, c, s.Password, log)
	if err != nil {
		return err
	}
	path, err := job.Archive.download(ctx, job.WorkDir, log)
	if err != nil {
		return err
	}
	header, err := readPgArchive(ctx, tool, path)
	if err != nil {
		return err
	}
	log.Printf("The archive was made by pg_dump %d from PostgreSQL %d", header.DumpedBy, header.DumpedFrom)
	if err := PgRestoreCompatible(tool.Version, header, server.major); err != nil {
		return err
	}

	log.Printf("Restoring into database %s on %s:%d with %d parallel jobs", c.Database, c.Host, c.Port, job.Jobs)
	args, env := PgRestoreCommand(c.PostgresConfig, s.Password, job.Jobs, path)
	return runTool(ctx, tool, args, env, nil, io.Discard, log)
}

// PgRestoreCommand is the arguments and the added environment of pg_restore. It replaces what
// the archive holds (--clean --if-exists) and stops at the first error, so a failed restore
// fails the job instead of leaving a half-restored database that reports success.
func PgRestoreCommand(c PostgresConfig, password string, jobs int, archive string) (args, env []string) {
	args = []string{"--clean", "--if-exists", "--no-owner", "--no-acl", "--exit-on-error", "--verbose", "--jobs=" + strconv.Itoa(jobs)}
	args = append(args, pgConnectionArgs(c)...)
	return append(args, archive), pgEnv(c, password)
}

// preparePostgres connects to the target database, creating it first when it is missing and
// the restore may create it, and returns the server.
func preparePostgres(ctx context.Context, c PostgresTarget, password string, log *runlog.Log) (postgresServer, error) {
	ctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	conn, err := connectPostgres(ctx, c.PostgresConfig, password)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUndefinedDatabase {
		if !c.CreateDatabase {
			return postgresServer{}, fmt.Errorf("the database %s does not exist on %s:%d: create it, or restore with \"Create the database\" on", c.Database, c.Host, c.Port)
		}
		if err := createPostgresDatabase(ctx, c.PostgresConfig, password); err != nil {
			return postgresServer{}, err
		}
		log.Printf("Created the database %s", c.Database)
		conn, err = connectPostgres(ctx, c.PostgresConfig, password)
	}
	if err != nil {
		return postgresServer{}, fmt.Errorf("could not connect to %s:%d: %s", c.Host, c.Port, ConnectMessage(err))
	}
	defer conn.Close(context.Background())
	server, err := queryPostgresVersion(ctx, conn)
	if err != nil {
		return postgresServer{}, err
	}
	log.Printf("Connected to %s, database %s", server.version, c.Database)
	return server, nil
}

func createPostgresDatabase(ctx context.Context, c PostgresConfig, password string) error {
	maintenance := c
	maintenance.Database = pgMaintenanceDatabase
	conn, err := connectPostgres(ctx, maintenance, password)
	if err != nil {
		return fmt.Errorf("could not connect to %s:%d to create the database %s: %s", c.Host, c.Port, c.Database, ConnectMessage(err))
	}
	defer conn.Close(context.Background())
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{c.Database}.Sanitize()); err != nil {
		return fmt.Errorf("could not create the database %s: %s", c.Database, ConnectMessage(err))
	}
	return nil
}

// PgArchive is what pg_restore --list says about an archive: the major versions of the
// pg_dump that made it and of the server it was dumped from.
type PgArchive struct {
	DumpedBy   int
	DumpedFrom int
}

var (
	pgDumpedBy   = regexp.MustCompile(`(?m)^;\s*Dumped by pg_dump version:\s*(\d+)`)
	pgDumpedFrom = regexp.MustCompile(`(?m)^;\s*Dumped from database version:\s*(\d+)`)
)

// ParsePgArchiveList reads the header of `pg_restore --list` output.
func ParsePgArchiveList(list string) (PgArchive, error) {
	by := pgDumpedBy.FindStringSubmatch(list)
	from := pgDumpedFrom.FindStringSubmatch(list)
	if by == nil || from == nil {
		return PgArchive{}, errors.New("pg_restore --list did not show which pg_dump and server the archive comes from")
	}
	byMajor, _ := strconv.Atoi(by[1])
	fromMajor, _ := strconv.Atoi(from[1])
	return PgArchive{DumpedBy: byMajor, DumpedFrom: fromMajor}, nil
}

func readPgArchive(ctx context.Context, tool tools.Tool, path string) (PgArchive, error) {
	cmd := exec.CommandContext(ctx, tool.Path, "--list", path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.WaitDelay = toolExitDelay
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return PgArchive{}, ctx.Err()
	}
	if err != nil {
		return PgArchive{}, fmt.Errorf("%s cannot read the archive (an archive from a newer pg_dump needs a pg_restore at least as new): %s",
			tool.Version, strings.TrimSpace(stderr.String()))
	}
	return ParsePgArchiveList(string(out))
}

// PgRestoreCompatible is the rule for pg_restore, by its `--version` output: it must be at least
// as new as the pg_dump that made the archive, which writes a format older ones cannot read, and
// the target server at least as new as the one dumped, whose SQL an older server may not know.
func PgRestoreCompatible(version string, archive PgArchive, serverMajor int) error {
	toolMajor, err := strconv.Atoi(firstNumber.FindString(version))
	if err != nil {
		return fmt.Errorf("could not read the version of pg_restore from %q", version)
	}
	if toolMajor < archive.DumpedBy {
		return fmt.Errorf("pg_restore %d is older than the pg_dump %d that made the archive and cannot read it: install pg_restore %d or newer",
			toolMajor, archive.DumpedBy, archive.DumpedBy)
	}
	if serverMajor < archive.DumpedFrom {
		return fmt.Errorf("the archive comes from PostgreSQL %d and the target server runs PostgreSQL %d: restore into a server of version %d or newer",
			archive.DumpedFrom, serverMajor, archive.DumpedFrom)
	}
	return nil
}

// restoreMySQL streams the archive through gunzip into mysql. The checksum is known only at
// the end, so a corrupt archive fails the restore after mysql ran what it got.
func restoreMySQL(ctx context.Context, job RestoreJob, log *runlog.Log) error {
	var c MySQLTarget
	var s DatabaseSecret
	if err := unmarshalSource(job.Target, &c, job.Secret, &s); err != nil {
		return err
	}
	tool, err := dumpTool(ctx, job.Tools, "mysql", log)
	if err != nil {
		return err
	}
	if err := prepareMySQL(ctx, c, s.Password, log); err != nil {
		return err
	}
	defaults, err := MySQLDefaultsFile(c.MySQLConfig, s.Password)
	if err != nil {
		return err
	}
	path, err := writePrivateFile("gogobackup-mysql-*.cnf", defaults)
	if err != nil {
		return err
	}
	defer os.Remove(path)

	// Verified before mysql sees a byte: mysql applies statements as they arrive, so a
	// checksum checked at the end would only report damage it had already done.
	archive, err := job.Archive.download(ctx, job.WorkDir, log)
	if err != nil {
		return err
	}
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("the archive is not gzip: %w", err)
	}
	log.Printf("Restoring %s (%s) into database %s on %s:%d", job.Archive.Name, formatSize(job.Archive.Size), c.Database, c.Host, c.Port)
	return runTool(ctx, tool, MySQLRestoreArgs(path, c.Database), nil, gz, io.Discard, log)
}

// MySQLRestoreArgs is the arguments of mysql for a restore. The password is in the defaults
// file, which must come first. A dump of one database names none, so it runs in database.
func MySQLRestoreArgs(defaultsFile, database string) []string {
	return []string{"--defaults-extra-file=" + defaultsFile, "--database=" + database}
}

// prepareMySQL checks that the target database exists, creating it when it is missing and the
// restore may create it.
func prepareMySQL(ctx context.Context, c MySQLTarget, password string, log *runlog.Log) error {
	ctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	conn, err := openMySQL(c.MySQLConfig, password, "")
	if err != nil {
		return err
	}
	defer conn.Close()
	version, err := queryMySQLVersion(ctx, conn)
	if err != nil {
		return fmt.Errorf("could not connect to %s:%d: %s", c.Host, c.Port, ConnectMessage(err))
	}
	log.Printf("Connected to %s", version)

	var found string
	err = conn.QueryRowContext(ctx, "SELECT SCHEMA_NAME FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME = ?", c.Database).Scan(&found)
	switch {
	case err == nil:
		return nil
	case !errors.Is(err, sql.ErrNoRows):
		return fmt.Errorf("could not look up the database %s: %s", c.Database, ConnectMessage(err))
	case !c.CreateDatabase:
		return fmt.Errorf("the database %s does not exist on %s:%d: create it, or restore with \"Create the database\" on", c.Database, c.Host, c.Port)
	}
	if _, err := conn.ExecContext(ctx, "CREATE DATABASE "+mysqlIdentifier(c.Database)); err != nil {
		return fmt.Errorf("could not create the database %s: %s", c.Database, ConnectMessage(err))
	}
	log.Printf("Created the database %s", c.Database)
	return nil
}

func mysqlIdentifier(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

// restoreSQLite writes the archive to a new file next to the target, checks it, keeps the file
// it replaces as <path>.before-restore-<UTC time>, and moves the new one into place, which is
// one rename on the same filesystem.
func restoreSQLite(ctx context.Context, job RestoreJob, log *runlog.Log) error {
	var c SQLiteTarget
	if err := json.Unmarshal(job.Target, &c); err != nil {
		return err
	}
	dir := filepath.Dir(c.Path)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("the directory %s does not exist", dir)
	}
	log.Warnf("Stop the application that uses %s before restoring it: it would keep writing to the file being replaced", c.Path)

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(c.Path)+".restore-*")
	if err != nil {
		return fmt.Errorf("could not write next to %s: %w", c.Path, err)
	}
	tmpPath := tmp.Name()
	defer removeSQLiteFiles(tmpPath)
	if err := writeSQLite(ctx, job.Archive, tmp, log); err != nil {
		return err
	}
	if err := checkSQLite(ctx, tmpPath); err != nil {
		return err
	}
	log.Printf("Integrity check: ok")

	if old, err := os.Stat(c.Path); err == nil {
		if err := os.Chmod(tmpPath, old.Mode().Perm()); err != nil {
			return err
		}
		kept := c.Path + ".before-restore-" + time.Now().UTC().Format("20060102T150405Z")
		if err := moveSQLite(c.Path, kept); err != nil {
			return fmt.Errorf("could not keep the current database as %s: %w", kept, err)
		}
		log.Printf("Kept the current database as %s", kept)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.Rename(tmpPath, c.Path); err != nil {
		return fmt.Errorf("could not move the restored database to %s: %w", c.Path, err)
	}
	log.Printf("Restored %s", c.Path)
	return nil
}

func writeSQLite(ctx context.Context, archive Archive, f *os.File, log *runlog.Log) error {
	defer f.Close()
	log.Printf("Downloading %s (%s)", archive.Name, formatSize(archive.Size))
	r, closeBody, err := archive.stream(ctx)
	if err != nil {
		return err
	}
	defer closeBody()
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("the archive is not gzip: %w", err)
	}
	if _, err := io.Copy(f, gz); err != nil {
		return err
	}
	log.Printf("Downloaded and verified (sha256 %s)", archive.SHA256)
	return f.Close()
}

// checkSQLite runs PRAGMA integrity_check on the restored file before it replaces anything.
func checkSQLite(ctx context.Context, path string) error {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer conn.Close()
	var result string
	if err := conn.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); err != nil {
		return fmt.Errorf("the restored database cannot be opened: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("the restored database fails its integrity check: %s", result)
	}
	return nil
}

// sqliteCompanions are the files SQLite keeps next to a database in WAL mode.
var sqliteCompanions = []string{"-wal", "-shm"}

// moveSQLite renames a database and its WAL files, so the moved copy is complete and no stale
// WAL is left for the file that takes its place.
func moveSQLite(from, to string) error {
	if err := os.Rename(from, to); err != nil {
		return err
	}
	for _, suffix := range sqliteCompanions {
		if err := os.Rename(from+suffix, to+suffix); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

func removeSQLiteFiles(path string) {
	os.Remove(path)
	for _, suffix := range sqliteCompanions {
		os.Remove(path + suffix)
	}
}
