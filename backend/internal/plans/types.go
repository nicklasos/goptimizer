package plans

import (
	"app/internal"
	"app/internal/storages"
)

// SourceRequest is what a plan backs up. Exactly the block named by source_kind is sent.
// A password left blank on an update keeps the stored one.
type SourceRequest struct {
	SourceKind SourceKind     `json:"source_kind" binding:"required,oneof=postgres mysql sqlite files" enums:"postgres,mysql,sqlite,files"`
	Postgres   *PostgresInput `json:"postgres" validate:"optional"`
	MySQL      *MySQLInput    `json:"mysql" validate:"optional"`
	SQLite     *SQLiteInput   `json:"sqlite" validate:"optional"`
	Files      *FilesInput    `json:"files" validate:"optional"`
}

// PlanRequest creates or updates a plan. No schedules means the plan only runs when started by
// hand. A retention value of 0 turns that rule off; keep_weekly and keep_monthly need max_age_days.
type PlanRequest struct {
	Name string `json:"name" binding:"required,max=100"`
	SourceRequest
	Schedules   []ScheduleInput `json:"schedules"`
	MaxAgeDays  int             `json:"max_age_days" binding:"min=0,max=3650"`
	KeepWeekly  int             `json:"keep_weekly" binding:"min=0,max=520"`
	KeepMonthly int             `json:"keep_monthly" binding:"min=0,max=240"`
	StorageIDs  []int64         `json:"storage_ids" binding:"required,min=1"`
	Enabled     bool            `json:"enabled"`
}

// PostgresInput is a PostgreSQL database, reached over TCP. Port defaults to 5432 and ssl_mode to disable.
type PostgresInput struct {
	Host     string `json:"host" binding:"required,max=255"`
	Port     int    `json:"port" validate:"optional" binding:"omitempty,min=1,max=65535"`
	User     string `json:"user" binding:"required,max=255"`
	Database string `json:"database" binding:"required,max=255"`
	SSLMode  string `json:"ssl_mode" validate:"optional" binding:"omitempty,oneof=disable require verify-full" enums:"disable,require,verify-full"`
	Password string `json:"password" validate:"optional"`
}

// MySQLInput is a MySQL or MariaDB database, reached over TCP. Port defaults to 3306.
type MySQLInput struct {
	Host     string `json:"host" binding:"required,max=255"`
	Port     int    `json:"port" validate:"optional" binding:"omitempty,min=1,max=65535"`
	User     string `json:"user" binding:"required,max=255"`
	Database string `json:"database" binding:"required,max=255"`
	Password string `json:"password" validate:"optional"`
}

// SQLiteInput is a SQLite database file on this server, as an absolute path
type SQLiteInput struct {
	Path string `json:"path" binding:"required"`
}

// FilesInput is directories and files on this server, as absolute paths, and glob patterns to leave out
type FilesInput struct {
	Paths    []string `json:"paths" binding:"required,min=1"`
	Excludes []string `json:"excludes" validate:"optional"`
}

// ScheduleInput is one line of the timetable: "HH:MM" in UTC on the given days, or every day when days is empty
type ScheduleInput struct {
	Time string   `json:"time"`
	Days []string `json:"days" enums:"mon,tue,wed,thu,fri,sat,sun"`
}

// PlanStorageResponse is a storage a plan uploads to
type PlanStorageResponse struct {
	ID   int64         `json:"id"`
	Name string        `json:"name"`
	Kind storages.Kind `json:"kind" enums:"local,s3,gcs"`
}

// LastRunResponse is the latest run of a plan, enough for a status at a glance
type LastRunResponse struct {
	ID         int64   `json:"id"`
	Status     string  `json:"status" enums:"queued,running,succeeded,failed"`
	Trigger    string  `json:"trigger" enums:"schedule,manual"`
	StartedAt  *string `json:"started_at" extensions:"x-nullable"`
	FinishedAt *string `json:"finished_at" extensions:"x-nullable"`
	SizeBytes  *int64  `json:"size_bytes" extensions:"x-nullable"`
}

