package storages

import (
	"database/sql"
	"net/http"
	"strconv"

	"app/internal"
	"app/internal/db"
	"app/internal/errs"
	"app/internal/middleware"
	"app/internal/storages/driver"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *StorageService
}

func NewHandler(service *StorageService) *Handler {
	return &Handler{service: service}
}

func storageResponse(storage *db.Storage) (*StorageResponse, error) {
	response := &StorageResponse{
		ID:        storage.ID,
		Name:      storage.Name,
		Kind:      Kind(storage.Kind),
		SecretSet: storage.Secret != nil,
		CreatedAt: internal.FormatTime(storage.CreatedAt),
		UpdatedAt: internal.FormatTime(storage.UpdatedAt),
	}
	if err := kinds[response.Kind].respond([]byte(storage.Config), response); err != nil {
		return nil, err
	}
	return response, nil
}

func testResultResponse(result *TestResult) TestResultResponse {
	if result.Failure == nil {
		return TestResultResponse{OK: true}
	}
	return TestResultResponse{Step: string(result.Failure.Step), Error: result.Failure.Err.Error()}
}

func objectResponse(entry driver.Entry) ObjectResponse {
	response := ObjectResponse{Name: entry.Name, IsDir: entry.IsDir, Size: entry.Size}
	if !entry.ModifiedAt.IsZero() {
		response.ModifiedAt = internal.FormatTime(sql.NullTime{Time: entry.ModifiedAt, Valid: true})
	}
	return response
}

func (h *Handler) respondStorage(c *gin.Context, storage *db.Storage) {
	response, err := storageResponse(storage)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, StorageDataResponse{Data: response})
}

// ListStorages lists the storages, by name
//
//	@Summary		List storages (paginated)
//	@Description	Every storage, without credentials. Admins only.
//	@Tags			storages
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			page		query		int	false	"Page number (default: 1)"					default(1)
//	@Param			page_size	query		int	false	"Page size (default: 20, min: 1, max: 100)"	default(20)
//	@Success		200			{object}	PaginatedStoragesResponse
//	@Failure		400			{object}	errs.ErrorResponse
//	@Failure		401			{object}	errs.ErrorResponse
//	@Failure		403			{object}	errs.ErrorResponse
//	@Failure		500			{object}	errs.ErrorResponse
//	@Router			/api/v1/storages [get]
func (h *Handler) ListStorages(c *gin.Context) {
	page, ok := middleware.Page(c)
	if !ok {
		return
	}

	result, err := h.service.ListStorages(c.Request.Context(), page.Page, page.PageSize)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}

	items := make([]StorageResponse, len(result.Data))
	for i := range result.Data {
		item, err := storageResponse(&result.Data[i])
		if err != nil {
			errs.RespondWithError(c, err)
			return
		}
		items[i] = *item
	}

	c.JSON(http.StatusOK, PaginatedStoragesResponse{
		Data:       items,
		Pagination: internal.NewPaginationMeta(result.Total, page.Page, page.PageSize),
	})
}

// GetStorage retrieves a storage by ID
//
//	@Summary		Get storage
//	@Description	A storage without its credentials: secret_set says whether they are stored. Admins only.
//	@Tags			storages
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"Storage ID"
//	@Success		200	{object}	StorageDataResponse
//	@Failure		400	{object}	errs.ErrorResponse
//	@Failure		401	{object}	errs.ErrorResponse
//	@Failure		403	{object}	errs.ErrorResponse
//	@Failure		404	{object}	errs.ErrorResponse
//	@Failure		500	{object}	errs.ErrorResponse
//	@Router			/api/v1/storages/{id} [get]
func (h *Handler) GetStorage(c *gin.Context) {
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}

	storage, err := h.service.GetStorage(c.Request.Context(), id)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	h.respondStorage(c, storage)
}

// CreateStorage creates a storage
//
//	@Summary		Create storage
//	@Description	Send the block named by kind (local, s3 or gcs) and no other. S3 and GCS need credentials. Admins only.
//	@Tags			storages
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		StorageRequest	true	"Storage"
//	@Success		200		{object}	StorageDataResponse
//	@Failure		400		{object}	errs.ValidationErrorResponse
//	@Failure		401		{object}	errs.ErrorResponse
//	@Failure		403		{object}	errs.ErrorResponse
//	@Failure		500		{object}	errs.ErrorResponse
//	@Router			/api/v1/storages [post]
func (h *Handler) CreateStorage(c *gin.Context) {
	var req StorageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errs.RespondWithValidationError(c, err)
		return
	}

	storage, err := h.service.CreateStorage(c.Request.Context(), &req)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	h.respondStorage(c, storage)
}

