package storages

import (
	"app/internal"
	"app/internal/middleware"
)

func RegisterRoutes(app *internal.App) {
	service := NewStorageService(app.Queries, app.Secrets)
	handler := NewHandler(service)

	storages := app.Api.Group("/storages")
	storages.Use(app.AuthMiddleware, middleware.RequireRole(middleware.RoleAdmin))
	{
		storages.GET("", handler.ListStorages)
		storages.POST("", handler.CreateStorage)
		storages.POST("/test", handler.TestDraft)
		storages.GET("/:id", handler.GetStorage)
		storages.PUT("/:id", handler.UpdateStorage)
		storages.DELETE("/:id", handler.DeleteStorage)
		storages.POST("/:id/test", handler.TestStorage)
		storages.GET("/:id/objects", handler.ListObjects)
	}
}
