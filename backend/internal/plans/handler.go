package plans

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"app/internal"
	"app/internal/errs"
	"app/internal/middleware"
	"app/internal/storages"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *PlanService
}

func NewHandler(service *PlanService) *Handler {
	return &Handler{service: service}
}

func planResponse(plan *Plan, now time.Time) (*PlanResponse, error) {
	response := &PlanResponse{
		ID:          plan.ID,
		Name:        plan.Name,
		SourceKind:  SourceKind(plan.SourceKind),
		SecretSet:   plan.SourceSecret != nil,
		MaxAgeDays:  plan.MaxAgeDays,
		KeepWeekly:  plan.KeepWeekly,
		KeepMonthly: plan.KeepMonthly,
		Storages:    make([]PlanStorageResponse, len(plan.Storages)),
		Enabled:     plan.Enabled,
		CreatedAt:   internal.FormatTime(plan.CreatedAt),
		UpdatedAt:   internal.FormatTime(plan.UpdatedAt),
	}
	if err := sources[response.SourceKind].respond([]byte(plan.SourceConfig), response); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(plan.Schedules), &response.Schedules); err != nil {
		return nil, err
	}
	if next, ok := NextRun(response.Schedules, now); ok && plan.Enabled {
		response.NextRunAt = internal.FormatOptionalTime(sql.NullTime{Time: next, Valid: true})
	}
	for i, storage := range plan.Storages {
		response.Storages[i] = PlanStorageResponse{ID: storage.ID, Name: storage.Name, Kind: storages.Kind(storage.Kind)}
	}
	if run := plan.LastRun; run != nil {
		response.LastRun = &LastRunResponse{
			ID:         run.ID,
			Status:     run.Status,
			Trigger:    run.Trigger,
			StartedAt:  internal.FormatOptionalTime(run.StartedAt),
			FinishedAt: internal.FormatOptionalTime(run.FinishedAt),
		}
		if run.SizeBytes.Valid {
			response.LastRun.SizeBytes = &run.SizeBytes.Int64
		}
	}
	response.LastSuccess = internal.FormatOptionalTime(plan.LastSuccessAt)
	if plan.LastSuccessSize.Valid {
		response.LastSize = &plan.LastSuccessSize.Int64
	}
	response.Health = PlanHealth(plan.Enabled, response.Schedules, plan.UpdatedAt.Time, plan.LastRun, plan.Restoring, plan.LastSuccessAt, now)
	response.Folder = Folder(plan.Name)
	return response, nil
}

func dashboardResponse(plans []DashboardPlan, now time.Time) (*DashboardResponse, error) {
	response := &DashboardResponse{Plans: make([]DashboardPlanResponse, len(plans))}
	for i := range plans {
		plan, err := planResponse(&plans[i].Plan, now)
		if err != nil {
			return nil, err
		}
		days := make([]DayResponse, len(plans[i].Days))
		for j, day := range plans[i].Days {
			days[j] = DayResponse{Date: day.Date, Succeeded: day.Succeeded, Failed: day.Failed}
		}
		response.Plans[i] = DashboardPlanResponse{PlanResponse: *plan, Days: days}
		response.Summary.add(plan.Health)
	}
	return response, nil
}

func (s *HealthSummaryResponse) add(health Health) {
	counters := map[Health]*int{
		HealthRestoring: &s.Restoring, HealthRunning: &s.Running, HealthFailed: &s.Failed, HealthPaused: &s.Paused,
		HealthOverdue: &s.Overdue, HealthNever: &s.Never, HealthOK: &s.OK,
	}
	*counters[health]++
}

func sourceTestResponse(result *TestResult) SourceTestResponse {
	response := SourceTestResponse{OK: result.Failure == nil, ServerVersion: result.ServerVersion}
	if result.Failure != nil {
		response.Step = string(result.Failure.Step)
		response.Error = result.Failure.Message
	}
	if result.Tool != nil {
		response.Tool = &ToolResponse{Name: result.Tool.Name, Version: result.Tool.Version, Found: result.Tool.Found}
	}
	return response
}

func (h *Handler) respondPlan(c *gin.Context, plan *Plan) {
	response, err := planResponse(plan, time.Now())
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, PlanDataResponse{Data: response})
}

