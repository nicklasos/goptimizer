package settings

import "app/internal/tools"

// SettingsRequest replaces the global settings
type SettingsRequest struct {
	// Workers is how many images are optimized at once; 0 means one less than the CPU count.
	Workers     *int64 `json:"workers" binding:"required,min=0,max=64"`
	ScanWorkers *int64 `json:"scan_workers" binding:"required,min=1,max=16"`
	// Nice runs the tools under nice and ionice, so the projects on the server stay fast.
	Nice *bool `json:"nice" binding:"required"`
}

// SettingsResponse is the global settings. Workers is as stored (0 = auto); EffectiveWorkers is the number in use.
type SettingsResponse struct {
	Workers          int64  `json:"workers"`
	EffectiveWorkers int64  `json:"effective_workers"`
	ScanWorkers      int64  `json:"scan_workers"`
	Nice             bool   `json:"nice"`
	UpdatedAt        string `json:"updated_at"`
}

// SettingsDataResponse wraps the settings
type SettingsDataResponse struct {
	Data SettingsResponse `json:"data"`
}

// ToolsResponse wraps the image tools report
type ToolsResponse struct {
	Data tools.Report `json:"data"`
}
