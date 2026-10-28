package integration

import (
	"runtime"
	"testing"

	"app/internal/factory"
	"app/tests/testkit"
)

func TestSettingsAPI_GetAndUpdate(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User()
	auto := max(1, runtime.NumCPU()-1)

	app.As(user).GET("/api/v1/settings").
		AssertOK().
		AssertJSONPath("data.workers", 0).
		AssertJSONPath("data.effective_workers", auto).
		AssertJSONPath("data.scan_workers", 2).
		AssertJSONPath("data.nice", true)

	app.As(user).PUT("/api/v1/settings", map[string]any{"workers": 6, "scan_workers": 3, "nice": false}).
		AssertOK().
		AssertJSONPath("data.workers", 6).
		AssertJSONPath("data.effective_workers", 6)
	app.As(user).GET("/api/v1/settings").
		AssertOK().
		AssertJSONPath("data.scan_workers", 3).
		AssertJSONPath("data.nice", false)

	app.As(user).PUT("/api/v1/settings", map[string]any{"workers": 0, "scan_workers": 1, "nice": true}).
		AssertOK().
		AssertJSONPath("data.workers", 0).
		AssertJSONPath("data.effective_workers", auto)
	app.AssertDatabaseCount("settings", 1)
}

func TestSettingsAPI_Validation(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User()

	app.As(user).PUT("/api/v1/settings", `{}`).AssertInvalid("workers", "scan_workers", "nice")
	app.As(user).PUT("/api/v1/settings", map[string]any{"workers": 65, "scan_workers": 2, "nice": true}).AssertInvalid("workers")
	app.As(user).PUT("/api/v1/settings", map[string]any{"workers": -1, "scan_workers": 2, "nice": true}).AssertInvalid("workers")
	app.As(user).PUT("/api/v1/settings", map[string]any{"workers": 0, "scan_workers": 0, "nice": true}).AssertInvalid("scan_workers")
	app.As(user).PUT("/api/v1/settings", map[string]any{"workers": 0, "scan_workers": 17, "nice": true}).AssertInvalid("scan_workers")
	app.AssertDatabaseHas("settings", testkit.Row{"workers": 0, "scan_workers": 2, "nice": true})
}

func TestSettingsAPI_EverySignedInUser(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User(factory.Unverified())
	body := map[string]any{"workers": 1, "scan_workers": 1, "nice": true}

	app.GET("/api/v1/settings").AssertUnauthorized()
	app.GET("/api/v1/tools").AssertUnauthorized()
	app.PUT("/api/v1/settings", body).AssertUnauthorized()
	app.As(user).GET("/api/v1/settings").AssertOK()
	app.As(user).PUT("/api/v1/settings", body).AssertOK()
	app.As(user).GET("/api/v1/tools").AssertOK()
}

func TestToolsAPI(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User()
	app.Tools.Remove("gifsicle")

	app.As(user).GET("/api/v1/tools").
		AssertOK().
		AssertJSONCount("data.programs", 5).
		AssertJSONPath("data.programs.0.name", "pngquant").
		AssertJSONPath("data.programs.0.installed", true).
		AssertJSONPath("data.programs.0.path", "/usr/bin/pngquant").
		AssertJSONPath("data.programs.0.version", "3.0.3").
		AssertJSONPath("data.programs.4.name", "gifsicle").
		AssertJSONPath("data.programs.4.installed", false).
		AssertJSONPath("data.programs.4.apt", "gifsicle").
		AssertJSONPath("data.programs.4.brew", "gifsicle").
		AssertJSONPath("data.needs.3.need", "gif").
		AssertJSONPath("data.needs.3.satisfied", false).
		AssertJSONPath("data.needs.3.missing", []string{"gifsicle"})

	app.Tools.Set("gifsicle", "LCDF Gifsicle 1.95")
	app.As(user).GET("/api/v1/tools").AssertOK().AssertJSONPath("data.programs.4.installed", false)
	app.As(user).GET("/api/v1/tools?refresh=1").
		AssertOK().
		AssertJSONPath("data.programs.4.installed", true).
		AssertJSONPath("data.programs.4.version", "LCDF Gifsicle 1.95").
		AssertJSONPath("data.needs.3.satisfied", true)
}
