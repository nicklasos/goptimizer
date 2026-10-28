package plans

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"app/internal/errs"
	"app/internal/runlog"
	"app/internal/tools"
)

type SourceKind string

const (
	SourcePostgres SourceKind = "postgres"
	SourceMySQL    SourceKind = "mysql"
	SourceSQLite   SourceKind = "sqlite"
	SourceFiles    SourceKind = "files"
)

// source is everything that differs between source kinds. Each kind has its own block in a
// request and a response, named after it, a typed config stored as JSON in plans.source_config,
// and, for a kind with a password, a typed secret sealed in plans.source_secret.
type source struct {
	// block returns the kind's block of a request, and whether the request has one.
	block func(*SourceRequest) (sourceBlock, bool)
	// secretFields are the request fields of the credentials; none for a kind without.
	secretFields []string
	// respond puts the stored config into the kind's block of a response.
	respond func(config []byte, r *PlanResponse) error
	// test checks that the source can be backed up from this server.
	test func(ctx context.Context, config, secret []byte, finder tools.Finder) TestResult
	// dump writes one backup of the source to w, compressed, and logs its steps. What the tool
	// prints goes to the log too.
	dump func(ctx context.Context, config, secret []byte, finder tools.Finder, w io.Writer, log *runlog.Log) error
	// extension ends the name of every archive of the kind.
	extension string
	// restoreKind is how an archive of the kind is put back (restore.go).
	restoreKind restoreKind
}

// sourceBlock is one kind's block of a request.
type sourceBlock interface {
	// split checks the block and returns the config to store and the credentials, or nil
	// credentials when they were left blank. Failures are keyed by field path ("postgres.port").
	split() (config any, secret any, err error)
}

var sources = map[SourceKind]source{
	SourcePostgres: {
		block:        func(r *SourceRequest) (sourceBlock, bool) { return r.Postgres, r.Postgres != nil },
		secretFields: []string{"postgres.password"},
		respond:      func(config []byte, r *PlanResponse) error { return json.Unmarshal(config, &r.Postgres) },
		test:         testPostgres,
		dump:         dumpPostgres,
		extension:    "dump",
		restoreKind: restoreKind{
			block:    func(r *RestoreRequest) (targetBlock, bool) { return r.Postgres, r.Postgres != nil },
			defaults: postgresDefaults,
			respond:  func(target []byte, r *RestoreTarget) error { return json.Unmarshal(target, &r.Postgres) },
			own:      ownDatabase,
			restore:  restorePostgres,
		},
	},
	SourceMySQL: {
		block:        func(r *SourceRequest) (sourceBlock, bool) { return r.MySQL, r.MySQL != nil },
		secretFields: []string{"mysql.password"},
		respond:      func(config []byte, r *PlanResponse) error { return json.Unmarshal(config, &r.MySQL) },
		test:         testMySQL,
		dump:         dumpMySQL,
		extension:    "sql.gz",
		restoreKind: restoreKind{
			block:    func(r *RestoreRequest) (targetBlock, bool) { return r.MySQL, r.MySQL != nil },
			defaults: mysqlDefaults,
			respond:  func(target []byte, r *RestoreTarget) error { return json.Unmarshal(target, &r.MySQL) },
			own:      ownDatabase,
			restore:  restoreMySQL,
		},
	},
	SourceSQLite: {
		block:     func(r *SourceRequest) (sourceBlock, bool) { return r.SQLite, r.SQLite != nil },
		respond:   func(config []byte, r *PlanResponse) error { return json.Unmarshal(config, &r.SQLite) },
		test:      testSQLite,
		dump:      dumpSQLite,
		extension: "db.gz",
		restoreKind: restoreKind{
			block:    func(r *RestoreRequest) (targetBlock, bool) { return r.SQLite, r.SQLite != nil },
			defaults: sqliteDefaults,
			respond:  func(target []byte, r *RestoreTarget) error { return json.Unmarshal(target, &r.SQLite) },
			own:      ownSQLite,
			restore:  restoreSQLite,
		},
	},
	SourceFiles: {
		block:     func(r *SourceRequest) (sourceBlock, bool) { return r.Files, r.Files != nil },
		respond:   func(config []byte, r *PlanResponse) error { return json.Unmarshal(config, &r.Files) },
		test:      testFiles,
		dump:      dumpFiles,
		extension: "tar.gz",
		restoreKind: restoreKind{
			block:    func(r *RestoreRequest) (targetBlock, bool) { return r.Files, r.Files != nil },
			defaults: filesDefaults,
			respond:  func(target []byte, r *RestoreTarget) error { return json.Unmarshal(target, &r.Files) },
			own:      ownFiles,
			restore:  restoreFiles,
		},
	},
}

