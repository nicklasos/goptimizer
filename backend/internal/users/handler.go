package users

import (
	"net/http"

	"app/internal"
	"app/internal/db"
	"app/internal/errs"
	"app/internal/middleware"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *UserService
}

func NewHandler(service *UserService) *Handler {
	return &Handler{service: service}
}

func userResponseFromDB(user db.User) UserResponse {
	return UserResponse{
		ID:            user.ID,
		Email:         user.Email,
		Name:          user.Name,
		IsAdmin:       user.IsAdmin,
		EmailVerified: user.EmailVerifiedAt.Valid,
		CreatedAt:     internal.FormatTime(user.CreatedAt),
		UpdatedAt:     internal.FormatTime(user.UpdatedAt),
	}
}

// ListUsers lists every account, newest first
//
//	@Summary		List users
//	@Description	Paginated list of every account, admins and users. Admins only.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			page		query		int		false	"Page number"	default(1)
//	@Param			page_size	query		int		false	"Page size"		default(20)
//	@Success		200			{object}	PaginatedUsersResponse
//	@Failure		400			{object}	errs.ErrorResponse
//	@Failure		401			{object}	errs.ErrorResponse
//	@Failure		403			{object}	errs.ErrorResponse
//	@Failure		500			{object}	errs.ErrorResponse
//	@Router			/api/v1/users [get]
func (h *Handler) ListUsers(c *gin.Context) {
	pagination, ok := middleware.Page(c)
	if !ok {
		return
	}

	result, err := h.service.List(c.Request.Context(), pagination.Page, pagination.PageSize)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}

	items := make([]UserResponse, len(result.Data))
	for i, user := range result.Data {
		items[i] = userResponseFromDB(user)
	}

	c.JSON(http.StatusOK, PaginatedUsersResponse{
		Data:       items,
		Pagination: internal.NewPaginationMeta(result.Total, pagination.Page, pagination.PageSize),
	})
}

// CreateUser creates a user
//
//	@Summary		Create user
//	@Description	Create a user or, with is_admin, an admin. Admins only.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		CreateUserRequest	true	"User details"
//	@Success		200		{object}	UserDataResponse
//	@Failure		400		{object}	errs.ErrorResponse
//	@Failure		401		{object}	errs.ErrorResponse
//	@Failure		403		{object}	errs.ErrorResponse
//	@Failure		500		{object}	errs.ErrorResponse
//	@Router			/api/v1/users [post]
func (h *Handler) CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errs.RespondWithValidationError(c, err)
		return
	}

	user, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, UserDataResponse{Data: userResponseFromDB(*user)})
}

// UpdateUser updates a user's name, email and admin flag
//
//	@Summary		Update user
//	@Description	Update name, email and admin flag. Admins only. An admin cannot remove their own admin flag, and the last admin cannot be demoted.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int					true	"User ID"
//	@Param			request	body		UpdateUserRequest	true	"User details"
//	@Success		200		{object}	UserDataResponse
//	@Failure		400		{object}	errs.ErrorResponse
//	@Failure		401		{object}	errs.ErrorResponse
//	@Failure		403		{object}	errs.ErrorResponse
//	@Failure		404		{object}	errs.ErrorResponse
//	@Failure		409		{object}	errs.ErrorResponse
//	@Failure		500		{object}	errs.ErrorResponse
//	@Router			/api/v1/users/{id} [put]
func (h *Handler) UpdateUser(c *gin.Context) {
	actor, ok := middleware.CurrentActor(c)
	if !ok {
		return
	}
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errs.RespondWithValidationError(c, err)
		return
	}

	user, err := h.service.Update(c.Request.Context(), actor, id, req)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, UserDataResponse{Data: userResponseFromDB(*user)})
}

// SetPassword sets a new password for a user
//
//	@Summary		Set user password
//	@Description	Set a new password for any user. Admins only. Revokes the user's refresh tokens.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int					true	"User ID"
//	@Param			request	body		SetPasswordRequest	true	"New password"
//	@Success		200		{object}	internal.MessageResponse
//	@Failure		400		{object}	errs.ErrorResponse
//	@Failure		401		{object}	errs.ErrorResponse
//	@Failure		403		{object}	errs.ErrorResponse
//	@Failure		404		{object}	errs.ErrorResponse
//	@Failure		500		{object}	errs.ErrorResponse
//	@Router			/api/v1/users/{id}/set-password [post]
func (h *Handler) SetPassword(c *gin.Context) {
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}

	var req SetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errs.RespondWithValidationError(c, err)
		return
	}

	if err := h.service.SetPassword(c.Request.Context(), id, req.Password); err != nil {
		errs.RespondWithError(c, err)
		return
	}

	internal.RespondMessage(c, "Password updated successfully")
}

// DeleteUser deletes a user
//
//	@Summary		Delete user
//	@Description	Delete any user. Admins only. Cannot delete yourself or the last admin.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"User ID"
//	@Success		200	{object}	internal.MessageResponse
//	@Failure		400	{object}	errs.ErrorResponse
//	@Failure		401	{object}	errs.ErrorResponse
//	@Failure		403	{object}	errs.ErrorResponse
//	@Failure		404	{object}	errs.ErrorResponse
//	@Failure		409	{object}	errs.ErrorResponse
//	@Failure		500	{object}	errs.ErrorResponse
//	@Router			/api/v1/users/{id} [delete]
func (h *Handler) DeleteUser(c *gin.Context) {
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}

	actor, ok := middleware.CurrentActor(c)
	if !ok {
		return
	}

	if err := h.service.Delete(c.Request.Context(), actor, id); err != nil {
		errs.RespondWithError(c, err)
		return
	}

	internal.RespondMessage(c, "User deleted successfully")
}
