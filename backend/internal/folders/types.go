package folders

import (
	"encoding/json"

	"app/internal/scans"
)

const (
	DefaultScanIntervalMinutes   = 60
	DefaultFullScanIntervalHours = 24
)

// FolderRequest creates or replaces a folder. Omitted fields, and omitted fields of the
// profile, take their defaults.
type FolderRequest struct {
	Name                  string  `json:"name" binding:"required,max=100"`
	Kind                  Kind    `json:"kind" validate:"optional" enums:"local"`
	Path                  string  `json:"path" binding:"required"`
	ScanIntervalMinutes   int64   `json:"scan_interval_minutes" binding:"min=5,max=10080" validate:"optional"`
	FullScanIntervalHours int64   `json:"full_scan_interval_hours" binding:"min=1,max=720" validate:"optional"`
	Profile               Profile `json:"profile" validate:"optional"`
}

func (r *FolderRequest) UnmarshalJSON(data []byte) error {
	type plain FolderRequest
	decoded := plain{
		Kind:                  KindLocal,
		ScanIntervalMinutes:   DefaultScanIntervalMinutes,
		FullScanIntervalHours: DefaultFullScanIntervalHours,
		Profile:               DefaultProfile(),
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = FolderRequest(decoded)
	return nil
}

// SetEnabledRequest pauses or resumes a folder
type SetEnabledRequest struct {
	Enabled *bool `json:"enabled" binding:"required"`
}

// FolderResponse is a folder with its parsed profile and the state of its image index:
// last_scan is its latest scan of any status, image_counts the number of images per status,
// bytes_before what its images weighed before they were optimized and bytes_now what they weigh
// now (their difference is what optimizing saved), and missing_tools the programs
// its profile needs that are not installed, whose images wait as pending.
type FolderResponse struct {
	ID                    int64               `json:"id"`
	Name                  string              `json:"name"`
	Kind                  Kind                `json:"kind" enums:"local"`
	Path                  string              `json:"path"`
	Enabled               bool                `json:"enabled"`
	ScanIntervalMinutes   int64               `json:"scan_interval_minutes"`
	FullScanIntervalHours int64               `json:"full_scan_interval_hours"`
	Profile               Profile             `json:"profile"`
	LastScan              *scans.ScanResponse `json:"last_scan" extensions:"x-nullable"`
	ImageCounts           map[string]int64    `json:"image_counts"`
	BytesBefore           int64               `json:"bytes_before"`
	BytesNow              int64               `json:"bytes_now"`
	MissingTools          []string            `json:"missing_tools"`
	CreatedAt             string              `json:"created_at"`
	UpdatedAt             string              `json:"updated_at"`
}

// FolderDataResponse wraps a folder
type FolderDataResponse struct {
	Data FolderResponse `json:"data"`
}

// FoldersResponse is every folder, by name. Not paginated: a server has tens of folders.
type FoldersResponse struct {
	Data []FolderResponse `json:"data"`
}
