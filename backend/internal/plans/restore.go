package plans

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"app/internal/db"
	"app/internal/errs"
	"app/internal/runlog"
	"app/internal/tools"
)

// OriginalLocations is what an admin types to confirm a files restore over the plan's own paths.
const OriginalLocations = "original locations"

// RestoreRequest restores an archive. Exactly the block of the plan's source kind is sent; it
// starts from the plan's own source (GET /archives/{id}/restore-target) and may point elsewhere.
// A target that is the plan's own source needs confirm: the database name, the file path, or
// "original locations".
type RestoreRequest struct {
	Postgres *PostgresTargetInput `json:"postgres" validate:"optional"`
	MySQL    *MySQLTargetInput    `json:"mysql" validate:"optional"`
	SQLite   *SQLiteTargetInput   `json:"sqlite" validate:"optional"`
	Files    *FilesTargetInput    `json:"files" validate:"optional"`
	Confirm  string               `json:"confirm" validate:"optional"`
}

// PostgresTargetInput is the database to restore into. A blank host, port, user or ssl_mode is
// the plan's, and so is a blank password. create_database defaults to true for a database
// other than the plan's.
type PostgresTargetInput struct {
	Host           string `json:"host" validate:"optional" binding:"max=255"`
	Port           int    `json:"port" validate:"optional" binding:"omitempty,min=1,max=65535"`
	User           string `json:"user" validate:"optional" binding:"max=255"`
	Database       string `json:"database" binding:"required,max=255"`
	SSLMode        string `json:"ssl_mode" validate:"optional" binding:"omitempty,oneof=disable require verify-full" enums:"disable,require,verify-full"`
	Password       string `json:"password" validate:"optional"`
	CreateDatabase *bool  `json:"create_database" validate:"optional"`
}

// MySQLTargetInput is the database to restore into, with the same defaults as for Postgres.
type MySQLTargetInput struct {
	Host           string `json:"host" validate:"optional" binding:"max=255"`
	Port           int    `json:"port" validate:"optional" binding:"omitempty,min=1,max=65535"`
	User           string `json:"user" validate:"optional" binding:"max=255"`
	Database       string `json:"database" binding:"required,max=255"`
	Password       string `json:"password" validate:"optional"`
	CreateDatabase *bool  `json:"create_database" validate:"optional"`
}

// SQLiteTargetInput is the database file to restore to, as an absolute path. Its directory must exist.
type SQLiteTargetInput struct {
	Path string `json:"path" binding:"required"`
}

// FilesTargetInput is where a files archive is extracted: under directory, an absolute path,
// or back to the paths the plan backs up when original_locations is true.
type FilesTargetInput struct {
	Directory         string `json:"directory" validate:"optional"`
	OriginalLocations bool   `json:"original_locations" validate:"optional"`
}

// PostgresTarget is a Postgres restore target as stored and returned, without its password.
type PostgresTarget struct {
	PostgresConfig
	CreateDatabase bool `json:"create_database"`
}

// MySQLTarget is a MySQL restore target as stored and returned, without its password.
type MySQLTarget struct {
	MySQLConfig
	CreateDatabase bool `json:"create_database"`
}

// SQLiteTarget is the database file a restore writes.
type SQLiteTarget struct {
	Path string `json:"path"`
}

// FilesTarget is where a files restore extracts to: directory, or the plan's paths when
// original_locations is true.
type FilesTarget struct {
	Directory         string `json:"directory"`
	OriginalLocations bool   `json:"original_locations"`
}

// RestoreTarget is a restore's target. Exactly the block named by source_kind is present.
type RestoreTarget struct {
	SourceKind SourceKind      `json:"source_kind" enums:"postgres,mysql,sqlite,files"`
	Postgres   *PostgresTarget `json:"postgres,omitempty" validate:"optional"`
	MySQL      *MySQLTarget    `json:"mysql,omitempty" validate:"optional"`
	SQLite     *SQLiteTarget   `json:"sqlite,omitempty" validate:"optional"`
	Files      *FilesTarget    `json:"files,omitempty" validate:"optional"`
}

// restoreKind is everything about restoring that differs between source kinds: one field of
// each entry in sources.
type restoreKind struct {
	// block returns the kind's block of a restore request, and whether the request has one.
	block func(*RestoreRequest) (targetBlock, bool)
	// defaults is the target a restore starts from: the plan's own source, or for files a new
	// directory under the work directory.
	defaults func(source []byte, env TargetEnv) (RestoreTarget, error)
	// respond puts a stored target into its block of a response.
	respond func(target []byte, r *RestoreTarget) error
	// own says whether a stored target is the plan's own source, and what an admin types to
	// confirm restoring over it.
	own func(source, target []byte) (phrase string, own bool, err error)
	// restore puts the archive back into the target and logs its steps.
	restore func(ctx context.Context, job RestoreJob, log *runlog.Log) error
}