// ListPlans lists the backup plans, by name
//
//	@Summary		List backup plans (paginated)
//	@Description	Every plan, without source passwords. Admins only.
//	@Tags			plans
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			page		query		int	false	"Page number (default: 1)"					default(1)
//	@Param			page_size	query		int	false	"Page size (default: 20, min: 1, max: 100)"	default(20)
//	@Success		200			{object}	PaginatedPlansResponse
//	@Failure		400			{object}	errs.ErrorResponse
//	@Failure		401			{object}	errs.ErrorResponse
//	@Failure		403			{object}	errs.ErrorResponse
//	@Failure		500			{object}	errs.ErrorResponse
//	@Router			/api/v1/plans [get]
func (h *Handler) ListPlans(c *gin.Context) {
	page, ok := middleware.Page(c)
	if !ok {
		return
	}

	result, err := h.service.ListPlans(c.Request.Context(), page.Page, page.PageSize)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}

	now := time.Now()
	items := make([]PlanResponse, len(result.Data))
	for i := range result.Data {
		item, err := planResponse(&result.Data[i], now)
		if err != nil {
			errs.RespondWithError(c, err)
			return
		}
		items[i] = *item
	}

	c.JSON(http.StatusOK, PaginatedPlansResponse{
		Data:       items,
		Pagination: internal.NewPaginationMeta(result.Total, page.Page, page.PageSize),
	})
}

// Dashboard lists every plan with its health and its runs per day
//
//	@Summary		Plans dashboard
//	@Description	Every plan, by name, not paginated: with its health, its latest run, its last successful backup and how many runs succeeded and failed on each of the last 30 UTC days. summary counts the plans per health. Admins only.
//	@Tags			plans
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	DashboardDataResponse
//	@Failure		401	{object}	errs.ErrorResponse
//	@Failure		403	{object}	errs.ErrorResponse
//	@Failure		500	{object}	errs.ErrorResponse
//	@Router			/api/v1/dashboard [get]
func (h *Handler) Dashboard(c *gin.Context) {
	now := time.Now()
	plans, err := h.service.Dashboard(c.Request.Context(), now)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	response, err := dashboardResponse(plans, now)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, DashboardDataResponse{Data: *response})
}

// SetEnabled pauses or resumes a backup plan
//
//	@Summary		Pause or resume a backup plan
//	@Description	A paused plan keeps its settings and can still be run by hand, but does not run on its schedule. Resuming starts with the next slot, not the ones missed while paused. Admins only.
//	@Tags			plans
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int					true	"Plan ID"
//	@Param			request	body		SetEnabledRequest	true	"Enabled"
//	@Success		200		{object}	PlanDataResponse
//	@Failure		400		{object}	errs.ValidationErrorResponse
//	@Failure		401		{object}	errs.ErrorResponse
//	@Failure		403		{object}	errs.ErrorResponse
//	@Failure		404		{object}	errs.ErrorResponse
//	@Failure		500		{object}	errs.ErrorResponse
//	@Router			/api/v1/plans/{id}/enabled [put]
func (h *Handler) SetEnabled(c *gin.Context) {
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}

	var req SetEnabledRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errs.RespondWithValidationError(c, err)
		return
	}

	plan, err := h.service.SetEnabled(c.Request.Context(), id, *req.Enabled)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	h.respondPlan(c, plan)
}

// GetPlan retrieves a backup plan by ID
//
//	@Summary		Get backup plan
//	@Description	A plan without its source password: secret_set says whether one is stored. Admins only.
//	@Tags			plans
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"Plan ID"
//	@Success		200	{object}	PlanDataResponse
//	@Failure		400	{object}	errs.ErrorResponse
//	@Failure		401	{object}	errs.ErrorResponse
//	@Failure		403	{object}	errs.ErrorResponse
//	@Failure		404	{object}	errs.ErrorResponse
//	@Failure		500	{object}	errs.ErrorResponse
//	@Router			/api/v1/plans/{id} [get]
func (h *Handler) GetPlan(c *gin.Context) {
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}

	plan, err := h.service.GetPlan(c.Request.Context(), id)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	h.respondPlan(c, plan)
}

// CreatePlan creates a backup plan
//
//	@Summary		Create backup plan
//	@Description	Send the source block named by source_kind (postgres, mysql, sqlite or files) and no other. Postgres and MySQL need a password. Admins only.
//	@Tags			plans
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		PlanRequest	true	"Plan"
//	@Success		200		{object}	PlanDataResponse
//	@Failure		400		{object}	errs.ValidationErrorResponse
//	@Failure		401		{object}	errs.ErrorResponse
//	@Failure		403		{object}	errs.ErrorResponse
//	@Failure		500		{object}	errs.ErrorResponse
//	@Router			/api/v1/plans [post]
func (h *Handler) CreatePlan(c *gin.Context) {
	var req PlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errs.RespondWithValidationError(c, err)
		return
	}

	plan, err := h.service.CreatePlan(c.Request.Context(), &req)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	h.respondPlan(c, plan)
}

