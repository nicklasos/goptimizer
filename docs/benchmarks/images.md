# Folder page benchmark

**Verdict.** On a folder of 900,000 images, every page of the folder page's image list takes about
**0.2 ms** in the service, the first page and one deep in the list alike: every status or one, the
latest change first, or the optimized ones by bytes saved. A search by path costs up to one pass over
the path index: **0.32 s** when nothing matches, a few milliseconds when matches are common. The three
indexes that make the pages fast cost the scanner's first full scan of the folder **4 s of CPU**
(26.5 s against 22.6 s, median of 3 interleaved runs each), and 83 MB of disk.

The list numbers are the median of 7 runs of each query, on macOS. A Linux server was not measured.

## What runs

`backend/tests/bench/images_list_test.go`, skipped unless `GOPTIMIZER_BENCH_DB` names a database that
`cmd/scanbench` kept:

```bash
cd backend
go build -o /tmp/scanbench ./cmd/scanbench && /tmp/scanbench -runs 0 -keep   # prints where it kept bench.db
GOPTIMIZER_BENCH_DB=/path/to/bench.db go test ./tests/bench -run TestImagesListOnABigFolder -count=1 -v
```

- **Data.** The scanbench folder: 900,000 images in 10,000 leaf directories (`t05/m03/l042/f017.png`),
  all pending. The test works on a copy, migrated to 006. It draws a status for each row at random, 70%
  optimized, 14% pending, 10% kept, 5% skipped, 1% gone and 0.1% failed (831 rows), with sizes, a
  saving of 6 to 60% for each optimized image and update times over the last 30 days. It prints the
  plans of the queries that ran before at 006, migrates to 007 and prints them again, then measures.
- **What is timed.** `ImageService.List` with a page of 50, as the handler calls it: the folder lookup
  and the page query, which fetches 51 rows to know whether there is a next page. Not the HTTP layer
  or the JSON.
- **Plans** are SQLite's `EXPLAIN QUERY PLAN` of the exact text sqlc generated, read from
  `internal/db/*.sql.go`, through the app's driver (modernc.org/sqlite).

## Machine

Apple M2 (4 performance and 4 efficiency cores), 16 GB, macOS 26.6.2, Go 1.25.4, modernc.org/sqlite
1.59.0. The machine was busy: load average 3 to 6 during the list runs, from processes that could not
be stopped. The database was in the page cache.

## Results

| Request | Plan | First page | Deep page |
|---|---|---|---|
| Every status, latest change first | `idx_images_updated (folder_id=? AND updated_at<?)` | 0.24 ms | 0.23 ms |
| Optimized, latest change first | `idx_images_status_updated (folder_id=? AND status=? AND updated_at<?)` | 0.23 ms | 0.23 ms |
| Failed (831 rows), latest change first | the same | 0.24 ms | 0.22 ms |
| Optimized, most bytes saved first | `idx_images_saved (folder_id=? AND <expr><?)` | 0.22 ms | 0.21 ms |
| Search matching 1 path in 3 | `idx_images_path (folder_id=? AND (dir,name)>(?,?))` | 0.27 ms | 0.27 ms |
| Search matching 1 path in 100 | the same | 1.8 ms | 1.9 ms |
| Search matching nothing | the same | 322 ms | |
| Search matching nothing, optimized only | the same | 323 ms | |
| Search matching every path, failed only | the same | 46 ms | 60 ms |
| Search matching every path, optimizing only (no rows) | the same | 700 ms | |

The range over 7 runs was within 10% of the median for every row except the first run of the first
query, 1.28 ms, which read pages the copy had not touched yet. A last run of the same test on the
final code, at load 7 to 9 while the admin panel's tests ran, gave the same browse times and 0.37 s
and 0.80 s for the two slow searches. That code lowers the search text in SQL instead of in Go;
interleaving the two on the same copy, 7 runs each, gave 318 ms and 321 ms, so the gap is the load.

