package folders

import (
	"net/http"

	"app/internal"
	"app/internal/errs"
	"app/internal/middleware"
	"app/internal/scans"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *FolderService
}

func NewHandler(service *FolderService) *Handler {
	return &Handler{service: service}
}

func folderResponse(folder Folder) FolderResponse {
	var lastScan *scans.ScanResponse
	if folder.LastScan != nil {
		scan := scans.ResponseFromDB(*folder.LastScan)
		lastScan = &scan
	}
	return FolderResponse{
		ID:                    folder.ID,
		Name:                  folder.Name,
		Kind:                  Kind(folder.Kind),
		Path:                  folder.Path,
		Enabled:               folder.Enabled,
		ScanIntervalMinutes:   folder.ScanIntervalMinutes,
		FullScanIntervalHours: folder.FullScanIntervalHours,
		Profile:               folder.Profile,
		LastScan:              lastScan,
		ImageCounts:           folder.ImageCounts,
		BytesBefore:           folder.BytesBefore,
		BytesNow:              folder.BytesNow,
		MissingTools:          folder.MissingTools,
		CreatedAt:             internal.FormatTime(folder.CreatedAt),
		UpdatedAt:             internal.FormatTime(folder.UpdatedAt),
	}
}

// ListFolders lists every folder
//
//	@Summary		List folders
//	@Description	Every folder, ordered by name, with its latest scan, its image counts by status, what its images weighed before optimizing and weigh now, and the programs its profile needs that are not installed. Not paginated.
//	@Tags			folders
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	FoldersResponse
//	@Failure		401	{object}	errs.ErrorResponse
//	@Failure		500	{object}	errs.ErrorResponse
//	@Router			/api/v1/folders [get]
func (h *Handler) ListFolders(c *gin.Context) {
	folders, err := h.service.List(c.Request.Context())
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	items := make([]FolderResponse, len(folders))
	for i, folder := range folders {
		items[i] = folderResponse(folder)
	}
	c.JSON(http.StatusOK, FoldersResponse{Data: items})
}

// GetFolder returns one folder
//
//	@Summary		Get folder
//	@Tags			folders
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"Folder ID"
//	@Success		200	{object}	FolderDataResponse
//	@Failure		400	{object}	errs.ErrorResponse
//	@Failure		401	{object}	errs.ErrorResponse
//	@Failure		404	{object}	errs.ErrorResponse
//	@Failure		500	{object}	errs.ErrorResponse
//	@Router			/api/v1/folders/{id} [get]
func (h *Handler) GetFolder(c *gin.Context) {
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}
	folder, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, FolderDataResponse{Data: folderResponse(*folder)})
}

// CreateFolder adds a folder
//
//	@Summary		Create folder
//	@Description	Add a folder. The path must be an absolute, readable directory outside the system directories that overlaps no other folder; symlinks are resolved and the real path is stored. Omitted profile fields take their defaults.
//	@Tags			folders
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		FolderRequest	true	"Folder"
//	@Success		200		{object}	FolderDataResponse
//	@Failure		400		{object}	errs.ErrorResponse
//	@Failure		401		{object}	errs.ErrorResponse
//	@Failure		500		{object}	errs.ErrorResponse
//	@Router			/api/v1/folders [post]
func (h *Handler) CreateFolder(c *gin.Context) {
	var req FolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errs.RespondWithValidationError(c, err)
		return
	}
	folder, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, FolderDataResponse{Data: folderResponse(*folder)})
}

// UpdateFolder replaces a folder's settings
//
//	@Summary		Update folder
//	@Description	Replace name, path, intervals and profile, with the same rules as create. Does not change whether it is paused.
//	@Tags			folders
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int				true	"Folder ID"
//	@Param			request	body		FolderRequest	true	"Folder"
//	@Success		200		{object}	FolderDataResponse
//	@Failure		400		{object}	errs.ErrorResponse
//	@Failure		401		{object}	errs.ErrorResponse
//	@Failure		404		{object}	errs.ErrorResponse
//	@Failure		500		{object}	errs.ErrorResponse
//	@Router			/api/v1/folders/{id} [put]
func (h *Handler) UpdateFolder(c *gin.Context) {
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}
	var req FolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errs.RespondWithValidationError(c, err)
		return
	}
	folder, err := h.service.Update(c.Request.Context(), id, req)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, FolderDataResponse{Data: folderResponse(*folder)})
}

// SetFolderEnabled pauses or resumes a folder
//
//	@Summary		Pause or resume folder
//	@Tags			folders
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int					true	"Folder ID"
//	@Param			request	body		SetEnabledRequest	true	"Enabled"
//	@Success		200		{object}	FolderDataResponse
//	@Failure		400		{object}	errs.ErrorResponse
//	@Failure		401		{object}	errs.ErrorResponse
//	@Failure		404		{object}	errs.ErrorResponse
//	@Failure		500		{object}	errs.ErrorResponse
//	@Router			/api/v1/folders/{id}/enabled [put]
func (h *Handler) SetFolderEnabled(c *gin.Context) {
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}
	var req SetEnabledRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errs.RespondWithValidationError(c, err)
		return
	}
	folder, err := h.service.SetEnabled(c.Request.Context(), id, *req.Enabled)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, FolderDataResponse{Data: folderResponse(*folder)})
}

// DeleteFolder removes a folder
//
//	@Summary		Delete folder
//	@Description	Forget a folder. Its files on disk are not touched.
//	@Tags			folders
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"Folder ID"
//	@Success		200	{object}	internal.MessageResponse
//	@Failure		400	{object}	errs.ErrorResponse
//	@Failure		401	{object}	errs.ErrorResponse
//	@Failure		404	{object}	errs.ErrorResponse
//	@Failure		500	{object}	errs.ErrorResponse
//	@Router			/api/v1/folders/{id} [delete]
func (h *Handler) DeleteFolder(c *gin.Context) {
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		errs.RespondWithError(c, err)
		return
	}
	internal.RespondMessage(c, "Folder deleted successfully")
}
