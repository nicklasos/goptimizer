package plans

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"app/internal/db"
	"app/internal/runlog"
	"app/internal/tools"
)

// toolExitDelay is how long a dump program has to exit after it is killed (a stopped run) or
// after its output is closed, before Wait gives up on it.
const toolExitDelay = 10 * time.Second

// Dump writes one backup of the plan's source to w, compressed, logging its steps and what the
// dump program prints. Cancelling ctx kills the program.
func (s *PlanService) Dump(ctx context.Context, plan *db.Plan, w io.Writer, log *runlog.Log) error {
	secret, err := s.unseal(plan)
	if err != nil {
		return err
	}
	kind := SourceKind(plan.SourceKind)
	return sources[kind].dump(ctx, []byte(plan.SourceConfig), secret, s.tools, w, log)
}

// ArchiveExtension ends the name of every archive of a source kind: "dump", "sql.gz", "db.gz", "tar.gz".
func ArchiveExtension(kind SourceKind) string {
	return sources[kind].extension
}

// dumpTool finds a dump program and logs its version, or says why it cannot be used.
func dumpTool(ctx context.Context, finder tools.Finder, name string, log *runlog.Log) (tools.Tool, error) {
	tool, err := finder.Find(ctx, name)
	if errors.Is(err, tools.ErrNotFound) {
		return tools.Tool{}, errors.New(name + " was not found on this server's PATH")
	}
	if err != nil {
		return tools.Tool{}, err
	}
	log.Printf("Using %s (%s)", tool.Path, tool.Version)
	return tool, nil
}

// runTool runs a dump or restore program with stdin read from in (nil for none), stdout going
// to w and stderr to the log. env is added to this process's environment, so a password never
// appears in the program's arguments. When in fails, that is the error: the program only saw
// its input end early.
func runTool(ctx context.Context, tool tools.Tool, args, env []string, in io.Reader, w io.Writer, log *runlog.Log) error {
	cmd := exec.CommandContext(ctx, tool.Path, args...)
	cmd.Env = append(os.Environ(), env...)
	var input *recordingReader
	if in != nil {
		input = &recordingReader{r: in}
		cmd.Stdin = input
	}
	cmd.Stdout = w
	stderr := log.Writer(tool.Name + ": ")
	cmd.Stderr = stderr
	cmd.WaitDelay = toolExitDelay
	err := cmd.Run()
	stderr.Close()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if input != nil && input.err != nil {
		return input.err
	}
	if err != nil {
		return fmt.Errorf("%s failed: %w", tool.Name, err)
	}
	return nil
}

// recordingReader keeps the first error other than io.EOF that its reader returned.
type recordingReader struct {
	r   io.Reader
	err error
}

func (r *recordingReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	if err != nil && !errors.Is(err, io.EOF) && r.err == nil {
		r.err = err
	}
	return n, err
}

func dumpPostgres(ctx context.Context, config, secret []byte, finder tools.Finder, w io.Writer, log *runlog.Log) error {
	var c PostgresConfig
	var s DatabaseSecret
	if err := unmarshalSource(config, &c, secret, &s); err != nil {
		return err
	}
	tool, err := dumpTool(ctx, finder, "pg_dump", log)
	if err != nil {
		return err
	}
	server, err := postgresVersion(ctx, c, s.Password)
	if err != nil {
		return fmt.Errorf("could not connect to %s:%d: %s", c.Host, c.Port, ConnectMessage(err))
	}
	log.Printf("Connected to %s, database %s", server.version, c.Database)
	if err := PgDumpCompatible(tool.Version, server.major); err != nil {
		return err
	}
	args, env := PgDumpCommand(c, s.Password)
	return runTool(ctx, tool, args, env, nil, w, log)
}

// PgDumpCommand is the arguments and the added environment of pg_dump for a database. The
// custom format is compressed already and is what pg_restore reads, in parallel too.
func PgDumpCommand(c PostgresConfig, password string) (args, env []string) {
	args = append([]string{"--format=custom", "--no-owner", "--no-acl"}, pgConnectionArgs(c)...)
	return args, pgEnv(c, password)
}

func pgConnectionArgs(c PostgresConfig) []string {
	return []string{"--no-password", "--host=" + c.Host, "--port=" + strconv.Itoa(c.Port), "--username=" + c.User, "--dbname=" + c.Database}
}

