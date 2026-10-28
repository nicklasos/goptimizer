package runs

import (
	"database/sql"
	"mime"
	"net/http"
	"strconv"
	"time"

	"app/internal"
	"app/internal/db"
	"app/internal/errs"
	"app/internal/middleware"
	"app/internal/storages"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service  *RunService
	archives *ArchiveService
}

func NewHandler(service *RunService, archives *ArchiveService) *Handler {
	return &Handler{service: service, archives: archives}
}

func runResponse(run db.Run) RunResponse {
	response := RunResponse{
		ID:           run.ID,
		PlanID:       run.PlanID,
		Trigger:      Trigger(run.Trigger),
		Status:       Status(run.Status),
		ScheduledFor: internal.FormatOptionalTime(run.ScheduledFor),
		StartedAt:    internal.FormatOptionalTime(run.StartedAt),
		FinishedAt:   internal.FormatOptionalTime(run.FinishedAt),
		CreatedAt:    internal.FormatTime(run.CreatedAt),
	}
	if run.StartedAt.Valid && run.FinishedAt.Valid {
		ms := run.FinishedAt.Time.Sub(run.StartedAt.Time).Milliseconds()
		response.DurationMs = &ms
	}
	if run.SizeBytes.Valid {
		response.SizeBytes = &run.SizeBytes.Int64
	}
	if run.Error.Valid {
		response.Error = &run.Error.String
	}
	return response
}

func archiveResponse(a db.Archive, storageName, storageKind string) ArchiveResponse {
	return ArchiveResponse{
		ID:          a.ID,
		RunID:       a.RunID,
		PlanID:      a.PlanID,
		StorageID:   a.StorageID,
		StorageName: storageName,
		StorageKind: storages.Kind(storageKind),
		ObjectKey:   a.ObjectKey,
		SizeBytes:   a.SizeBytes,
		SHA256:      a.Sha256,
		CreatedAt:   internal.FormatTime(sql.NullTime{Time: a.CreatedAt, Valid: true}),
		DeletedAt:   internal.FormatOptionalTime(a.DeletedAt),
	}
}

// RunPlan queues a run of a plan now
//
//	@Summary		Run a backup plan now
//	@Description	Queues a manual run, which the runner starts within seconds. A plan with a run queued or running already is a 409 plans.run_in_progress. Disabled plans can be run by hand. Admins only.
//	@Tags			runs
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"Plan ID"
//	@Success		202	{object}	RunDataResponse
//	@Failure		400	{object}	errs.ErrorResponse
//	@Failure		401	{object}	errs.ErrorResponse
//	@Failure		403	{object}	errs.ErrorResponse
//	@Failure		404	{object}	errs.ErrorResponse
//	@Failure		409	{object}	errs.ErrorResponse
//	@Failure		500	{object}	errs.ErrorResponse
//	@Router			/api/v1/plans/{id}/run [post]
func (h *Handler) RunPlan(c *gin.Context) {
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}

	run, err := h.service.RunNow(c.Request.Context(), id)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, RunDataResponse{Data: runResponse(*run)})
}

// ListRuns lists the runs of a plan, newest first
//
//	@Summary		List the runs of a plan (paginated)
//	@Description	Newest first, without logs. Admins only.
//	@Tags			runs
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id			path		int	true	"Plan ID"
//	@Param			page		query		int	false	"Page number (default: 1)"					default(1)
//	@Param			page_size	query		int	false	"Page size (default: 20, min: 1, max: 100)"	default(20)
//	@Success		200			{object}	PaginatedRunsResponse
//	@Failure		400			{object}	errs.ErrorResponse
//	@Failure		401			{object}	errs.ErrorResponse
//	@Failure		403			{object}	errs.ErrorResponse
//	@Failure		404			{object}	errs.ErrorResponse
//	@Failure		500			{object}	errs.ErrorResponse
//	@Router			/api/v1/plans/{id}/runs [get]
func (h *Handler) ListRuns(c *gin.Context) {
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}
	page, ok := middleware.Page(c)
	if !ok {
		return
	}

	result, err := h.service.ListRuns(c.Request.Context(), id, page.Page, page.PageSize)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	items := make([]RunResponse, len(result.Data))
	for i, row := range result.Data {
		items[i] = runResponse(db.Run(row))
	}
	c.JSON(http.StatusOK, PaginatedRunsResponse{
		Data:       items,
		Pagination: internal.NewPaginationMeta(result.Total, page.Page, page.PageSize),
	})
}

// GetRun retrieves a run with its log
//
//	@Summary		Get a run with its log
//	@Description	The log is written while the run goes on, every few seconds, so polling this shows it live. archives lists what the run stored, per storage. Admins only.
//	@Tags			runs
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"Run ID"
//	@Success		200	{object}	RunDetailDataResponse
//	@Failure		400	{object}	errs.ErrorResponse
//	@Failure		401	{object}	errs.ErrorResponse
//	@Failure		403	{object}	errs.ErrorResponse
//	@Failure		404	{object}	errs.ErrorResponse
//	@Failure		500	{object}	errs.ErrorResponse
//	@Router			/api/v1/runs/{id} [get]
func (h *Handler) GetRun(c *gin.Context) {
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}

	run, err := h.service.GetRun(c.Request.Context(), id)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	archives := make([]ArchiveResponse, len(run.Archives))
	for i, row := range run.Archives {
		archives[i] = archiveResponse(row.Archive, row.StorageName, row.StorageKind)
	}
	c.JSON(http.StatusOK, RunDetailDataResponse{Data: RunDetailResponse{
		RunResponse: runResponse(run.Run),
		Log:         run.Log,
		Archives:    archives,
	}})
}

