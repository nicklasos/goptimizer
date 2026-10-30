package images

import (
	"net/http"
	"time"

	"app/internal/errs"
	"app/internal/middleware"

	"github.com/gin-gonic/gin"
)

const defaultLimit = 50

type Handler struct {
	service *ImageService
}

func NewHandler(service *ImageService) *Handler {
	return &Handler{service: service}
}

// ListImages lists one page of a folder's images
//
//	@Summary		List folder images
//	@Description	One keyset page of the folder's images: every status or one, the latest change first, or with sort=saved and status=optimized the most bytes saved first. q keeps the paths that contain it, ignoring ASCII case, listed by path. Pass the response's next_cursor as cursor for the next page.
//	@Tags			images
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int		true	"Folder ID"
//	@Param			status	query		string	false	"Status"	Enums(pending, optimizing, optimized, kept, skipped, failed, gone)
//	@Param			q		query		string	false	"Text the path contains"
//	@Param			sort	query		string	false	"Order"	Enums(updated, saved)	default(updated)
//	@Param			cursor	query		string	false	"next_cursor of the previous page"
//	@Param			limit	query		int		false	"Page size"	default(50)	minimum(1)	maximum(100)
//	@Success		200		{object}	ImagesPageResponse
//	@Failure		400		{object}	errs.ErrorResponse
//	@Failure		401		{object}	errs.ErrorResponse
//	@Failure		404		{object}	errs.ErrorResponse
//	@Failure		500		{object}	errs.ErrorResponse
//	@Router			/api/v1/folders/{id}/images [get]
func (h *Handler) ListImages(c *gin.Context) {
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}
	var req ListImagesQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		errs.RespondWithValidationError(c, err)
		return
	}
	query, err := parseListQuery(req)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	page, err := h.service.List(c.Request.Context(), id, query)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	items := make([]ImageResponse, len(page.Images))
	for i, image := range page.Images {
		items[i] = imageResponse(image)
	}
	response := ImagesPage{Items: items, Order: string(page.Sort)}
	if page.Next != nil {
		next := EncodeCursor(*page.Next)
		response.NextCursor = &next
	}
	c.JSON(http.StatusOK, ImagesPageResponse{Data: response})
}

func parseListQuery(req ListImagesQuery) (Query, error) {
	query := Query{Status: Status(req.Status), Search: req.Search, Sort: SortUpdated, Limit: req.Limit}
	if query.Limit == 0 {
		query.Limit = defaultLimit
	}
	switch {
	case req.Sort == string(SortSaved) && query.Status != StatusOptimized:
		return Query{}, errs.FieldError("sort", "invalid")
	case req.Search != "":
		query.Sort = SortPath
	case req.Sort == string(SortSaved):
		query.Sort = SortSaved
	}
	if req.Cursor != "" {
		cursor, err := DecodeCursor(req.Cursor)
		if err != nil {
			return Query{}, errs.FieldError("cursor", "invalid")
		}
		query.After = &cursor
	}
	return query, nil
}

// RetryImage makes a failed image pending again
//
//	@Summary		Retry image
//	@Description	A failed image goes back to pending with its attempts reset, and the optimizer takes it again. 409 images.not_failed for an image in any other status.
//	@Tags			images
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"Image ID"
//	@Success		200	{object}	ImageDataResponse
//	@Failure		400	{object}	errs.ErrorResponse
//	@Failure		401	{object}	errs.ErrorResponse
//	@Failure		404	{object}	errs.ErrorResponse
//	@Failure		409	{object}	errs.ErrorResponse
//	@Failure		500	{object}	errs.ErrorResponse
//	@Router			/api/v1/images/{id}/retry [post]
func (h *Handler) RetryImage(c *gin.Context) {
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}
	image, err := h.service.Retry(c.Request.Context(), id, time.Now())
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, ImageDataResponse{Data: imageResponse(image)})
}

// RetryFailedImages makes every failed image of a folder pending again
//
//	@Summary		Retry failed images
//	@Description	Every failed image of the folder goes back to pending with its attempts reset. Returns how many.
//	@Tags			images
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"Folder ID"
//	@Success		200	{object}	RetriedResponse
//	@Failure		400	{object}	errs.ErrorResponse
//	@Failure		401	{object}	errs.ErrorResponse
//	@Failure		404	{object}	errs.ErrorResponse
//	@Failure		500	{object}	errs.ErrorResponse
//	@Router			/api/v1/folders/{id}/images/retry-failed [post]
func (h *Handler) RetryFailedImages(c *gin.Context) {
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}
	n, err := h.service.RetryFailed(c.Request.Context(), id, time.Now())
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, RetriedResponse{Data: RetriedData{Retried: n}})
}

// DailySavings lists what the optimizations saved per day
//
//	@Summary		Daily savings
//	@Description	The bytes the optimizations saved on each of the last 30 UTC days, oldest first, today last, a day without any as zero. One folder with folder_id, every folder together without.
//	@Tags			images
//	@Produce		json
//	@Security		BearerAuth
//	@Param			folder_id	query		int	false	"Folder ID"
//	@Success		200			{object}	DailySavingsResponse
//	@Failure		400			{object}	errs.ErrorResponse
//	@Failure		401			{object}	errs.ErrorResponse
//	@Failure		404			{object}	errs.ErrorResponse
//	@Failure		500			{object}	errs.ErrorResponse
//	@Router			/api/v1/daily-savings [get]
func (h *Handler) DailySavings(c *gin.Context) {
	var req DailySavingsQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		errs.RespondWithValidationError(c, err)
		return
	}
	var folderID *int64
	if req.FolderID != 0 {
		folderID = &req.FolderID
	}
	days, err := h.service.Savings(c.Request.Context(), folderID, time.Now())
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	items := make([]DaySavingsResponse, len(days))
	for i, day := range days {
		items[i] = DaySavingsResponse{Day: day.Day, Optimized: day.Optimized, BytesSaved: day.BytesSaved}
	}
	c.JSON(http.StatusOK, DailySavingsResponse{Data: items})
}
