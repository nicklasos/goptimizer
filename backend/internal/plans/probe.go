package plans

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"time"

	"app/internal/tools"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// connectTimeout bounds a connection test, so a database that does not answer fails the
// request instead of holding it open.
const connectTimeout = 10 * time.Second

type TestStep string

const (
	StepConnect TestStep = "connect"
	StepTool    TestStep = "tool"
	StepPath    TestStep = "path"
)

// TestResult is the outcome of a source test: nil Failure means it can be backed up. Tool is
// nil for a kind that needs no program; ServerVersion is empty when the test did not get that far.
type TestResult struct {
	Failure       *TestFailure
	ServerVersion string
	Tool          *ToolStatus
}

type TestFailure struct {
	Step    TestStep
	Message string
}

// ToolStatus is a dump program as found on this server. Problem says why it cannot be used
// when it was not found or did not report a version.
type ToolStatus struct {
	Name    string
	Version string
	Found   bool
	Problem string
}

func findTool(ctx context.Context, finder tools.Finder, name string) ToolStatus {
	tool, err := finder.Find(ctx, name)
	switch {
	case errors.Is(err, tools.ErrNotFound):
		return ToolStatus{Name: name, Problem: name + " was not found on this server's PATH"}
	case err != nil:
		return ToolStatus{Name: name, Found: true, Problem: err.Error()}
	}
	return ToolStatus{Name: name, Version: tool.Version, Found: true}
}

func failed(result TestResult, step TestStep, message string) TestResult {
	result.Failure = &TestFailure{Step: step, Message: message}
	return result
}

// postgresServer is what a test learns from the server: its version as shown, and its major
// version, which pg_dump must reach.
type postgresServer struct {
	version string
	major   int
}

func testPostgres(ctx context.Context, config, secret []byte, finder tools.Finder) TestResult {
	var c PostgresConfig
	var s DatabaseSecret
	if err := unmarshalSource(config, &c, secret, &s); err != nil {
		return TestResult{Failure: &TestFailure{Step: StepConnect, Message: err.Error()}}
	}
	tool := findTool(ctx, finder, "pg_dump")
	server, err := postgresVersion(ctx, c, s.Password)
	return postgresResult(server, err, tool)
}

func postgresVersion(ctx context.Context, c PostgresConfig, password string) (postgresServer, error) {
	ctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	conn, err := connectPostgres(ctx, c, password)
	if err != nil {
		return postgresServer{}, err
	}
	defer conn.Close(context.Background())
	return queryPostgresVersion(ctx, conn)
}

func connectPostgres(ctx context.Context, c PostgresConfig, password string) (*pgx.Conn, error) {
	dsn := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.User, password),
		Host:     net.JoinHostPort(c.Host, strconv.Itoa(c.Port)),
		Path:     "/" + c.Database,
		RawQuery: url.Values{"sslmode": {c.SSLMode}}.Encode(),
	}
	cfg, err := pgx.ParseConfig(dsn.String())
	if err != nil {
		return nil, err
	}
	cfg.ConnectTimeout = connectTimeout
	return pgx.ConnectConfig(ctx, cfg)
}

func queryPostgresVersion(ctx context.Context, conn *pgx.Conn) (postgresServer, error) {
	var server postgresServer
	var number int
	err := conn.QueryRow(ctx, "SELECT current_setting('server_version'), current_setting('server_version_num')::int").Scan(&server.version, &number)
	if err != nil {
		return postgresServer{}, err
	}
	server.version = "PostgreSQL " + server.version
	server.major = number / 10000
	return server, nil
}

// postgresResult combines what the test found. pg_dump refuses to dump a server of a newer
// major version than itself, so an older one fails the test at the tool step.
func postgresResult(server postgresServer, connectErr error, tool ToolStatus) TestResult {
	result := TestResult{ServerVersion: server.version, Tool: &tool}
	if connectErr != nil {
		return failed(result, StepConnect, ConnectMessage(connectErr))
	}
	if tool.Problem != "" {
		return failed(result, StepTool, tool.Problem)
	}
	if err := PgDumpCompatible(tool.Version, server.major); err != nil {
		return failed(result, StepTool, err.Error())
	}
	return result
}

var firstNumber = regexp.MustCompile(`\d+`)

// PgDumpCompatible says whether pg_dump, by its `--version` output ("pg_dump (PostgreSQL) 16.2"),
// can dump a server of the given major version. pg_dump refuses a server newer than itself.
func PgDumpCompatible(version string, serverMajor int) error {
	toolMajor, err := strconv.Atoi(firstNumber.FindString(version))
	if err != nil {
		return fmt.Errorf("could not read the version of pg_dump from %q", version)
	}
	if toolMajor < serverMajor {
		return fmt.Errorf("pg_dump %d is older than the server (PostgreSQL %d) and refuses to dump it: install pg_dump %d or newer",
			toolMajor, serverMajor, serverMajor)
	}
	return nil
}

