package unit

import (
	"context"
	"testing"

	"app/internal/tools"
	"app/tests/testkit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func needs(report tools.Report) map[tools.Need]tools.NeedStatus {
	byNeed := map[tools.Need]tools.NeedStatus{}
	for _, status := range report.Needs {
		byNeed[status.Need] = status
	}
	return byNeed
}

func program(t *testing.T, report tools.Report, name string) tools.ProgramStatus {
	t.Helper()
	for _, status := range report.Programs {
		if status.Name == name {
			return status
		}
	}
	t.Fatalf("no %s in the report", name)
	return tools.ProgramStatus{}
}

func TestTools_Check(t *testing.T) {
	ctx := context.Background()

	t.Run("everything installed covers every need", func(t *testing.T) {
		report := tools.Check(ctx, tools.NewFake(testkit.FakeToolVersions))

		require.Len(t, report.Programs, 5)
		require.Len(t, report.Needs, 4)
		for _, status := range report.Needs {
			assert.True(t, status.Satisfied, status.Need)
			assert.Empty(t, status.Missing, status.Need)
		}
		pngquant := program(t, report, "pngquant")
		assert.True(t, pngquant.Installed)
		assert.Equal(t, "/usr/bin/pngquant", pngquant.Path)
		assert.Equal(t, "3.0.3", pngquant.Version)
		assert.Equal(t, []tools.Need{tools.NeedPNGLossy}, pngquant.UsedFor)
		assert.Empty(t, pngquant.Alternatives)
		assert.Equal(t, "pngquant", pngquant.Apt)
		assert.Equal(t, "pngquant", pngquant.Brew)

		oxipng := program(t, report, "oxipng")
		assert.Equal(t, []tools.Need{tools.NeedPNGLossy, tools.NeedPNGLossless}, oxipng.UsedFor)
		assert.Equal(t, []string{"optipng"}, oxipng.Alternatives)
		assert.Empty(t, oxipng.Apt, "Ubuntu has no oxipng package")
		assert.Equal(t, []string{"oxipng"}, program(t, report, "optipng").Alternatives)
	})

	t.Run("optipng stands in for a missing oxipng", func(t *testing.T) {
		fake := tools.NewFake(testkit.FakeToolVersions)
		fake.Remove("oxipng")
		report := tools.Check(ctx, fake)

		assert.False(t, program(t, report, "oxipng").Installed)
		assert.Empty(t, program(t, report, "oxipng").Path)
		assert.True(t, needs(report)[tools.NeedPNGLossless].Satisfied)
		assert.True(t, needs(report)[tools.NeedPNGLossy].Satisfied)
	})

	t.Run("a missing step names its preferred program", func(t *testing.T) {
		fake := tools.NewFake(testkit.FakeToolVersions)
		fake.Remove("oxipng")
		fake.Remove("optipng")
		fake.Remove("pngquant")
		fake.Remove("gifsicle")
		byNeed := needs(tools.Check(ctx, fake))

		assert.Equal(t, tools.NeedStatus{Need: tools.NeedPNGLossy, Missing: []string{"pngquant", "oxipng"}}, byNeed[tools.NeedPNGLossy])
		assert.Equal(t, tools.NeedStatus{Need: tools.NeedPNGLossless, Missing: []string{"oxipng"}}, byNeed[tools.NeedPNGLossless])
		assert.Equal(t, tools.NeedStatus{Need: tools.NeedGIF, Missing: []string{"gifsicle"}}, byNeed[tools.NeedGIF])
		assert.True(t, byNeed[tools.NeedJPEG].Satisfied)
	})
}
