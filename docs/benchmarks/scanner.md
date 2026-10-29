# Scanner benchmark

**Verdict.** On a folder of 1,000,000 files in 10,111 directories, a quick scan that finds nothing new
takes **18 ms** when the directories are in the kernel's cache and about **0.4 s** right after a full
scan has pushed them out. Neither writes a row. A full scan with no change takes **12.6 s** and writes
nothing. The first full scan of the folder takes **22 s** and writes 920,221 rows. A quick scan after
100 files were added and 50 deleted takes **79 ms** and writes exactly those 150 image rows and the 15
directories they were in.

Every number is the median of 7 interleaved rounds, measured on macOS. A Linux server was not measured.

## What runs

`backend/cmd/scanbench` builds the tree, migrates a fresh SQLite database, adds one folder and calls
the scanner directly, the way the scan worker does:

```bash
cd backend && go build -o /tmp/scanbench ./cmd/scanbench && /usr/bin/time -l /tmp/scanbench -runs 7
```

- **Tree.** 10 × 10 × 100 = 10,000 leaf directories, 10,111 with the two levels above them and the
  root. Each leaf holds 100 empty files: 30 each of `.png`, `.jpg` and `.gif`, and 10 `.txt`. That is
  1,000,000 files and 900,000 images. The scanner never reads a file's content, so empty files cost
  what full ones would.
- **Phases.** (a) once on the fresh database, then 7 rounds of the rest in this order: (b) quick, no
  change; (b2) the same again at once; a 2.1 s wait; (b3) quick, no change; add 10 images to each of 10
  leaves and delete 10 from each of 5 others; a 2.1 s wait; (c) quick; (d) full, no change. The waits
  are longer than the scanner's 2 s settle rule, so no directory is stored with mtime 0 and every
  round does the same work.
- **Rows written** is SQLite's `total_changes()` before and after a phase. The benchmark's pool has a
  single connection, so that counts every insert, update and delete of the scan. The scanner uses one
  connection at a time in production too. The worker's own writes (the claim, the progress every 2 s,
  the finish) are not part of it.
- **CPU** is user plus system time of the process (`getrusage`) over the phase.
- Profiles and traces come from separate runs that are not in the table (`-profile <phase>
  -cpuprofile <file>` or `-trace <file>`, read with `go tool pprof` and `go tool trace -pprof=syscall`).

## Machine

Apple M2 (4 performance and 4 efficiency cores), 16 GB, macOS 26.6.2, APFS on the internal SSD, Go
1.25.4, modernc.org/sqlite (pure Go). The machine was busy and could not be cleared: load average 5.3
to 6.3 during the run, with Spotlight (`spotlightknowledged`, about 85% of a core), Steam and XProtect's
remediators (60 to 100% each) at times. The rounds are interleaved so every phase saw the same noise,
and the spread within each phase is small. The page cache was warm: the tree had just been written,
and dropping the cache (`purge`) needs root.

## Results

| Phase | Runs | Median | Range | CPU median | CPU / wall | Rows written | Directories seen / listed | Files new / changed / gone |
|---|---|---|---|---|---|---|---|---|
| (a) first full scan | 1 | 22.1 s | | 18.0 s | 0.81 | 920,221 | 10,111 / 10,111 | 900,000 / 0 / 0 |
| (b) quick, no change | 7 | 383 ms | 90 to 402 ms | 94 ms | 0.25 | 0 | 10,111 / 0 | 0 / 0 / 0 |
| (b2) quick, again at once | 7 | 18 ms | 17 to 19 ms | 22 ms | 1.22 | 0 | 10,111 / 0 | 0 / 0 / 0 |
| (b3) quick, after 2.1 s idle | 7 | 57 ms | 53 to 63 ms | 65 ms | 1.15 | 0 | 10,111 / 0 | 0 / 0 / 0 |
| (c) quick, +100 / −50 images | 7 | 79 ms | 76 to 88 ms | 87 ms | 1.09 | 165 | 10,111 / 15 | 100 / 0 / 50 |
| (d) full, no change | 7 | 12.56 s | 12.52 to 12.79 s | 8.5 s | 0.68 | 0 | 10,111 / 10,111 | 0 / 0 / 0 |

The (b) runs were 90, 379, 402, 383, 391, 391 and 379 ms: the first follows (a), the others follow a (d).

The folder list's grouped count of images by status (`CountImagesByStatus`, what `GET /folders` runs
for `image_counts`) took 81 ms on these 900,000 rows, range 80 to 211 ms over 7 runs.

The whole run, generation included, peaked at 43 MB resident (`/usr/bin/time -l`).

## Rows written

The counts match what the rules predict, so no phase did work it should not have:

