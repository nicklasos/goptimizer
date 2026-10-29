package bench

import (
	"context"
	"database/sql"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"app/internal/db"
	"app/internal/images"
	"app/migrations"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
)

const runs = 7

const spread = `
UPDATE images SET
  status = 'r' || (abs(random()) % 1000),
  size = 10000 + abs(random()) % 2000000,
  updated_at = strftime('%Y-%m-%d %H:%M:%f', 'now', '-' || (abs(random()) % 2592000) || ' seconds') || '+00:00';
UPDATE images SET status = CASE
    WHEN CAST(substr(status, 2) AS INTEGER) < 700 THEN 'optimized'
    WHEN CAST(substr(status, 2) AS INTEGER) < 800 THEN 'kept'
    WHEN CAST(substr(status, 2) AS INTEGER) < 850 THEN 'skipped'
    WHEN CAST(substr(status, 2) AS INTEGER) = 850 THEN 'failed'
    WHEN CAST(substr(status, 2) AS INTEGER) < 990 THEN 'pending'
    ELSE 'gone' END;
UPDATE images SET original_size = size, optimized_size = size * (40 + abs(random()) % 55) / 100, mode = 'lossy'
  WHERE status = 'optimized';
UPDATE images SET size = optimized_size WHERE status = 'optimized';
UPDATE images SET reason = 'no_saving: 1000 bytes to 990' WHERE status = 'kept';
UPDATE images SET reason = 'apng' WHERE status = 'skipped';
UPDATE images SET reason = 'tool_failed: pngquant: signal: killed', attempts = 3 WHERE status = 'failed';
`

type scenario struct {
	name  string
	sql   string
	query images.Query
	deep  *images.Cursor
}

func TestImagesListOnABigFolder(t *testing.T) {
	source := os.Getenv("GOPTIMIZER_BENCH_DB")
	if source == "" {
		t.Skip("set GOPTIMIZER_BENCH_DB to a database cmd/scanbench kept")
	}
	ctx := context.Background()
	conn := openCopy(t, source)
	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())
	require.NoError(t, goose.SetDialect(migrations.Dialect))
	require.NoError(t, goose.UpTo(conn, ".", 6))
	_, err := conn.ExecContext(ctx, spread)
	require.NoError(t, err)

	var folderID, total int64
	require.NoError(t, conn.QueryRowContext(ctx, "SELECT folder_id, COUNT(*) FROM images GROUP BY folder_id").Scan(&folderID, &total))
	fmt.Printf("%d images in folder %d\n", total, folderID)
	printCounts(t, conn)

	existing := []string{"countImagesByStatus", "countImagesOfFolderByStatus", "oldestClaimableImage", "listImagesInDir", "markImagesGoneUnder", "countOptimizingByFolder", "requeueDueImages", "retryFailedImages"}
	fmt.Println("\n## Plans of the queries that ran before, at migration 006")
	plans(t, conn, existing)
	require.NoError(t, goose.Up(conn, "."))
	fmt.Println("\n## The same at migration 007")
	plans(t, conn, existing)

	middle := time.Now().UTC().Add(-15 * 24 * time.Hour)
	scenarios := []scenario{
		{"every status, latest change first", "listImagesByUpdated", images.Query{Sort: images.SortUpdated}, &images.Cursor{UpdatedAt: middle, ID: 1 << 62}},
		{"optimized, latest change first", "listImagesOfStatusByUpdated", images.Query{Status: images.StatusOptimized, Sort: images.SortUpdated}, &images.Cursor{UpdatedAt: middle, ID: 1 << 62}},
		{"failed, latest change first", "listImagesOfStatusByUpdated", images.Query{Status: images.StatusFailed, Sort: images.SortUpdated}, &images.Cursor{UpdatedAt: middle, ID: 1 << 62}},
		{"optimized, most saved first", "listOptimizedImagesBySaved", images.Query{Status: images.StatusOptimized, Sort: images.SortSaved}, &images.Cursor{Saved: 400_000, ID: 1 << 62}},
		{"search that matches 1 in 3, every status", "searchImages", images.Query{Sort: images.SortPath, Search: ".PNG"}, &images.Cursor{Dir: "t05", Name: ""}},
		{"search that matches 1 in 100, every status", "searchImages", images.Query{Sort: images.SortPath, Search: "l050/"}, &images.Cursor{Dir: "t05", Name: ""}},
		{"search that matches nothing, every status", "searchImages", images.Query{Sort: images.SortPath, Search: "no-such-file"}, nil},
		{"search that matches nothing, optimized", "searchImagesOfStatus", images.Query{Status: images.StatusOptimized, Sort: images.SortPath, Search: "no-such-file"}, nil},
		{"search that matches everything, failed", "searchImagesOfStatus", images.Query{Status: images.StatusFailed, Sort: images.SortPath, Search: "/"}, &images.Cursor{Dir: "t05", Name: ""}},
		{"search that matches everything, optimizing (none)", "searchImagesOfStatus", images.Query{Status: images.StatusOptimizing, Sort: images.SortPath, Search: "/"}, nil},
	}
	service := images.NewImageService(db.New(conn))
	fmt.Println("\n## The folder page's queries at migration 007")
	for _, s := range scenarios {
		plans(t, conn, []string{s.sql})
		for _, page := range []struct {
			name  string
			after *images.Cursor
		}{{"first page", nil}, {"deep page", s.deep}} {
			if page.name == "deep page" && page.after == nil {
				continue
			}
			q := s.query
			q.Limit, q.After = 50, page.after
			var times []time.Duration
			var got int
			for range runs {
				start := time.Now()
				result, err := service.List(ctx, folderID, q)
				times = append(times, time.Since(start))
				require.NoError(t, err)
				got = len(result.Images)
			}
			slices.Sort(times)
			fmt.Printf("| %s | %s | %d | %s | %s to %s |\n", s.name, page.name, got,
				times[len(times)/2].Round(10*time.Microsecond), times[0].Round(10*time.Microsecond), times[len(times)-1].Round(10*time.Microsecond))
		}
	}
}