func pgEnv(c PostgresConfig, password string) []string {
	return []string{
		"PGPASSWORD=" + password,
		"PGSSLMODE=" + c.SSLMode,
		"PGCONNECT_TIMEOUT=" + strconv.Itoa(int(connectTimeout.Seconds())),
	}
}

func dumpMySQL(ctx context.Context, config, secret []byte, finder tools.Finder, w io.Writer, log *runlog.Log) error {
	var c MySQLConfig
	var s DatabaseSecret
	if err := unmarshalSource(config, &c, secret, &s); err != nil {
		return err
	}
	tool, err := dumpTool(ctx, finder, "mysqldump", log)
	if err != nil {
		return err
	}
	defaults, err := MySQLDefaultsFile(c, s.Password)
	if err != nil {
		return err
	}
	path, err := writePrivateFile("gogobackup-mysql-*.cnf", defaults)
	if err != nil {
		return err
	}
	defer os.Remove(path)

	log.Printf("Dumping database %s from %s:%d", c.Database, c.Host, c.Port)
	gz := gzip.NewWriter(w)
	if err := runTool(ctx, tool, MySQLDumpArgs(path, c), nil, nil, gz, log); err != nil {
		return err
	}
	return gz.Close()
}

// MySQLDumpArgs is the arguments of mysqldump. The password is in the defaults file, which
// must come first. --no-tablespaces spares the user the PROCESS privilege MySQL 8 asks for.
func MySQLDumpArgs(defaultsFile string, c MySQLConfig) []string {
	return []string{
		"--defaults-extra-file=" + defaultsFile,
		"--single-transaction", "--routines", "--triggers", "--events", "--hex-blob", "--no-tablespaces",
		c.Database,
	}
}

// MySQLDefaultsFile is the option file that gives mysqldump its connection, password included,
// so the password is in neither its arguments nor its environment.
func MySQLDefaultsFile(c MySQLConfig, password string) (string, error) {
	values := []struct{ name, value string }{
		{"host", c.Host}, {"port", strconv.Itoa(c.Port)}, {"user", c.User}, {"password", password},
	}
	var b strings.Builder
	b.WriteString("[client]\nprotocol=TCP\n")
	for _, v := range values {
		quoted, err := optionValue(v.value)
		if err != nil {
			return "", fmt.Errorf("the MySQL %s cannot be written to an option file: %w", v.name, err)
		}
		fmt.Fprintf(&b, "%s=%s\n", v.name, quoted)
	}
	return b.String(), nil
}

// optionValue quotes a value for a MySQL option file. The reader strips one pair of quotes and
// treats # outside quotes as a comment, so the value is wrapped in a quote it does not contain.
func optionValue(value string) (string, error) {
	escaped := strings.NewReplacer(`\`, `\\`, "\n", `\n`, "\r", `\r`, "\t", `\t`).Replace(value)
	switch {
	case !strings.Contains(value, `"`):
		return `"` + escaped + `"`, nil
	case !strings.Contains(value, `'`):
		return `'` + escaped + `'`, nil
	}
	return "", errors.New(`it contains both ' and "`)
}

