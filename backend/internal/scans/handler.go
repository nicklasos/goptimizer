package scans

import (
	"net/http"

	"app/internal/errs"
	"app/internal/middleware"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *ScanService
}

func NewHandler(service *ScanService) *Handler {
	return &Handler{service: service}
}

// StartScan queues a scan of a folder
//
//	@Summary		Scan folder now
//	@Description	Queue a quick or full scan. A quick scan lists the directories whose modification time changed; a full scan lists every directory and also finds files rewritten in place. 409 scans.in_progress while one is queued or running, 409 folders.paused for a paused folder.
//	@Tags			scans
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int					true	"Folder ID"
//	@Param			request	body		StartScanRequest	true	"Kind"
//	@Success		202		{object}	ScanDataResponse
//	@Failure		400		{object}	errs.ErrorResponse
//	@Failure		401		{object}	errs.ErrorResponse
//	@Failure		404		{object}	errs.ErrorResponse
//	@Failure		409		{object}	errs.ErrorResponse
//	@Failure		500		{object}	errs.ErrorResponse
//	@Router			/api/v1/folders/{id}/scans [post]
func (h *Handler) StartScan(c *gin.Context) {
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}
	var req StartScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errs.RespondWithValidationError(c, err)
		return
	}
	scan, err := h.service.Start(c.Request.Context(), id, req.Kind)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, ScanDataResponse{Data: ResponseFromDB(*scan)})
}

// ListScans lists a folder's latest scans
//
//	@Summary		List folder scans
//	@Description	The latest 20 scans of the folder, newest first.
//	@Tags			scans
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"Folder ID"
//	@Success		200	{object}	ScansResponse
//	@Failure		400	{object}	errs.ErrorResponse
//	@Failure		401	{object}	errs.ErrorResponse
//	@Failure		404	{object}	errs.ErrorResponse
//	@Failure		500	{object}	errs.ErrorResponse
//	@Router			/api/v1/folders/{id}/scans [get]
func (h *Handler) ListScans(c *gin.Context) {
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}
	scans, err := h.service.List(c.Request.Context(), id)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	items := make([]ScanResponse, len(scans))
	for i, scan := range scans {
		items[i] = ResponseFromDB(scan)
	}
	c.JSON(http.StatusOK, ScansResponse{Data: items})
}