// UpdateStorage updates a storage
//
//	@Summary		Update storage
//	@Description	Credentials left blank keep the stored ones, unless the kind changes. Admins only.
//	@Tags			storages
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int				true	"Storage ID"
//	@Param			request	body		StorageRequest	true	"Storage"
//	@Success		200		{object}	StorageDataResponse
//	@Failure		400		{object}	errs.ValidationErrorResponse
//	@Failure		401		{object}	errs.ErrorResponse
//	@Failure		403		{object}	errs.ErrorResponse
//	@Failure		404		{object}	errs.ErrorResponse
//	@Failure		500		{object}	errs.ErrorResponse
//	@Router			/api/v1/storages/{id} [put]
func (h *Handler) UpdateStorage(c *gin.Context) {
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}

	var req StorageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errs.RespondWithValidationError(c, err)
		return
	}

	storage, err := h.service.UpdateStorage(c.Request.Context(), id, &req)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	h.respondStorage(c, storage)
}

// DeleteStorage deletes a storage
//
//	@Summary		Delete storage
//	@Description	Removes the storage from Gogobackup. The files in it are not touched. A storage that backup plans use is a 409 storages.in_use whose details.plans lists them. Admins only.
//	@Tags			storages
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"Storage ID"
//	@Success		200	{object}	internal.MessageResponse
//	@Failure		400	{object}	errs.ErrorResponse
//	@Failure		401	{object}	errs.ErrorResponse
//	@Failure		403	{object}	errs.ErrorResponse
//	@Failure		404	{object}	errs.ErrorResponse
//	@Failure		409	{object}	errs.ErrorResponse
//	@Failure		500	{object}	errs.ErrorResponse
//	@Router			/api/v1/storages/{id} [delete]
func (h *Handler) DeleteStorage(c *gin.Context) {
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}

	if err := h.service.DeleteStorage(c.Request.Context(), id); err != nil {
		errs.RespondWithError(c, err)
		return
	}
	internal.RespondMessage(c, "Storage deleted successfully")
}

// TestDraft tests a storage form that has not been saved
//
//	@Summary		Test an unsaved storage
//	@Description	Writes a small object, reads it back and deletes it. A failed test is a 200 whose step says what failed. On an edit form, pass storage_id so blank credentials are taken from that storage. Admins only.
//	@Tags			storages
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			storage_id	query		int				false	"The storage being edited, whose credentials a blank form uses"
//	@Param			request		body		StorageRequest	true	"Storage"
//	@Success		200			{object}	TestResultDataResponse
//	@Failure		400			{object}	errs.ValidationErrorResponse
//	@Failure		401			{object}	errs.ErrorResponse
//	@Failure		403			{object}	errs.ErrorResponse
//	@Failure		404			{object}	errs.ErrorResponse
//	@Failure		500			{object}	errs.ErrorResponse
//	@Router			/api/v1/storages/test [post]
func (h *Handler) TestDraft(c *gin.Context) {
	var storageID int64
	if raw := c.Query("storage_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			errs.RespondWithError(c, errs.FieldError("storage_id", "invalid"))
			return
		}
		storageID = id
	}

	var req StorageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errs.RespondWithValidationError(c, err)
		return
	}

	result, err := h.service.TestDraft(c.Request.Context(), &req, storageID)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, TestResultDataResponse{Data: testResultResponse(result)})
}

// TestStorage tests a saved storage
//
//	@Summary		Test a storage
//	@Description	Writes a small object, reads it back and deletes it. A failed test is a 200 whose step says what failed. Admins only.
//	@Tags			storages
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"Storage ID"
//	@Success		200	{object}	TestResultDataResponse
//	@Failure		400	{object}	errs.ErrorResponse
//	@Failure		401	{object}	errs.ErrorResponse
//	@Failure		403	{object}	errs.ErrorResponse
//	@Failure		404	{object}	errs.ErrorResponse
//	@Failure		500	{object}	errs.ErrorResponse
//	@Router			/api/v1/storages/{id}/test [post]
func (h *Handler) TestStorage(c *gin.Context) {
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}

	result, err := h.service.TestStorage(c.Request.Context(), id)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, TestResultDataResponse{Data: testResultResponse(result)})
}

// ListObjects lists one folder of a storage
//
//	@Summary		Browse a storage
//	@Description	The folders, then the files, directly under prefix (empty for the top). A storage that cannot be reached is a 502 with the reason in details.reason. Admins only.
//	@Tags			storages
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int		true	"Storage ID"
//	@Param			prefix	query		string	false	"Folder inside the storage, such as nightly/2026/"
//	@Success		200		{object}	ObjectsResponse
//	@Failure		400		{object}	errs.ErrorResponse
//	@Failure		401		{object}	errs.ErrorResponse
//	@Failure		403		{object}	errs.ErrorResponse
//	@Failure		404		{object}	errs.ErrorResponse
//	@Failure		500		{object}	errs.ErrorResponse
//	@Failure		502		{object}	errs.ErrorResponse
//	@Router			/api/v1/storages/{id}/objects [get]
func (h *Handler) ListObjects(c *gin.Context) {
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}

	entries, err := h.service.ListObjects(c.Request.Context(), id, c.Query("prefix"))
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}

	items := make([]ObjectResponse, len(entries))
	for i, entry := range entries {
		items[i] = objectResponse(entry)
	}
	c.JSON(http.StatusOK, ObjectsResponse{Data: items})
}