// writePrivateFile writes content to a new temporary file only this user can read.
func writePrivateFile(pattern, content string) (string, error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// dumpSQLite copies the database with sqlite3's .backup, which reads a consistent snapshot
// even while the application writes to it in WAL mode, then compresses the copy.
func dumpSQLite(ctx context.Context, config, _ []byte, finder tools.Finder, w io.Writer, log *runlog.Log) error {
	var c SQLiteConfig
	if err := json.Unmarshal(config, &c); err != nil {
		return err
	}
	// sqlite3 creates a database that does not exist; a mistyped path must fail instead.
	version, err := sqliteFileVersion(c.Path)
	if err != nil {
		return err
	}
	tool, err := dumpTool(ctx, finder, "sqlite3", log)
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "gogobackup-sqlite-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	copyPath := filepath.Join(dir, "backup.db")
	log.Printf("Copying %s (%s)", c.Path, version)
	command := ".backup main '" + strings.ReplaceAll(copyPath, "'", "''") + "'"
	if err := runTool(ctx, tool, []string{"-bail", c.Path, command}, nil, nil, io.Discard, log); err != nil {
		return err
	}

	f, err := os.Open(copyPath)
	if err != nil {
		return fmt.Errorf("sqlite3 did not write the copy: %w", err)
	}
	defer f.Close()
	gz := gzip.NewWriter(w)
	if _, err := io.Copy(gz, contextReader{ctx: ctx, r: f}); err != nil {
		return err
	}
	return gz.Close()
}

// dumpFiles writes a tar.gz of the paths. Names in the archive are the absolute paths without
// the leading slash, so a restore knows where each file goes. A file inside the paths that
// cannot be read is a warning in the log, not a failed run; a path itself that is missing fails.
func dumpFiles(ctx context.Context, config, _ []byte, _ tools.Finder, w io.Writer, log *runlog.Log) error {
	var c FilesConfig
	if err := json.Unmarshal(config, &c); err != nil {
		return err
	}
	gz := gzip.NewWriter(w)
	a := &archiver{ctx: ctx, tw: tar.NewWriter(gz), excludes: c.Excludes, log: log}
	for _, path := range c.Paths {
		if err := readable(path); err != nil {
			return err
		}
		log.Printf("Archiving %s", path)
		if err := filepath.WalkDir(path, a.visit); err != nil {
			return err
		}
	}
	if err := a.tw.Close(); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	log.Printf("Archived %d files, %d directories, %d links; skipped %d excluded, %d special, %d unreadable",
		a.files, a.dirs, a.links, a.excluded, a.special, a.unreadable)
	return nil
}

type archiver struct {
	ctx      context.Context
	tw       *tar.Writer
	excludes []string
	log      *runlog.Log

	files, dirs, links, excluded, special, unreadable int
}

func (a *archiver) visit(path string, entry fs.DirEntry, walkErr error) error {
	if err := a.ctx.Err(); err != nil {
		return err
	}
	if walkErr != nil {
		a.unreadable++
		a.log.Warnf("%s", pathError(path, walkErr))
		if entry != nil && entry.IsDir() {
			return fs.SkipDir
		}
		return nil
	}
	if a.isExcluded(path) {
		a.excluded++
		if entry.IsDir() {
			return fs.SkipDir
		}
		return nil
	}

	info, err := entry.Info()
	if err != nil {
		a.unreadable++
		a.log.Warnf("%s", pathError(path, err))
		return nil
	}
	name := strings.TrimPrefix(filepath.ToSlash(path), "/")
	switch mode := info.Mode(); {
	case mode.IsDir():
		if name == "" {
			return nil
		}
		a.dirs++
		return a.header(info, name+"/", "")
	case mode&fs.ModeSymlink != 0:
		target, err := os.Readlink(path)
		if err != nil {
			a.unreadable++
			a.log.Warnf("%s", pathError(path, err))
			return nil
		}
		a.links++
		return a.header(info, name, target)
	case mode.IsRegular():
		return a.file(path, name, info)
	}
	a.special++
	return nil
}

func (a *archiver) isExcluded(path string) bool {
	for _, pattern := range a.excludes {
		if ok, _ := filepath.Match(pattern, filepath.Base(path)); ok {
			return true
		}
		if ok, _ := filepath.Match(pattern, path); ok {
			return true
		}
	}
	return false
}

func (a *archiver) header(info fs.FileInfo, name, link string) error {
	hdr, err := tar.FileInfoHeader(info, link)
	if err != nil {
		return err
	}
	hdr.Name = name
	return a.tw.WriteHeader(hdr)
}

// file archives one regular file. A file that shrinks while it is read is padded with zeros,
// since its size is already in the header, and noted in the log.
func (a *archiver) file(path, name string, info fs.FileInfo) error {
	f, err := os.Open(path)
	if err != nil {
		a.unreadable++
		a.log.Warnf("%s", pathError(path, err))
		return nil
	}
	defer f.Close()
	if err := a.header(info, name, ""); err != nil {
		return err
	}
	n, err := io.CopyN(a.tw, contextReader{ctx: a.ctx, r: f}, info.Size())
	if err != nil && a.ctx.Err() != nil {
		return a.ctx.Err()
	}
	if n < info.Size() {
		if _, werr := io.CopyN(a.tw, zeros{}, info.Size()-n); werr != nil {
			return werr
		}
		a.log.Warnf("%s changed while it was read: archived %d of %d bytes", path, n, info.Size())
	}
	a.files++
	return nil
}

type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

// contextReader stops a long copy when the run is stopped.
type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (c contextReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}
