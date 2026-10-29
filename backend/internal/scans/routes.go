package scans

import "app/internal"

func RegisterRoutes(app *internal.App) {
	handler := NewHandler(NewScanService(app.Queries, app.WakeScans))

	folders := app.Api.Group("/folders")
	folders.Use(app.AuthMiddleware)
	{
		folders.POST("/:id/scans", handler.StartScan)
		folders.GET("/:id/scans", handler.ListScans)
	}
}