// targetBlock is one kind's block of a restore request.
type targetBlock interface {
	// resolve fills what the block left blank from the plan's source and checks it. The secret
	// is nil when the password was left blank.
	resolve(source []byte, env TargetEnv) (target any, secret any, err error)
}

// TargetEnv is what target defaults and checks need from the service's configuration.
type TargetEnv struct {
	// WorkDir is the absolute directory restores work in; a files restore defaults to a
	// directory under it.
	WorkDir string
	// DatabasePath is this service's own database, which a restore must never replace under it.
	DatabasePath string
	Now          time.Time
	PlanName     string
}

// ParsedTarget is a restore request checked against the plan: the target to store and its
// password (JSON, not yet sealed; nil to use the plan's).
type ParsedTarget struct {
	Config []byte
	Secret []byte
	Own    bool
}

var errConfirm = errs.Fields{"confirm": {errs.ErrKeyRestoresConfirm}}

// ParseRestoreTarget checks a restore request against the plan: the block of the plan's kind
// and only that one, the target it resolves to, and the confirmation a target that is the
// plan's own source needs.
func ParseRestoreTarget(plan *db.Plan, req *RestoreRequest, env TargetEnv) (*ParsedTarget, error) {
	kind := SourceKind(plan.SourceKind)
	fields := errs.Fields{}
	var chosen targetBlock
	for k, spec := range sources {
		b, present := spec.restoreKind.block(req)
		switch {
		case k == kind && present:
			chosen = b
		case k == kind:
			fields.Add(string(k), "required")
		case present:
			fields.Add(string(k), "invalid")
		}
	}
	if len(fields) > 0 {
		return nil, fields
	}

	spec := sources[kind].restoreKind
	source := []byte(plan.SourceConfig)
	target, secret, err := chosen.resolve(source, env)
	if err != nil {
		return nil, err
	}
	parsed := &ParsedTarget{}
	if parsed.Config, err = json.Marshal(target); err != nil {
		return nil, err
	}
	if secret != nil {
		if parsed.Secret, err = json.Marshal(secret); err != nil {
			return nil, err
		}
	}
	phrase, isOwn, err := spec.own(source, parsed.Config)
	if err != nil {
		return nil, err
	}
	if isOwn && req.Confirm != phrase {
		return nil, errConfirm
	}
	parsed.Own = isOwn
	return parsed, nil
}

// RestoreTargetIsOwn says whether a stored target is the plan's own source, and what an admin
// types to confirm restoring over it. The admin panel runs the same cases (testdata/own_target.json).
func RestoreTargetIsOwn(kind SourceKind, source, target []byte) (phrase string, own bool, err error) {
	return sources[kind].restoreKind.own(source, target)
}

// DefaultRestoreTarget is the target a restore of the plan's archives starts from.
func DefaultRestoreTarget(plan *db.Plan, env TargetEnv) (RestoreTarget, error) {
	env.PlanName = plan.Name
	return sources[SourceKind(plan.SourceKind)].restoreKind.defaults([]byte(plan.SourceConfig), env)
}

// RespondRestoreTarget is a stored target as a response shows it.
func RespondRestoreTarget(kind SourceKind, target []byte) (RestoreTarget, error) {
	response := RestoreTarget{SourceKind: kind}
	err := sources[kind].restoreKind.respond(target, &response)
	return response, err
}

// sameDatabase compares two databases by server and name. localhost, 127.x and ::1 are one
// server: a restore must not dodge the confirmation by naming the plan's server differently.
func sameDatabase(hostA string, portA int, nameA string, hostB string, portB int, nameB string) bool {
	return nameA == nameB && portA == portB && serverName(hostA) == serverName(hostB)
}

func serverName(host string) string {
	host = strings.Trim(strings.ToLower(strings.TrimSpace(host)), "[]")
	if host == "localhost" || host == "::1" || strings.HasPrefix(host, "127.") {
		return "localhost"
	}
	return host
}

func orDefault[T comparable](value, fallback T) T {
	var zero T
	if value == zero {
		return fallback
	}
	return value
}

