package plans

import (
	"app/internal"
	"app/internal/middleware"
)

func RegisterRoutes(app *internal.App) {
	service := NewPlanService(app.Queries, app.Tx, app.Secrets, app.Tools)
	handler := NewHandler(service)

	plans := app.Api.Group("/plans")
	plans.Use(app.AuthMiddleware, middleware.RequireRole(middleware.RoleAdmin))
	{
		plans.GET("", handler.ListPlans)
		plans.POST("", handler.CreatePlan)
		plans.POST("/test", handler.TestDraft)
		plans.GET("/:id", handler.GetPlan)
		plans.PUT("/:id", handler.UpdatePlan)
		plans.DELETE("/:id", handler.DeletePlan)
		plans.POST("/:id/test", handler.TestPlan)
		plans.PUT("/:id/enabled", handler.SetEnabled)
	}

	dashboard := app.Api.Group("/dashboard")
	dashboard.Use(app.AuthMiddleware, middleware.RequireRole(middleware.RoleAdmin))
	dashboard.GET("", handler.Dashboard)
}