func testMySQL(ctx context.Context, config, secret []byte, finder tools.Finder) TestResult {
	var c MySQLConfig
	var s DatabaseSecret
	if err := unmarshalSource(config, &c, secret, &s); err != nil {
		return TestResult{Failure: &TestFailure{Step: StepConnect, Message: err.Error()}}
	}
	tool := findTool(ctx, finder, "mysqldump")
	version, err := mysqlVersion(ctx, c, s.Password)
	result := TestResult{ServerVersion: version, Tool: &tool}
	if err != nil {
		return failed(result, StepConnect, ConnectMessage(err))
	}
	if tool.Problem != "" {
		return failed(result, StepTool, tool.Problem)
	}
	return result
}

func mysqlVersion(ctx context.Context, c MySQLConfig, password string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	conn, err := openMySQL(c, password, c.Database)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	return queryMySQLVersion(ctx, conn)
}

// openMySQL is a pool of one connection to the server of c, using database (none when empty).
func openMySQL(c MySQLConfig, password, database string) (*sql.DB, error) {
	cfg := mysql.NewConfig()
	cfg.User = c.User
	cfg.Passwd = password
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
	cfg.DBName = database
	cfg.Timeout = connectTimeout
	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		return nil, err
	}
	conn := sql.OpenDB(connector)
	conn.SetMaxOpenConns(1)
	return conn, nil
}

func queryMySQLVersion(ctx context.Context, conn *sql.DB) (string, error) {
	var version string
	if err := conn.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err != nil {
		return "", err
	}
	return "MySQL " + version, nil
}

// ConnectMessage is what a failed database connection shows: the server's own message when it
// answered (a wrong password, a missing database), the network error otherwise.
func ConnectMessage(err error) string {
	var pgErr *pgconn.PgError
	var mysqlErr *mysql.MySQLError
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Sprintf("no answer within %d seconds", int(connectTimeout.Seconds()))
	case errors.As(err, &pgErr):
		return pgErr.Message
	case errors.As(err, &mysqlErr):
		return mysqlErr.Message
	}
	return err.Error()
}

// sqliteHeader starts every SQLite database file. The version of the library that last wrote
// the file is a big-endian number at offset 96.
var sqliteHeader = []byte("SQLite format 3\x00")

func testSQLite(ctx context.Context, config, _ []byte, finder tools.Finder) TestResult {
	var c SQLiteConfig
	if err := json.Unmarshal(config, &c); err != nil {
		return TestResult{Failure: &TestFailure{Step: StepPath, Message: err.Error()}}
	}
	tool := findTool(ctx, finder, "sqlite3")
	result := TestResult{Tool: &tool}

	version, err := sqliteFileVersion(c.Path)
	if err != nil {
		return failed(result, StepPath, err.Error())
	}
	result.ServerVersion = version
	if tool.Problem != "" {
		return failed(result, StepTool, tool.Problem)
	}
	return result
}

func sqliteFileVersion(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", pathError(path, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory, not a database file", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return "", pathError(path, err)
	}
	defer f.Close()

	header := make([]byte, 100)
	n, err := io.ReadFull(f, header)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", pathError(path, err)
	}
	if n < len(sqliteHeader) || string(header[:len(sqliteHeader)]) != string(sqliteHeader) {
		return "", fmt.Errorf("%s is not a SQLite database", path)
	}
	if n < len(header) {
		return "SQLite", nil
	}
	number := binary.BigEndian.Uint32(header[96:])
	return fmt.Sprintf("SQLite %d.%d.%d", number/1000000, number/1000%1000, number%1000), nil
}

func testFiles(_ context.Context, config, _ []byte, _ tools.Finder) TestResult {
	var c FilesConfig
	if err := json.Unmarshal(config, &c); err != nil {
		return TestResult{Failure: &TestFailure{Step: StepPath, Message: err.Error()}}
	}
	for _, path := range c.Paths {
		if err := readable(path); err != nil {
			return TestResult{Failure: &TestFailure{Step: StepPath, Message: err.Error()}}
		}
	}
	return TestResult{}
}

// readable opens a file, or lists the first entry of a directory, as the archiver will.
func readable(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return pathError(path, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return pathError(path, err)
	}
	if info.IsDir() {
		if _, err := f.Readdirnames(1); err != nil && !errors.Is(err, io.EOF) {
			return pathError(path, err)
		}
	}
	return nil
}

func pathError(path string, err error) error {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%s does not exist", path)
	case errors.Is(err, fs.ErrPermission):
		return fmt.Errorf("%s cannot be read: permission denied", path)
	}
	return fmt.Errorf("%s cannot be read: %w", path, err)
}

func unmarshalSource(config []byte, c any, secret []byte, s any) error {
	if err := json.Unmarshal(config, c); err != nil {
		return err
	}
	if secret == nil {
		return nil
	}
	return json.Unmarshal(secret, s)
}