func (in *PostgresTargetInput) resolve(source []byte, _ TargetEnv) (any, any, error) {
	var plan PostgresConfig
	if err := json.Unmarshal(source, &plan); err != nil {
		return nil, nil, err
	}
	target := PostgresTarget{PostgresConfig: PostgresConfig{
		Host:     orDefault(strings.TrimSpace(in.Host), plan.Host),
		Port:     orDefault(in.Port, plan.Port),
		User:     orDefault(strings.TrimSpace(in.User), plan.User),
		Database: strings.TrimSpace(in.Database),
		SSLMode:  orDefault(in.SSLMode, plan.SSLMode),
	}}
	if target.Database == "" {
		return nil, nil, errs.FieldError("postgres.database", "required")
	}
	own := sameDatabase(plan.Host, plan.Port, plan.Database, target.Host, target.Port, target.Database)
	target.CreateDatabase = createDatabase(in.CreateDatabase, own)
	return target, passwordSecret(in.Password), nil
}

func (in *MySQLTargetInput) resolve(source []byte, _ TargetEnv) (any, any, error) {
	var plan MySQLConfig
	if err := json.Unmarshal(source, &plan); err != nil {
		return nil, nil, err
	}
	target := MySQLTarget{MySQLConfig: MySQLConfig{
		Host:     orDefault(strings.TrimSpace(in.Host), plan.Host),
		Port:     orDefault(in.Port, plan.Port),
		User:     orDefault(strings.TrimSpace(in.User), plan.User),
		Database: strings.TrimSpace(in.Database),
	}}
	if target.Database == "" {
		return nil, nil, errs.FieldError("mysql.database", "required")
	}
	own := sameDatabase(plan.Host, plan.Port, plan.Database, target.Host, target.Port, target.Database)
	target.CreateDatabase = createDatabase(in.CreateDatabase, own)
	return target, passwordSecret(in.Password), nil
}

// createDatabase is the request's choice, or yes for a database other than the plan's: a copy
// usually goes to a database that does not exist yet.
func createDatabase(choice *bool, own bool) bool {
	if choice != nil {
		return *choice
	}
	return !own
}

func (in *SQLiteTargetInput) resolve(_ []byte, env TargetEnv) (any, any, error) {
	path := strings.TrimSpace(in.Path)
	if !filepath.IsAbs(path) {
		return nil, nil, errs.FieldError("sqlite.path", "invalid")
	}
	path = filepath.Clean(path)
	if env.DatabasePath != "" && samePath(path, env.DatabasePath) {
		return nil, nil, errs.Fields{"sqlite.path": {errs.ErrKeyRestoresOwnDatabase}}
	}
	return SQLiteTarget{Path: path}, nil, nil
}

func (in *FilesTargetInput) resolve(_ []byte, _ TargetEnv) (any, any, error) {
	if in.OriginalLocations {
		return FilesTarget{OriginalLocations: true}, nil, nil
	}
	dir := strings.TrimSpace(in.Directory)
	if dir == "" {
		return nil, nil, errs.FieldError("files.directory", "required")
	}
	// The archive holds absolute paths: extracting under / would be the original locations
	// without the confirmation.
	if !filepath.IsAbs(dir) || filepath.Clean(dir) == string(filepath.Separator) {
		return nil, nil, errs.FieldError("files.directory", "invalid")
	}
	return FilesTarget{Directory: filepath.Clean(dir)}, nil, nil
}

// samePath compares two paths as files: the same once absolute, or the same file once symbolic
// links are followed.
func samePath(a, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA == nil && errB == nil && absA == absB {
		return true
	}
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && ra == rb
}

func ownDatabase(source, target []byte) (string, bool, error) {
	type server struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Database string `json:"database"`
	}
	var s, t server
	if err := json.Unmarshal(source, &s); err != nil {
		return "", false, err
	}
	if err := json.Unmarshal(target, &t); err != nil {
		return "", false, err
	}
	return t.Database, sameDatabase(s.Host, s.Port, s.Database, t.Host, t.Port, t.Database), nil
}

func ownSQLite(source, target []byte) (string, bool, error) {
	var s SQLiteConfig
	var t SQLiteTarget
	if err := json.Unmarshal(source, &s); err != nil {
		return "", false, err
	}
	if err := json.Unmarshal(target, &t); err != nil {
		return "", false, err
	}
	return s.Path, filepath.Clean(s.Path) == filepath.Clean(t.Path), nil
}

func ownFiles(_, target []byte) (string, bool, error) {
	var t FilesTarget
	if err := json.Unmarshal(target, &t); err != nil {
		return "", false, err
	}
	return OriginalLocations, t.OriginalLocations, nil
}

func postgresDefaults(source []byte, _ TargetEnv) (RestoreTarget, error) {
	var c PostgresConfig
	err := json.Unmarshal(source, &c)
	return RestoreTarget{SourceKind: SourcePostgres, Postgres: &PostgresTarget{PostgresConfig: c}}, err
}