// ListArchives lists the archives of a plan that retention has not deleted
//
//	@Summary		List the archives of a plan (paginated)
//	@Description	Archives still kept, newest first, one entry per storage. storage_id narrows the list to one storage. Admins only.
//	@Tags			runs
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id			path		int	true	"Plan ID"
//	@Param			storage_id	query		int	false	"Only the archives on this storage"
//	@Param			page		query		int	false	"Page number (default: 1)"					default(1)
//	@Param			page_size	query		int	false	"Page size (default: 20, min: 1, max: 100)"	default(20)
//	@Success		200			{object}	PaginatedArchivesResponse
//	@Failure		400			{object}	errs.ErrorResponse
//	@Failure		401			{object}	errs.ErrorResponse
//	@Failure		403			{object}	errs.ErrorResponse
//	@Failure		404			{object}	errs.ErrorResponse
//	@Failure		500			{object}	errs.ErrorResponse
//	@Router			/api/v1/plans/{id}/archives [get]
func (h *Handler) ListArchives(c *gin.Context) {
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}
	page, ok := middleware.Page(c)
	if !ok {
		return
	}
	var storageID int64
	if raw := c.Query("storage_id"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			errs.RespondWithError(c, errs.FieldError("storage_id", "invalid"))
			return
		}
		storageID = parsed
	}

	result, err := h.service.ListArchives(c.Request.Context(), id, storageID, page.Page, page.PageSize)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	items := make([]ArchiveResponse, len(result.Data))
	for i, row := range result.Data {
		items[i] = archiveResponse(row.Archive, row.StorageName, row.StorageKind)
	}
	c.JSON(http.StatusOK, PaginatedArchivesResponse{
		Data:       items,
		Pagination: internal.NewPaginationMeta(result.Total, page.Page, page.PageSize),
	})
}

// DownloadArchive streams an archive from its storage
//
//	@Summary		Download an archive
//	@Description	Streams the archive's object from its storage, as an attachment named after the object. An archive that retention deleted is a 404; a storage that fails, or has lost the object, a 502 storages.unreachable. A browser uses a download link instead. Admins only.
//	@Tags			runs
//	@Produce		octet-stream
//	@Security		BearerAuth
//	@Param			id	path		int	true	"Archive ID"
//	@Success		200	{file}		file
//	@Failure		400	{object}	errs.ErrorResponse
//	@Failure		401	{object}	errs.ErrorResponse
//	@Failure		403	{object}	errs.ErrorResponse
//	@Failure		404	{object}	errs.ErrorResponse
//	@Failure		500	{object}	errs.ErrorResponse
//	@Failure		502	{object}	errs.ErrorResponse
//	@Router			/api/v1/archives/{id}/download [get]
func (h *Handler) DownloadArchive(c *gin.Context) {
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}

	download, err := h.archives.Open(c.Request.Context(), id)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	streamDownload(c, download)
}

// CreateDownloadLink makes a short-lived link that downloads an archive without a session
//
//	@Summary		Make a download link for an archive
//	@Description	A token for GET /api/v1/downloads/{token}, valid for a minute. A browser follows it to save a large archive straight to disk; a request with the Authorization header cannot do that. Admins only.
//	@Tags			runs
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"Archive ID"
//	@Success		200	{object}	DownloadLinkDataResponse
//	@Failure		400	{object}	errs.ErrorResponse
//	@Failure		401	{object}	errs.ErrorResponse
//	@Failure		403	{object}	errs.ErrorResponse
//	@Failure		404	{object}	errs.ErrorResponse
//	@Failure		500	{object}	errs.ErrorResponse
//	@Router			/api/v1/archives/{id}/download-link [post]
func (h *Handler) CreateDownloadLink(c *gin.Context) {
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}

	token, expires, err := h.archives.Link(c.Request.Context(), id, time.Now())
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, DownloadLinkDataResponse{Data: DownloadLinkResponse{
		Token:     token,
		ExpiresAt: internal.FormatTime(sql.NullTime{Time: expires, Valid: true}),
	}})
}

// DownloadByLink streams the archive a download link names
//
//	@Summary		Download an archive by link
//	@Description	Needs no session: the token from POST /api/v1/archives/{id}/download-link is the permission, until it expires a minute after it was made. An invalid or expired token is a 403 archives.invalid_link.
//	@Tags			runs
//	@Produce		octet-stream
//	@Param			token	path		string	true	"Download token"
//	@Success		200		{file}		file
//	@Failure		403		{object}	errs.ErrorResponse
//	@Failure		404		{object}	errs.ErrorResponse
//	@Failure		500		{object}	errs.ErrorResponse
//	@Failure		502		{object}	errs.ErrorResponse
//	@Router			/api/v1/downloads/{token} [get]
func (h *Handler) DownloadByLink(c *gin.Context) {
	download, err := h.archives.OpenLink(c.Request.Context(), c.Param("token"), time.Now())
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	streamDownload(c, download)
}

func streamDownload(c *gin.Context, download *Download) {
	defer download.Close()
	c.DataFromReader(http.StatusOK, download.Size, "application/octet-stream", download, map[string]string{
		"Content-Disposition": mime.FormatMediaType("attachment", map[string]string{"filename": download.Name}),
		"Cache-Control":       "no-store",
	})
}
