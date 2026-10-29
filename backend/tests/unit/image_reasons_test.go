package unit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"app/internal/images"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// reasonConstants is every string constant named Reason* in a package's source: the reasons it can
// record on an image.
func reasonConstants(t *testing.T, dir string) map[string]string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	require.NoError(t, err)
	found := map[string]string{}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		require.NoError(t, err)
		ast.Inspect(parsed, func(n ast.Node) bool {
			spec, ok := n.(*ast.ValueSpec)
			if !ok {
				return true
			}
			for i, name := range spec.Names {
				if !strings.HasPrefix(name.Name, "Reason") || i >= len(spec.Values) {
					continue
				}
				if lit, ok := spec.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					value, err := strconv.Unquote(lit.Value)
					require.NoError(t, err)
					found[filepath.Base(dir)+"."+name.Name] = value
				}
			}
			return true
		})
	}
	return found
}

// The admin panel words each reason by its code. A reason the optimizer or the checks gain fails
// here until images.ReasonCodes, and so the API's enum and both translations, have it.
func TestImageReasons_EveryReasonTheOptimizerRecordsHasACode(t *testing.T) {
	constants := map[string]string{}
	for _, dir := range []string{"../../internal/optimizer", "../../internal/verify", "../../internal/images"} {
		for name, value := range reasonConstants(t, dir) {
			constants[name] = value
		}
	}
	require.GreaterOrEqual(t, len(constants), len(images.ReasonCodes), "the scan found the reason constants")

	for name, value := range constants {
		reason, _ := images.ParseReason(value)
		assert.NotEmpty(t, reason.Code, "%s = %q has no code in images.ReasonCodes", name, value)
	}
}

func TestImageReasons_TheAPIEnumListsEveryCode(t *testing.T) {
	field, ok := reflect.TypeOf(images.ReasonResponse{}).FieldByName("Code")
	require.True(t, ok)

	assert.ElementsMatch(t, images.ReasonCodes, strings.Split(field.Tag.Get("enums"), ","))
}

func TestParseReason(t *testing.T) {
	tests := []struct {
		stored   string
		reason   images.Reason
		lossless *images.Reason
	}{
		{"apng", images.Reason{Code: "apng"}, nil},
		{"extra_data: bytes after the end", images.Reason{Code: "extra_data", Detail: "bytes after the end"}, nil},
		{"tool_failed: oxipng: exit status 1: bad", images.Reason{Code: "tool_failed", Detail: "oxipng: exit status 1: bad"}, nil},
		{images.ReasonDied, images.Reason{Code: "interrupted", Detail: "the process died while optimizing it"}, nil},
		{
			"ssim_below_threshold: 0.7100, below 0.8500, at 128,64; lossless: no_saving: 1000 bytes to 990",
			images.Reason{Code: "ssim_below_threshold", Detail: "0.7100, below 0.8500, at 128,64"},
			&images.Reason{Code: "no_saving", Detail: "1000 bytes to 990"},
		},
		{
			"metadata_changed: iCCP; lossless: tool_failed: oxipng: signal: killed",
			images.Reason{Code: "metadata_changed", Detail: "iCCP"},
			&images.Reason{Code: "tool_failed", Detail: "oxipng: signal: killed"},
		},
		{"frames_changed: ; lossless: tool_missing", images.Reason{Code: "frames_changed"}, &images.Reason{Code: "tool_missing"}},
		{"a reason from another version: 12", images.Reason{Detail: "a reason from another version: 12"}, nil},
	}
	for _, tc := range tests {
		reason, lossless := images.ParseReason(tc.stored)

		assert.Equal(t, tc.reason, reason, tc.stored)
		assert.Equal(t, tc.lossless, lossless, tc.stored)
	}
}