The queries that ran before 007 keep their plans, except two that move to the new index on
`(folder_id, status, updated_at)`, which serves them as well as `idx_images_claim` did: the counts by
status (`COUNT(*) GROUP BY folder_id, status`, covering either way) and a retry of a folder's failed
images (`folder_id=? AND status=?`). The claim still seeks `idx_images_claim` on all four of its
columns.

## Why each number is what it is

**0.2 ms is two queries and 51 rows.** Each browse page is the folder lookup, one seek into its index
and 51 steps along it, each reading its table row. The deep page costs what the first does because
the cursor is the seek key: nothing before it is read. With OFFSET, page 10,000 of the optimized
images would step over 500,000 rows first.

**A search reads the index, not the table.** `idx_images_path` holds `dir` and `name`, so SQLite tests
`instr(lower(dir || '/' || name), lower(?))` on the index entry and reads a row only for a path that matches.
When nothing matches it walks all 900,000 entries on one core: 322 ms, about 360 ns an entry. Measured
on the same copy, the case folding is about 75 ms of it (`instr` alone takes 200 ms, `LIKE` 245 ms),
and the sqlite3 command-line tool, native C, walks the same entries with `instr` alone in 107 ms. So the
limit is per-entry work in SQLite's bytecode engine, which modernc runs as Go, not the disk. A search
whose matches are common fills its page early and costs what a browse page does.

**700 to 800 ms is the worst case, and it is a corner.** A search that matches every path, in a status almost
no image has, reads the row of every entry to check its status: 900,000 row reads for nothing. The
failed images are 0.1% of this folder, and the same search in them fills a page after about 50,000
entries, 46 ms. The folder page shows the count of each status, so this takes a search for `/` or
`png` in an empty tab.

**Not done.** A search that stays fast for any text needs an index of substrings: SQLite's FTS5 with
the trigram tokenizer. It would be a second copy of every path, kept in step on every insert the
scanner makes, for a search an admin runs by hand. A third of a second on a million images does not
justify that yet.

## What the indexes cost

The scanner's first full scan inserts every image as pending, so it pays for the two new indexes that
hold every row; `idx_images_saved` holds only optimized images. Phase (a) of `cmd/scanbench -runs 0`,
built before and after migration 007 and run alternately, 3 times each (scanbench prints the phase,
then stops on a known bug in its report for zero rounds, which does not touch the numbers):

| Migration | Wall | CPU | Rows written |
|---|---|---|---|
| 006 | 28.3 s (28.2 to 29.6) | 22.6 s (22.2 to 22.7) | 920,221 |
| 007 | 31.9 s (31.5 to 32.1) | 26.5 s (26.3 to 26.6) | 920,221 |

The machine was much busier than for the list runs, load 4 to 18, because the admin panel's test suite
ran beside it; the two sides alternated, so both saw it, and the 4 s gap is several times the spread
of either. The CPU is the scan's: 900,000 inserts that now write five B-trees instead of three, about
4 µs more each. Both new keys start with the update time, which the scanner writes as now, so each
insert lands at the end of its index. A first full scan runs once per folder.

After the first scan, each image changes status a few times in its life, and each change moves its
entry in both new indexes, and in `idx_images_saved` when it becomes optimized. Against the seconds the
tools take on one image, that is not measurable; it was not measured.

On disk, on the 900,000 images above (`dbstat`): `idx_images_status_updated` 41.6 MB,
`idx_images_updated` 33.8 MB and `idx_images_saved` 7.8 MB (630,000 optimized images), next to 153 MB
for the table and 67 MB for the two indexes it had. Most of an entry is `updated_at`, a timestamp
stored as 32 characters of text.

## Caveats

- **One folder of 900,000 images.** Every query starts with `folder_id`, so other folders add nothing
  to a page, but the counts by status that `GET /folders` runs read every folder.
- **Random statuses.** A real folder's failed images cluster in time and directory. The browse pages do
  not care; a search in a status does, through how soon its page fills.
- **APFS, a warm page cache and a busy machine.** As in [scanner.md](scanner.md).
