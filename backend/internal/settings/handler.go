package settings

import (
	"net/http"

	"app/internal"
	"app/internal/db"
	"app/internal/errs"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *SettingsService
}

func NewHandler(service *SettingsService) *Handler {
	return &Handler{service: service}
}

func settingsResponseFromDB(setting db.Setting) SettingsResponse {
	return SettingsResponse{
		Workers:          setting.Workers,
		EffectiveWorkers: EffectiveWorkers(setting.Workers),
		ScanWorkers:      setting.ScanWorkers,
		Nice:             setting.Nice,
		UpdatedAt:        internal.FormatTime(setting.UpdatedAt),
	}
}

// GetSettings returns the global settings
//
//	@Summary		Get settings
//	@Tags			settings
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	SettingsDataResponse
//	@Failure		401	{object}	errs.ErrorResponse
//	@Failure		500	{object}	errs.ErrorResponse
//	@Router			/api/v1/settings [get]
func (h *Handler) GetSettings(c *gin.Context) {
	setting, err := h.service.Get(c.Request.Context())
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, SettingsDataResponse{Data: settingsResponseFromDB(*setting)})
}

// UpdateSettings replaces the global settings
//
//	@Summary		Update settings
//	@Description	workers 0..64 (0 = one less than the CPU count), scan_workers 1..16, nice.
//	@Tags			settings
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		SettingsRequest	true	"Settings"
//	@Success		200		{object}	SettingsDataResponse
//	@Failure		400		{object}	errs.ErrorResponse
//	@Failure		401		{object}	errs.ErrorResponse
//	@Failure		500		{object}	errs.ErrorResponse
//	@Router			/api/v1/settings [put]
func (h *Handler) UpdateSettings(c *gin.Context) {
	var req SettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errs.RespondWithValidationError(c, err)
		return
	}
	setting, err := h.service.Update(c.Request.Context(), req)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, SettingsDataResponse{Data: settingsResponseFromDB(*setting)})
}

// GetTools reports the image tools
//
//	@Summary		Image tools
//	@Description	Every image tool, installed or not, with its path, version and install hints, and which kinds of work the installed ones cover. Cached for 30 seconds; refresh=1 looks again.
//	@Tags			settings
//	@Produce		json
//	@Security		BearerAuth
//	@Param			refresh	query		int	false	"1 to skip the cache"
//	@Success		200		{object}	ToolsResponse
//	@Failure		401		{object}	errs.ErrorResponse
//	@Failure		500		{object}	errs.ErrorResponse
//	@Router			/api/v1/tools [get]
func (h *Handler) GetTools(c *gin.Context) {
	report, err := h.service.Tools(c.Request.Context(), c.Query("refresh") == "1")
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, ToolsResponse{Data: report})
}
