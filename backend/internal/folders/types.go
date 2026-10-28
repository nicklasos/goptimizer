package folders

import "encoding/json"

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

// FolderResponse is a folder with its parsed profile
type FolderResponse struct {
	ID                    int64   `json:"id"`
	Name                  string  `json:"name"`
	Kind                  Kind    `json:"kind" enums:"local"`
	Path                  string  `json:"path"`
	Enabled               bool    `json:"enabled"`
	ScanIntervalMinutes   int64   `json:"scan_interval_minutes"`
	FullScanIntervalHours int64   `json:"full_scan_interval_hours"`
	Profile               Profile `json:"profile"`
	CreatedAt             string  `json:"created_at"`
	UpdatedAt             string  `json:"updated_at"`
}

// FolderDataResponse wraps a folder
type FolderDataResponse struct {
	Data FolderResponse `json:"data"`
}

// FoldersResponse is every folder, by name. Not paginated: a server has tens of folders.
type FoldersResponse struct {
	Data []FolderResponse `json:"data"`
}