- **(a)** 900,000 image inserts plus 20,221 directory writes. The root is written once. Each of the
  other 10,110 directories is written twice: with mtime 0 when its parent lists it, then with its real
  mtime when it is listed itself. That first row is what lets a stopped scan converge (see PLAN.md).
- **(b), (b2), (b3), (d)** 0 rows in every run.
- **(c)** 165 rows in every run: 100 inserts, 50 updates to `gone`, and the new mtime of the 15
  directories that changed.
- At the end the folder held 900,350 pending and 350 gone images: 900,000 + 7 × 100 − 7 × 50, and
  7 × 50.

## Why each number is what it is

**(b2), 18 ms, is one core doing 10,111 `lstat` calls.** CPU per wall time is above 1 (the garbage
collector runs beside it), so the scan is CPU-bound, at about 1.8 µs per directory including reading
the 10,111 `dirs` rows. That is the estimate PLAN.md made: one `stat` per directory and no file
touched. Running the directories on several goroutines is the way to make it faster; at 18 ms there
is no reason to.

**(b), 383 ms, is the same `lstat` calls finding the directories evicted from the kernel's vnode
cache.** An execution trace of one (b) run puts 214 of its 236 ms inside `syscall.Lstat`, against
0.1 ms waiting for the Go scheduler, so the busy machine is not what slows it down. CPU per wall time
is 0.25: the calls mostly wait in the kernel. The cause is inferred, not traced inside the kernel,
which needs root:

- macOS caps the vnode cache at `kern.maxvnodes` = 250,981, and it was full. The tree has 1,010,111
  entries, so a full scan cycles the whole cache and the directories it lstat'ed first are gone by the
  end.
- (b2), the same scan immediately after, is 21 times faster and CPU-bound: the directories are cached
  again.

Linux has no fixed cap like this: the dentry and inode caches grow with free memory, so a server with
memory to spare should see (b2), not (b). That is an expectation, not a measurement.

**(c), 79 ms, is (b3) plus the change.** (c) is the only phase that starts after an idle wait, so (b3)
repeats the wait with nothing changed. The wait alone costs about 40 ms over (b2). About half of that
shows in a trace of (c): the first two SQLite reads after the wait spent 23 ms in `fcntl`, SQLite's
file lock, which is fast everywhere else. The other half was not traced to a cause. The remaining 22 ms over (b3) is listing the 15 changed directories, `lstat` on the
roughly 1,400 files in them and one transaction of 165 writes.

**(d), 12.6 s, is system calls.** A trace puts 12.8 of 13.0 s inside syscalls: `lstat` on the
1,000,000 files 4.3 s, `readdir` 3.8 s, SQLite's `fcntl` locks 2.3 s and `pread` 1.6 s, for the
10,111 queries that read each directory's rows. The CPU profile has Go code itself under 5%. CPU per
wall time is 0.68: `readdir` in particular waits (0.64 s of CPU for 3.8 s in the call). The scan is
one goroutine. Walking directories in parallel would attack the largest part; reading a folder's rows
in fewer, larger queries would attack SQLite's share. A full scan runs once a day by default.

**(a), 22 s, is SQLite's write path.** In a trace, `pwrite` to the WAL and the database file is 51% of
the time in syscalls (10.3 s), then `lstat` 3.6 s and `readdir` 2.5 s. In the CPU profile the insert
path is 37% of samples, and half of that is SQLite parsing the same `INSERT` again for every row
(`sqlite3RunParser`, 19%). A statement prepared once per batch, or one multi-row insert per batch,
would cut the parsing. This is a one-time cost per folder, and the optimizer can start on its first
images after the first batch of 1,000 commits.

**Not explained.** The first round's (b) took 90 ms, the six after it about 385 ms. The first follows
(a) and the others follow (d), and both touch every file. A trace of a (b) after (a) has the same
shape, only smaller: 64 ms in `lstat` (against 214 ms after (d)) and 24 ms in `pread` reading the
`dirs` rows (a) had just written. So (a) leaves more directories in the vnode cache than (d) does. Why
was not established.

## Caveats

- **APFS, not ext4.** Syscall costs differ, and so does the vnode cache (above). The numbers are for
  this Mac, a development machine, not for a server.
- **Warm page cache.** Every phase read metadata the kernel already had in memory: 0 block input
  operations for the whole run. A quick scan on a server whose cache was cold would read the
  directories' inodes from disk, and a full scan the files' too.
- **Empty files.** Sizes are 0 and the content is never read, so this matches real images for the
  scanner's work. It does not say anything about the optimizer.
- **A loaded machine.** Load 5 to 6 on 8 cores, from processes that could not be stopped. The rounds
  are interleaved and their spread is a few percent, but absolute numbers on an idle machine would be
  lower.
- **The scanner alone.** The worker adds its claim, a progress update every 2 s and its finish, a few
  rows per scan.

The synthetic tree is deleted at the end of every run unless `-keep` is given.