// UpdatePlan updates a backup plan
//
//	@Summary		Update backup plan
//	@Description	A password left blank keeps the stored one, unless the source kind changes. Admins only.
//	@Tags			plans
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int			true	"Plan ID"
//	@Param			request	body		PlanRequest	true	"Plan"
//	@Success		200		{object}	PlanDataResponse
//	@Failure		400		{object}	errs.ValidationErrorResponse
//	@Failure		401		{object}	errs.ErrorResponse
//	@Failure		403		{object}	errs.ErrorResponse
//	@Failure		404		{object}	errs.ErrorResponse
//	@Failure		500		{object}	errs.ErrorResponse
//	@Router			/api/v1/plans/{id} [put]
func (h *Handler) UpdatePlan(c *gin.Context) {
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}

	var req PlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errs.RespondWithValidationError(c, err)
		return
	}

	plan, err := h.service.UpdatePlan(c.Request.Context(), id, &req)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	h.respondPlan(c, plan)
}

// DeletePlan deletes a backup plan
//
//	@Summary		Delete backup plan
//	@Description	Removes the plan. Its storages and the archives in them are not touched. Admins only.
//	@Tags			plans
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"Plan ID"
//	@Success		200	{object}	internal.MessageResponse
//	@Failure		400	{object}	errs.ErrorResponse
//	@Failure		401	{object}	errs.ErrorResponse
//	@Failure		403	{object}	errs.ErrorResponse
//	@Failure		404	{object}	errs.ErrorResponse
//	@Failure		500	{object}	errs.ErrorResponse
//	@Router			/api/v1/plans/{id} [delete]
func (h *Handler) DeletePlan(c *gin.Context) {
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}

	if err := h.service.DeletePlan(c.Request.Context(), id); err != nil {
		errs.RespondWithError(c, err)
		return
	}
	internal.RespondMessage(c, "Plan deleted successfully")
}

// TestDraft tests the source of a plan form that has not been saved
//
//	@Summary		Test an unsaved source
//	@Description	Connects to the database (or opens the files) and looks for the dump program on this server. A failed test is a 200 whose step says what failed. On an edit form, pass plan_id so a blank password is taken from that plan. Admins only.
//	@Tags			plans
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			plan_id	query		int				false	"The plan being edited, whose password a blank form uses"
//	@Param			request	body		SourceRequest	true	"Source"
//	@Success		200		{object}	SourceTestDataResponse
//	@Failure		400		{object}	errs.ValidationErrorResponse
//	@Failure		401		{object}	errs.ErrorResponse
//	@Failure		403		{object}	errs.ErrorResponse
//	@Failure		404		{object}	errs.ErrorResponse
//	@Failure		500		{object}	errs.ErrorResponse
//	@Router			/api/v1/plans/test [post]
func (h *Handler) TestDraft(c *gin.Context) {
	var planID int64
	if raw := c.Query("plan_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			errs.RespondWithError(c, errs.FieldError("plan_id", "invalid"))
			return
		}
		planID = id
	}

	var req SourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errs.RespondWithValidationError(c, err)
		return
	}

	result, err := h.service.TestDraft(c.Request.Context(), &req, planID)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, SourceTestDataResponse{Data: sourceTestResponse(result)})
}

// TestPlan tests the source of a saved plan
//
//	@Summary		Test a plan's source
//	@Description	Connects to the database (or opens the files) and looks for the dump program on this server. A failed test is a 200 whose step says what failed. Admins only.
//	@Tags			plans
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"Plan ID"
//	@Success		200	{object}	SourceTestDataResponse
//	@Failure		400	{object}	errs.ErrorResponse
//	@Failure		401	{object}	errs.ErrorResponse
//	@Failure		403	{object}	errs.ErrorResponse
//	@Failure		404	{object}	errs.ErrorResponse
//	@Failure		500	{object}	errs.ErrorResponse
//	@Router			/api/v1/plans/{id}/test [post]
func (h *Handler) TestPlan(c *gin.Context) {
	id, ok := middleware.PathID(c, "id")
	if !ok {
		return
	}

	result, err := h.service.TestPlan(c.Request.Context(), id)
	if err != nil {
		errs.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, SourceTestDataResponse{Data: sourceTestResponse(result)})
}