func mysqlDefaults(source []byte, _ TargetEnv) (RestoreTarget, error) {
	var c MySQLConfig
	err := json.Unmarshal(source, &c)
	return RestoreTarget{SourceKind: SourceMySQL, MySQL: &MySQLTarget{MySQLConfig: c}}, err
}

func sqliteDefaults(source []byte, _ TargetEnv) (RestoreTarget, error) {
	var c SQLiteConfig
	err := json.Unmarshal(source, &c)
	return RestoreTarget{SourceKind: SourceSQLite, SQLite: &SQLiteTarget{Path: c.Path}}, err
}

func filesDefaults(_ []byte, env TargetEnv) (RestoreTarget, error) {
	name := Slug(env.PlanName) + "-" + env.Now.UTC().Format("20060102T150405Z")
	return RestoreTarget{SourceKind: SourceFiles, Files: &FilesTarget{Directory: filepath.Join(env.WorkDir, "restores", name)}}, nil
}

// Archive is the archive a restore reads. Open streams its object from the storage; a restore
// reads it through stream or download, which check it against its size and sha256.
type Archive struct {
	Name   string
	Size   int64
	SHA256 string
	Open   func(ctx context.Context) (io.ReadCloser, error)
}

// RestoreJob is one restore as a kind's restore function gets it.
type RestoreJob struct {
	// Source is the plan's source config, Target the target's, Secret the target's password
	// as JSON (the plan's when the restore was not given one).
	Source, Target, Secret []byte
	Archive                Archive
	// WorkDir is an empty directory of this restore, removed after it.
	WorkDir string
	// Jobs is how many parallel jobs pg_restore runs.
	Jobs  int
	Tools tools.Finder
}

// Restore puts an archive of the plan back into the restore's target, logging its steps and
// what the restore program prints. Cancelling ctx kills the program.
func (s *PlanService) Restore(ctx context.Context, plan *db.Plan, restore *db.Restore, archive Archive, workDir string, jobs int, log *runlog.Log) error {
	secret, err := s.restoreSecret(plan, restore)
	if err != nil {
		return err
	}
	job := RestoreJob{
		Source:  []byte(plan.SourceConfig),
		Target:  []byte(restore.TargetConfig),
		Secret:  secret,
		Archive: archive,
		WorkDir: workDir,
		Jobs:    max(jobs, 1),
		Tools:   s.tools,
	}
	return sources[SourceKind(restore.SourceKind)].restoreKind.restore(ctx, job, log)
}

func (s *PlanService) restoreSecret(plan *db.Plan, restore *db.Restore) ([]byte, error) {
	if restore.TargetSecret != nil {
		return s.secrets.Open(restore.TargetSecret)
	}
	return s.unseal(plan)
}

// checked reads an archive and fails its last read, instead of ending it, when what was read
// is not the archive that was stored.
type checked struct {
	ctx      context.Context
	r        io.Reader
	hash     hash.Hash
	size     int64
	expected Archive
}

func (c *checked) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := c.r.Read(p)
	c.hash.Write(p[:n])
	c.size += int64(n)
	if errors.Is(err, io.EOF) {
		if sum := hex.EncodeToString(c.hash.Sum(nil)); sum != c.expected.SHA256 || c.size != c.expected.Size {
			return n, fmt.Errorf("the archive does not match the one stored: %d bytes with sha256 %s, expected %d bytes with sha256 %s",
				c.size, sum, c.expected.Size, c.expected.SHA256)
		}
	}
	return n, err
}

// stream opens the archive for one pass. The last read fails when it does not match.
func (a Archive) stream(ctx context.Context) (io.Reader, func() error, error) {
	body, err := a.Open(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("could not open %s: %w", a.Name, err)
	}
	return &checked{ctx: ctx, r: body, hash: sha256.New(), expected: a}, body.Close, nil
}

// download saves the archive as a file in dir, checked against its size and sha256, and
// returns its path.
func (a Archive) download(ctx context.Context, dir string, log *runlog.Log) (string, error) {
	log.Printf("Downloading %s (%s)", a.Name, formatSize(a.Size))
	r, closeBody, err := a.stream(ctx)
	if err != nil {
		return "", err
	}
	defer closeBody()
	path := filepath.Join(dir, "archive")
	if err := writeFile(path, r, 0o600); err != nil {
		return "", err
	}
	log.Printf("Downloaded and verified (sha256 %s)", a.SHA256)
	return path, nil
}

// writeFile writes r to a new file at path, removing it again when r fails.
func writeFile(path string, r io.Reader, perm os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, r)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(path)
	}
	return err
}

func formatSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
