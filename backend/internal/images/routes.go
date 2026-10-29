package images

import "app/internal"

func RegisterRoutes(app *internal.App) {
	handler := NewHandler(NewImageService(app.Queries))

	folders := app.Api.Group("/folders")
	folders.Use(app.AuthMiddleware)
	{
		folders.GET("/:id/images", handler.ListImages)
		folders.POST("/:id/images/retry-failed", handler.RetryFailedImages)
	}
	images := app.Api.Group("/images")
	images.Use(app.AuthMiddleware)
	{
		images.POST("/:id/retry", handler.RetryImage)
	}
	app.Api.GET("/daily-savings", app.AuthMiddleware, handler.DailySavings)
}
