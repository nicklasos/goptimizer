package settings

import (
	"app/internal"
	"app/internal/middleware"
)

// RegisterRoutes serves the settings page: the global settings and the image tools.
func RegisterRoutes(app *internal.App) {
	handler := NewHandler(NewSettingsService(app.Queries, app.Cache, app.Tools))

	admin := app.Api.Group("")
	admin.Use(app.AuthMiddleware, middleware.RequireRole(middleware.RoleAdmin))
	{
		admin.GET("/settings", handler.GetSettings)
		admin.PUT("/settings", handler.UpdateSettings)
		admin.GET("/tools", handler.GetTools)
	}
}
