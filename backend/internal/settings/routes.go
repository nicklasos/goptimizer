package settings

import "app/internal"

// RegisterRoutes serves the settings page: the global settings and the image tools.
func RegisterRoutes(app *internal.App) {
	handler := NewHandler(NewSettingsService(app.Queries, app.ToolReport))

	authed := app.Api.Group("")
	authed.Use(app.AuthMiddleware)
	{
		authed.GET("/settings", handler.GetSettings)
		authed.PUT("/settings", handler.UpdateSettings)
		authed.GET("/tools", handler.GetTools)
	}
}
