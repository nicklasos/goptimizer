package runs

import (
	"net/http"
	"time"

	"app/internal"
	"app/internal/errs"
	"app/internal/middleware"
	"app/internal/plans"

	"github.com/gin-gonic/gin"
)

type RestoreHandler struct {
	service *RestoreService
}

func NewRestoreHandler(service *RestoreService) *RestoreHandler {
	return &RestoreHandler{service: service}
}

func restoreResponse(r *Restore) (RestoreResponse, error) {
	target, err := plans.RespondRestoreTarget(plans.SourceKind(r.Restore.SourceKind), []byte(r.Restore.TargetConfig))
	if err != nil {
		return RestoreResponse{}, err
	}
	response := RestoreResponse{
		ID:         r.Restore.ID,
		PlanID:     r.Restore.PlanID,
		Target:     target,
		SecretSet:  r.Restore.TargetSecret != nil,
		Status:     Status(r.Restore.Status),
		StartedAt:  internal.FormatOptionalTime(r.Restore.StartedAt),
		FinishedAt: internal.FormatOptionalTime(r.Restore.FinishedAt),
		CreatedAt:  internal.FormatTime(r.Restore.CreatedAt),
	}
	if r.Restore.ArchiveID.Valid {
		response.ArchiveID = &r.Restore.ArchiveID.Int64
	}
	if r.ArchiveKey.Valid {
		response.Archive = &RestoreArchiveResponse{
			ObjectKey:   r.ArchiveKey.String,
			StorageName: r.StorageName.String,
			SizeBytes:   r.ArchiveSize.Int64,
			CreatedAt:   internal.FormatTime(r.ArchiveCreatedAt),
		}
	}
	if r.Restore.StartedAt.Valid && r.Restore.FinishedAt.Valid {
		ms := r.Restore.FinishedAt.Time.Sub(r.Restore.StartedAt.Time).Milliseconds()
		response.DurationMs = &ms
	}
	if r.Restore.Error.Valid {
		response.Error = &r.Restore.Error.String
	}
	if r.Restore.CreatedBy.Valid && r.CreatedByEmail.Valid {
		response.CreatedBy = &RestoreUserResponse{ID: r.Restore.CreatedBy.Int64, Name: r.CreatedByName.String, Email: r.CreatedByEmail.String}
	}
	return response, nil
}

func (h *RestoreHandler) respondDetail(c *gin.Context, status int, restore *Restore) {
	response, err := restoreResponse(restore)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	c.JSON(status, RestoreDetailDataResponse{Data: RestoreDetailResponse{RestoreResponse: response, Log: restore.Restore.Log}})
}

// GetRestoreTarget is the target a restore of an archive starts from
//
//	@Summary		Get the default target of a restore
//	@Description	The plan's own source, in the block of its kind, for a restore form to start from: for a database its server and database, for SQLite its path, for files a new directory in the work directory. Restoring over the plan's own source needs confirm. Admins only.
//	@Tags			restores
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"Archive ID"
//	@Success		200	{object}	RestoreTargetDataResponse
//	@Failure		400	{object}	errs.ErrorResponse
//	@Failure		401	{object}	errs.ErrorResponse
//	@Failure		403	{object}	errs.ErrorResponse
//	@Failure		404	{object}	errs.ErrorResponse
//	@Failure		409	{object}	errs.ErrorResponse
//	@Failure		500	{object}	errs.ErrorResponse
//	@Router			/api/v1/archives/{id}/restore-target [get]
func (h *RestoreHandler) GetRestoreTarget(c *gin.Context) {
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}

	target, err := h.service.DefaultTarget(c.Request.Context(), id, time.Now())
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, RestoreTargetDataResponse{Data: target})
}

// RestoreArchive queues a restore of an archive
//
//	@Summary		Restore an archive
//	@Description	Queues a restore of the archive into the target in the block of the plan's source kind. Blank database connection fields and a blank password are the plan's. A target that is the plan's own source (the same server and database, the same SQLite path, or files to their original locations) needs confirm set to the database name, the path or "original locations", or it is a 400 restores.confirm_mismatch on confirm. The worker starts it once no run or other restore of the plan is running; a plan with a restore queued or running already is a 409 restores.in_progress. A deleted archive is a 404 archives.not_found. Admins only.
//	@Tags			restores
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int						true	"Archive ID"
//	@Param			request	body		plans.RestoreRequest	true	"Restore target"
//	@Success		202		{object}	RestoreDetailDataResponse
//	@Failure		400		{object}	errs.ValidationErrorResponse
//	@Failure		401		{object}	errs.ErrorResponse
//	@Failure		403		{object}	errs.ErrorResponse
//	@Failure		404		{object}	errs.ErrorResponse
//	@Failure		409		{object}	errs.ErrorResponse
//	@Failure		500		{object}	errs.ErrorResponse
//	@Router			/api/v1/archives/{id}/restore [post]
func (h *RestoreHandler) RestoreArchive(c *gin.Context) {
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		return
	}
	var req plans.RestoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errs.RespondWithValidationError(c, err)
		return
	}

	restore, err := h.service.Start(c.Request.Context(), id, userID, &req, time.Now())
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	h.respondDetail(c, http.StatusAccepted, restore)
}

// GetRestore retrieves a restore with its log
//
//	@Summary		Get a restore with its log
//	@Description	The log is written while the restore goes on, every few seconds, so polling this shows it live. Admins only.
//	@Tags			restores
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"Restore ID"
//	@Success		200	{object}	RestoreDetailDataResponse
//	@Failure		400	{object}	errs.ErrorResponse
//	@Failure		401	{object}	errs.ErrorResponse
//	@Failure		403	{object}	errs.ErrorResponse
//	@Failure		404	{object}	errs.ErrorResponse
//	@Failure		500	{object}	errs.ErrorResponse
//	@Router			/api/v1/restores/{id} [get]
func (h *RestoreHandler) GetRestore(c *gin.Context) {
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}

	restore, err := h.service.GetRestore(c.Request.Context(), id)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	h.respondDetail(c, http.StatusOK, restore)
}

// ListRestores lists the restores of a plan, newest first
//
//	@Summary		List the restores of a plan (paginated)
//	@Description	Newest first, without logs. Admins only.
//	@Tags			restores
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id			path		int	true	"Plan ID"
//	@Param			page		query		int	false	"Page number (default: 1)"					default(1)
//	@Param			page_size	query		int	false	"Page size (default: 20, min: 1, max: 100)"	default(20)
//	@Success		200			{object}	PaginatedRestoresResponse
//	@Failure		400			{object}	errs.ErrorResponse
//	@Failure		401			{object}	errs.ErrorResponse
//	@Failure		403			{object}	errs.ErrorResponse
//	@Failure		404			{object}	errs.ErrorResponse
//	@Failure		500			{object}	errs.ErrorResponse
//	@Router			/api/v1/plans/{id}/restores [get]
func (h *RestoreHandler) ListRestores(c *gin.Context) {
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}
	page, ok := middleware.Page(c)
	if !ok {
		return
	}

	result, err := h.service.ListRestores(c.Request.Context(), id, page.Page, page.PageSize)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	items := make([]RestoreResponse, len(result.Data))
	for i := range result.Data {
		if items[i], err = restoreResponse(&result.Data[i]); err != nil {
			errs.RespondWithError(c, err)
			return
		}
	}
	c.JSON(http.StatusOK, PaginatedRestoresResponse{
		Data:       items,
		Pagination: internal.NewPaginationMeta(result.Total, page.Page, page.PageSize),
	})
}
