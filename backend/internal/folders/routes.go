package folders

import (
	"app/internal"
	"app/internal/middleware"
)

func RegisterRoutes(app *internal.App) {
	handler := NewHandler(NewFolderService(app.Queries))

	folders := app.Api.Group("/folders")
	folders.Use(app.AuthMiddleware, middleware.RequireRole(middleware.RoleAdmin))
	{
		folders.GET("", handler.ListFolders)
		folders.POST("", handler.CreateFolder)
		folders.GET("/:id", handler.GetFolder)
		folders.PUT("/:id", handler.UpdateFolder)
		folders.PUT("/:id/enabled", handler.SetFolderEnabled)
		folders.DELETE("/:id", handler.DeleteFolder)
	}
}
