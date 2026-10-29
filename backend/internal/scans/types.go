package scans

import (
	"database/sql"

	"app/internal"
	"app/internal/db"
)

// StartScanRequest queues a scan of a folder by hand
type StartScanRequest struct {
	Kind Kind `json:"kind" binding:"required,oneof=quick full" enums:"quick,full"`
}

// ScanResponse is one scan of a folder. The counts grow while it runs. files_new includes files
// that came back after being gone. It is named for swag because folders embeds it too, and swag
// would otherwise describe it twice. The enum fields are plain strings for the same reason.
type ScanResponse struct {
	ID           int64   `json:"id"`
	FolderID     int64   `json:"folder_id"`
	Kind         string  `json:"kind" enums:"quick,full"`
	Trigger      string  `json:"trigger" enums:"schedule,manual"`
	Status       string  `json:"status" enums:"queued,running,succeeded,failed"`
	QueuedAt     string  `json:"queued_at"`
	StartedAt    *string `json:"started_at" extensions:"x-nullable"`
	FinishedAt   *string `json:"finished_at" extensions:"x-nullable"`
	DirsSeen     int64   `json:"dirs_seen"`
	DirsListed   int64   `json:"dirs_listed"`
	FilesSeen    int64   `json:"files_seen"`
	FilesNew     int64   `json:"files_new"`
	FilesChanged int64   `json:"files_changed"`
	FilesGone    int64   `json:"files_gone"`
	Error        *string `json:"error" extensions:"x-nullable"`
} //	@name	ScanResponse

// ScanDataResponse wraps a scan
type ScanDataResponse struct {
	Data ScanResponse `json:"data"`
}

type PaginatedScansResponse struct {
	Data       []ScanResponse          `json:"data"`
	Pagination internal.PaginationMeta `json:"pagination"`
}

func ResponseFromDB(scan db.Scan) ScanResponse {
	response := ScanResponse{
		ID:           scan.ID,
		FolderID:     scan.FolderID,
		Kind:         scan.Kind,
		Trigger:      scan.Trigger,
		Status:       scan.Status,
		QueuedAt:     internal.FormatTime(sql.NullTime{Time: scan.QueuedAt, Valid: true}),
		StartedAt:    internal.FormatOptionalTime(scan.StartedAt),
		FinishedAt:   internal.FormatOptionalTime(scan.FinishedAt),
		DirsSeen:     scan.DirsSeen,
		DirsListed:   scan.DirsListed,
		FilesSeen:    scan.FilesSeen,
		FilesNew:     scan.FilesNew,
		FilesChanged: scan.FilesChanged,
		FilesGone:    scan.FilesGone,
	}
	if scan.Error.Valid {
		response.Error = &scan.Error.String
	}
	return response
}