// PostgresConfig is a PostgreSQL database as stored and returned, without its password.
type PostgresConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Database string `json:"database"`
	SSLMode  string `json:"ssl_mode" enums:"disable,require,verify-full"`
}

// MySQLConfig is a MySQL or MariaDB database as stored and returned, without its password.
type MySQLConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Database string `json:"database"`
}

// SQLiteConfig is a SQLite database file on this server.
type SQLiteConfig struct {
	Path string `json:"path"`
}

// FilesConfig is what a files source archives: paths, minus what matches an exclude pattern.
type FilesConfig struct {
	Paths    []string `json:"paths"`
	Excludes []string `json:"excludes"`
}

// DatabaseSecret is a database password as sealed in plans.source_secret. No response carries
// it: a response only says whether it is set.
type DatabaseSecret struct {
	Password string `json:"password"`
}

const (
	defaultPostgresPort = 5432
	defaultMySQLPort    = 3306
)

func (in *PostgresInput) split() (any, any, error) {
	config := PostgresConfig{
		Host:     strings.TrimSpace(in.Host),
		Port:     in.Port,
		User:     strings.TrimSpace(in.User),
		Database: strings.TrimSpace(in.Database),
		SSLMode:  in.SSLMode,
	}
	if config.Port == 0 {
		config.Port = defaultPostgresPort
	}
	if config.SSLMode == "" {
		config.SSLMode = "disable"
	}
	return config, passwordSecret(in.Password), nil
}

func (in *MySQLInput) split() (any, any, error) {
	config := MySQLConfig{
		Host:     strings.TrimSpace(in.Host),
		Port:     in.Port,
		User:     strings.TrimSpace(in.User),
		Database: strings.TrimSpace(in.Database),
	}
	if config.Port == 0 {
		config.Port = defaultMySQLPort
	}
	return config, passwordSecret(in.Password), nil
}

func (in *SQLiteInput) split() (any, any, error) {
	path := strings.TrimSpace(in.Path)
	if !filepath.IsAbs(path) {
		return nil, nil, errs.FieldError("sqlite.path", "invalid")
	}
	return SQLiteConfig{Path: filepath.Clean(path)}, nil, nil
}

func (in *FilesInput) split() (any, any, error) {
	fields := errs.Fields{}
	config := FilesConfig{Paths: []string{}, Excludes: []string{}}
	for i, raw := range in.Paths {
		path := strings.TrimSpace(raw)
		if !filepath.IsAbs(path) {
			fields.Add(fmt.Sprintf("files.paths.%d", i), "invalid")
			continue
		}
		config.Paths = append(config.Paths, filepath.Clean(path))
	}
	for i, raw := range in.Excludes {
		pattern := strings.TrimSpace(raw)
		if _, err := filepath.Match(pattern, ""); pattern == "" || err != nil {
			fields.Add(fmt.Sprintf("files.excludes.%d", i), "invalid")
			continue
		}
		config.Excludes = append(config.Excludes, pattern)
	}
	if len(fields) > 0 {
		return nil, nil, fields
	}
	return config, nil, nil
}

// passwordSecret is the secret for a password, or nil when it was left blank. A password is
// taken as typed: spaces can be part of it.
func passwordSecret(password string) any {
	if password == "" {
		return nil
	}
	return DatabaseSecret{Password: password}
}

// parseSource checks that the request has the block its kind names, and only that one, and
// returns the config to store and the credentials as JSON (nil when left blank).
func parseSource(req *SourceRequest) (config, secret []byte, err error) {
	fields := errs.Fields{}
	var chosen sourceBlock
	for kind, spec := range sources {
		b, present := spec.block(req)
		switch {
		case kind == req.SourceKind && present:
			chosen = b
		case kind == req.SourceKind:
			fields.Add(string(kind), "required")
		case present:
			fields.Add(string(kind), "invalid")
		}
	}
	if len(fields) > 0 {
		return nil, nil, fields
	}

	plainConfig, plainSecret, err := chosen.split()
	if err != nil {
		return nil, nil, err
	}
	if config, err = json.Marshal(plainConfig); err != nil {
		return nil, nil, err
	}
	if plainSecret != nil {
		if secret, err = json.Marshal(plainSecret); err != nil {
			return nil, nil, err
		}
	}
	return config, secret, nil
}

func secretRequired(kind SourceKind) errs.Fields {
	fields := errs.Fields{}
	for _, field := range sources[kind].secretFields {
		fields.Add(field, "required")
	}
	return fields
}