// PlanResponse is a plan without its source password: secret_set says whether one is stored.
// Exactly the source block named by source_kind is present. next_run_at is null for a plan
// that is disabled or has no schedules. last_run is null before the first run, and
// last_success_at and last_size_bytes (the size of that backup) before the first that
// succeeded. folder is where its archives go in each storage, such as "myproject-dev/".
type PlanResponse struct {
	ID          int64                 `json:"id"`
	Name        string                `json:"name"`
	SourceKind  SourceKind            `json:"source_kind" enums:"postgres,mysql,sqlite,files"`
	Postgres    *PostgresConfig       `json:"postgres,omitempty" validate:"optional"`
	MySQL       *MySQLConfig          `json:"mysql,omitempty" validate:"optional"`
	SQLite      *SQLiteConfig         `json:"sqlite,omitempty" validate:"optional"`
	Files       *FilesConfig          `json:"files,omitempty" validate:"optional"`
	SecretSet   bool                  `json:"secret_set"`
	Schedules   []Schedule            `json:"schedules"`
	MaxAgeDays  int64                 `json:"max_age_days"`
	KeepWeekly  int64                 `json:"keep_weekly"`
	KeepMonthly int64                 `json:"keep_monthly"`
	Storages    []PlanStorageResponse `json:"storages"`
	Enabled     bool                  `json:"enabled"`
	NextRunAt   *string               `json:"next_run_at" extensions:"x-nullable"`
	LastRun     *LastRunResponse      `json:"last_run" extensions:"x-nullable"`
	LastSuccess *string               `json:"last_success_at" extensions:"x-nullable"`
	LastSize    *int64                `json:"last_size_bytes" extensions:"x-nullable"`
	Health      Health                `json:"health" enums:"restoring,running,failed,paused,overdue,never,ok"`
	Folder      string                `json:"folder"`
	CreatedAt   string                `json:"created_at"`
	UpdatedAt   string                `json:"updated_at"`
}

// SetEnabledRequest pauses (false) or resumes (true) a plan
type SetEnabledRequest struct {
	Enabled *bool `json:"enabled" binding:"required"`
}

// DayResponse is how many runs of a plan that started on one UTC day succeeded and failed
type DayResponse struct {
	Date      string `json:"date" example:"2026-10-17"`
	Succeeded int64  `json:"succeeded"`
	Failed    int64  `json:"failed"`
}

// DashboardPlanResponse is a plan with its runs per day: the last 30 UTC days, oldest first,
// today last, every day present
type DashboardPlanResponse struct {
	PlanResponse
	Days []DayResponse `json:"days"`
}

// HealthSummaryResponse is how many plans have each health
type HealthSummaryResponse struct {
	Restoring int `json:"restoring"`
	Running   int `json:"running"`
	Failed    int `json:"failed"`
	Paused    int `json:"paused"`
	Overdue   int `json:"overdue"`
	Never     int `json:"never"`
	OK        int `json:"ok"`
}

// DashboardResponse is every plan, by name, and how many have each health
type DashboardResponse struct {
	Plans   []DashboardPlanResponse `json:"plans"`
	Summary HealthSummaryResponse   `json:"summary"`
}

// DashboardDataResponse wraps the dashboard
type DashboardDataResponse struct {
	Data DashboardResponse `json:"data"`
}

// PlanDataResponse wraps a plan
type PlanDataResponse struct {
	Data *PlanResponse `json:"data"`
}

// PaginatedPlansResponse wraps a page of plans
type PaginatedPlansResponse struct {
	Data       []PlanResponse          `json:"data"`
	Pagination internal.PaginationMeta `json:"pagination"`
}

// ToolResponse is the dump program a source needs, as found on this server
type ToolResponse struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty" validate:"optional"`
	Found   bool   `json:"found"`
}

// SourceTestResponse is the outcome of a source test. A failed test is a result, not an HTTP
// error: step names what failed and error says why. server_version and tool are filled in as
// far as the test got.
type SourceTestResponse struct {
	OK            bool          `json:"ok"`
	Step          string        `json:"step,omitempty" validate:"optional" enums:"connect,tool,path"`
	Error         string        `json:"error,omitempty" validate:"optional"`
	ServerVersion string        `json:"server_version,omitempty" validate:"optional"`
	Tool          *ToolResponse `json:"tool,omitempty" validate:"optional"`
}

// SourceTestDataResponse wraps a source test result
type SourceTestDataResponse struct {
	Data SourceTestResponse `json:"data"`
}