func openCopy(t *testing.T, source string) *sql.DB {
	t.Helper()
	target := filepath.Join(t.TempDir(), "bench.db")
	in, err := os.Open(source)
	require.NoError(t, err)
	defer in.Close()
	out, err := os.Create(target)
	require.NoError(t, err)
	_, err = io.Copy(out, in)
	require.NoError(t, err)
	require.NoError(t, out.Close())
	conn, err := db.Open(target)
	require.NoError(t, err)
	conn.SetMaxOpenConns(1)
	t.Cleanup(func() { conn.Close() })
	return conn
}

func printCounts(t *testing.T, conn *sql.DB) {
	t.Helper()
	rows, err := conn.Query("SELECT status, COUNT(*) FROM images GROUP BY status ORDER BY 2 DESC")
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var status string
		var n int64
		require.NoError(t, rows.Scan(&status, &n))
		fmt.Printf("  %s %d\n", status, n)
	}
}

func plans(t *testing.T, conn *sql.DB, names []string) {
	t.Helper()
	queries := generatedQueries(t)
	for _, name := range names {
		text, ok := queries[name]
		require.True(t, ok, "no generated query %s", name)
		args := make([]any, strings.Count(text, "?"))
		rows, err := conn.Query("EXPLAIN QUERY PLAN "+text, args...)
		require.NoError(t, err, name)
		var steps []string
		for rows.Next() {
			var id, parent, unused int
			var detail string
			require.NoError(t, rows.Scan(&id, &parent, &unused, &detail))
			steps = append(steps, detail)
		}
		rows.Close()
		fmt.Printf("- %s: %s\n", name, strings.Join(steps, "; "))
	}
}

func generatedQueries(t *testing.T) map[string]string {
	t.Helper()
	files, err := filepath.Glob("../../internal/db/*.sql.go")
	require.NoError(t, err)
	found := map[string]string{}
	for _, file := range files {
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		require.NoError(t, err)
		ast.Inspect(parsed, func(n ast.Node) bool {
			spec, ok := n.(*ast.ValueSpec)
			if !ok || len(spec.Values) != 1 {
				return true
			}
			if lit, ok := spec.Values[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
				text, err := strconv.Unquote(lit.Value)
				require.NoError(t, err)
				found[spec.Names[0].Name] = text
			}
			return true
		})
	}
	return found
}
