package runs

import (
	"app/internal"
	"app/internal/plans"
	"app/internal/storages"
)

// RunResponse is a run without its log. scheduled_for is set for a scheduled run; started_at,
// finished_at, duration_ms and size_bytes once it got that far; error when it failed.
type RunResponse struct {
	ID           int64   `json:"id"`
	PlanID       int64   `json:"plan_id"`
	Trigger      Trigger `json:"trigger" enums:"schedule,manual"`
	Status       Status  `json:"status" enums:"queued,running,succeeded,failed"`
	ScheduledFor *string `json:"scheduled_for" extensions:"x-nullable"`
	StartedAt    *string `json:"started_at" extensions:"x-nullable"`
	FinishedAt   *string `json:"finished_at" extensions:"x-nullable"`
	DurationMs   *int64  `json:"duration_ms" extensions:"x-nullable"`
	SizeBytes    *int64  `json:"size_bytes" extensions:"x-nullable"`
	Error        *string `json:"error" extensions:"x-nullable"`
	CreatedAt    string  `json:"created_at"`
}

// RunDetailResponse is a run with its log, which grows while it runs, and the archives it stored
type RunDetailResponse struct {
	RunResponse
	Log      string            `json:"log"`
	Archives []ArchiveResponse `json:"archives"`
}

// ArchiveResponse is one archive on one storage. deleted_at is set once retention removed it.
type ArchiveResponse struct {
	ID          int64         `json:"id"`
	RunID       int64         `json:"run_id"`
	PlanID      int64         `json:"plan_id"`
	StorageID   int64         `json:"storage_id"`
	StorageName string        `json:"storage_name"`
	StorageKind storages.Kind `json:"storage_kind" enums:"local,s3,gcs"`
	ObjectKey   string        `json:"object_key"`
	SizeBytes   int64         `json:"size_bytes"`
	SHA256      string        `json:"sha256"`
	CreatedAt   string        `json:"created_at"`
	DeletedAt   *string       `json:"deleted_at" extensions:"x-nullable"`
}

// RunDataResponse wraps a run
type RunDataResponse struct {
	Data RunResponse `json:"data"`
}

// RunDetailDataResponse wraps a run with its log
type RunDetailDataResponse struct {
	Data RunDetailResponse `json:"data"`
}

// PaginatedRunsResponse wraps a page of runs
type PaginatedRunsResponse struct {
	Data       []RunResponse           `json:"data"`
	Pagination internal.PaginationMeta `json:"pagination"`
}

// PaginatedArchivesResponse wraps a page of archives
type PaginatedArchivesResponse struct {
	Data       []ArchiveResponse       `json:"data"`
	Pagination internal.PaginationMeta `json:"pagination"`
}

// DownloadLinkResponse is a token for GET /api/v1/downloads/{token}, which downloads the archive
// without a session until expires_at, a minute from now
type DownloadLinkResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

// DownloadLinkDataResponse wraps a download link
type DownloadLinkDataResponse struct {
	Data DownloadLinkResponse `json:"data"`
}

// RestoreArchiveResponse is the archive a restore reads
type RestoreArchiveResponse struct {
	ObjectKey   string `json:"object_key"`
	StorageName string `json:"storage_name"`
	SizeBytes   int64  `json:"size_bytes"`
	CreatedAt   string `json:"created_at"`
}

// RestoreUserResponse is who started a restore
type RestoreUserResponse struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// RestoreResponse is a restore without its log. archive is null once its archive row is gone
// (its storage was deleted), created_by once that user is. secret_set says whether a password
// was given for the target; without one the plan's is used. started_at, finished_at and
// duration_ms are set once it got that far, error when it failed.
type RestoreResponse struct {
	ID         int64                   `json:"id"`
	PlanID     int64                   `json:"plan_id"`
	ArchiveID  *int64                  `json:"archive_id" extensions:"x-nullable"`
	Archive    *RestoreArchiveResponse `json:"archive" extensions:"x-nullable"`
	Target     plans.RestoreTarget     `json:"target"`
	SecretSet  bool                    `json:"secret_set"`
	Status     Status                  `json:"status" enums:"queued,running,succeeded,failed"`
	StartedAt  *string                 `json:"started_at" extensions:"x-nullable"`
	FinishedAt *string                 `json:"finished_at" extensions:"x-nullable"`
	DurationMs *int64                  `json:"duration_ms" extensions:"x-nullable"`
	Error      *string                 `json:"error" extensions:"x-nullable"`
	CreatedBy  *RestoreUserResponse    `json:"created_by" extensions:"x-nullable"`
	CreatedAt  string                  `json:"created_at"`
}

// RestoreDetailResponse is a restore with its log, which grows while it runs
type RestoreDetailResponse struct {
	RestoreResponse
	Log string `json:"log"`
}

// RestoreDetailDataResponse wraps a restore with its log
type RestoreDetailDataResponse struct {
	Data RestoreDetailResponse `json:"data"`
}

// PaginatedRestoresResponse wraps a page of restores
type PaginatedRestoresResponse struct {
	Data       []RestoreResponse       `json:"data"`
	Pagination internal.PaginationMeta `json:"pagination"`
}

// RestoreTargetDataResponse wraps the target a restore starts from
type RestoreTargetDataResponse struct {
	Data plans.RestoreTarget `json:"data"`
}
