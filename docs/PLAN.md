# Goptimizer plan

Goptimizer is a web app that runs on a server and keeps the images of the projects on that server optimized. You add a folder in the browser. The service finds new and changed images in it on a schedule, shrinks each one with command-line tools such as `pngquant`, `oxipng`, `jpegoptim` and `gifsicle`, and replaces the file only when the result is smaller and passes every check. The dashboard shows each folder, its queue and how many bytes it saved.

It is built from [Gogobackup](https://github.com/nicklasos/gogobackup), which is [Gogokit](https://github.com/nicklasos/gogokit) already converted to SQLite, with an in-process scheduler, a claim-based worker, a tool finder and Ansible deploy. This file is the plan. Each phase is one pull request that ends with `make test` green.

## Decisions

| Question | Decision |
|---|---|
| Where results go | In place. No backup of originals. Safety comes from the checks in [Verification](#verification). |
| Default compression | Lossy, as TinyPNG does: palette quantization for PNG, a capped quality for JPEG. Each folder can switch to lossless. |
| Process user | root, so it can rewrite any project's files and give each file back its owner and mode. The API listens on `127.0.0.1` only, behind nginx. |
| Sources | Local folders only. `kind` is a registry, so SSH, S3 and GCS are new entries later. |
| WebP and AVIF copies | Not in v1. |
| SVG | Not in v1. SVG optimizers rewrite markup and can change how a file renders. |
| Users | Two kinds: admins and users (`users.is_admin`). Every user can add, edit, pause and delete any folder and change the settings. Admins also manage accounts and can make other admins. |
| Scan history | The latest 100 scans of each folder, and its latest successful full scan. |

## Phases

| # | Phase | Ends when |
|---|---|---|
| 1 | Base | Gogobackup copied in and renamed, the backup domain removed, `make test` green |
| 2 | Folders and settings | Folders can be added, edited, paused. The settings page lists every tool, installed or not, with how to install it |
| 3 | Scanner | Quick and full scans fill the image index. A scan of 1,000,000 synthetic files is measured |
| 4 | Optimizer | Pending images are optimized, verified and swapped in by a worker pool shared fairly by all folders |
| 5 | Dashboard | Folder cards with stats and a 30-day chart, a folder page with scans and images filtered by status, README screenshots |
| 6 | Deploy | `make provision`, `make install-remote` and `make deploy-remote` install the tools and the app and release it |

All six phases are done. Phase 1 removes storages, plans, runs, restores, archives, self-backup and the secret box from both halves before anything is added, so phases 2 to 5 build on a smaller base.

## Data model

**Folder.** `id`, `name` (unique, case-insensitive), `kind` (`local`), `path` (absolute, must exist and be a readable directory, stored with symlinks resolved), `enabled`, `scan_interval_minutes` (quick scan, default 60, 5 to 10080), `full_scan_interval_hours` (default 24, 1 to 720), `profile` (JSON, below), `created_at`, `updated_at`. Two folders cannot overlap: a path inside another folder's path, or around it, is refused. `/` and the system directories (`/proc`, `/sys`, `/dev`, `/boot`, `/etc`, `/bin`, `/sbin`, `/usr`, `/lib*`, `/var/lib`) and everything inside them are refused too.

**Profile.** `mode` (`lossy` or `lossless`), `jpeg_max_quality` (82), `png_quality_min` and `png_quality_max` (65 and 80, pngquant's `--quality 65-80`), `gif_lossy` (30), `min_saving_percent` (3: a smaller saving is not worth a rewrite), `min_ssim` (lossy only, 0.85 on the worst 64-pixel tile, calibrated in phase 4: [docs/benchmarks/ssim.md](benchmarks/ssim.md)), `min_age_minutes` (5: newer files may still be uploading), `excludes` (glob patterns with `**`, such as `cache/**`). A field left out of a request or of a stored profile takes its default. A folder saved before phase 4 keeps the 0.98 it was saved with, which refuses most lossy results under the new score; edit the folder to take the new default.

**Dir.** `folder_id`, `rel_path` (`''` is the folder's root), `mtime_ns`. One row per directory the scanner has listed. `mtime_ns = 0` means "list it again": a directory found but not listed yet, or one that changed within 2 seconds of its listing.

**Image.** `id`, `folder_id`, `dir` (relative, `''` for the root), `name`, `format`, `size`, `mtime_ns`, `status`, `original_size`, `optimized_size`, `mode`, `reason`, `attempts`, `next_attempt_at`, `output_sha256`, `discovered_at`, `optimized_at`, `updated_at`. Unique on `(folder_id, dir, name)`, which also serves the scanner's lookups of one directory. `idx_images_claim` on `(folder_id, status, format, mtime_ns)` serves the claim and the counts by status. Partial indexes hold the images being optimized, the failed ones waiting for a retry and the rows with an output hash. `size` and `mtime_ns` are the file as it is on disk now, so after a swap they are the optimized file's. `original_size` is the size of the content before it was first optimized: a new upload at the same path is a new original. `output_sha256` is the SHA-256 of the bytes the optimizer last wrote to the file. `next_attempt_at` is when a failed image is tried again, empty once it has failed three times. `optimizer_outputs` holds the SHA-256 of every output the optimizer ever wrote, with no foreign key, so it outlives the image row, the folder and a new upload at the same path. The format comes from the extension (`.png`, `.jpg`, `.jpeg`, `.gif`, in any case); the optimizer checks the content.

Image status is a state machine. Each step is one conditional UPDATE on the current status, as in Gogobackup's runner. The scanner writes `pending` and `gone`. The optimizer moves `pending` to `optimizing` and `optimizing` to every other status, `gone` included when it finds the file missing; `internal/images/lifecycle.go` holds the transitions:

| Status | Meaning |
|---|---|
| `pending` | New, or changed on disk since it was last handled |
| `optimizing` | A worker claimed it |
| `optimized` | Replaced with a smaller file that passed every check |
| `kept` | Left as it was, with the reason: no saving, failed a check (`ssim_below_threshold: 0.71, below 0.85, at 128,64` and the like), or `already_optimized`: bytes the optimizer wrote at another path |
| `skipped` | Never tried, with the reason: APNG, symlink, hard link, not a regular file, a POSIX ACL, over 50 megapixels or 256 MB, data the tools would drop (`extra_data`), unreadable, wrong content for its extension |
| `failed` | A tool crashed, exited with an error or ran past its time limit, the swap failed, or the process died while optimizing it. Tried again after 10 minutes and after an hour, then left for the admin |
| `gone` | Deleted from disk |

An image's `reason` is `code` or `code: detail`, such as `ssim_below_threshold: 0.7100, below 0.8500, at 128,64`. A refused lossy result is followed by how the lossless attempt after it ended, behind `; lossless: `. Every code is in `images.ReasonCodes`, which the API sends as an enum, so the admin panel words each one in every language; a test fails when the optimizer or the checks gain a reason that list lacks.

An image that is `optimized` or `kept` and then changes on disk goes back to `pending`, with its attempts reset and its reason cleared: the reason was about the old content. An image is never compressed lossily twice, and the rule follows the bytes, not the path. Before the rename that swaps a result in, the optimizer stores the SHA-256 of the result in `output_sha256`. When it claims an image whose bytes hash to the row's own `output_sha256`, the file was only touched, and it goes back to `optimized` with its new mtime. When they hash to any other output in `optimizer_outputs`, the file was moved or copied, or its folder was deleted and added again, and it is `kept` as `already_optimized`. Nothing runs on it. Bytes with any other hash are a new upload, and lossy compression is allowed again.

**Scan.** `id`, `folder_id`, `kind` (`quick` or `full`), `trigger` (`schedule` or `manual`), `status` (`queued`, `running`, `succeeded`, `failed`), `queued_at`, `started_at`, `finished_at`, `dirs_seen`, `dirs_listed`, `files_seen`, `files_new`, `files_changed`, `files_gone`, `error`. A partial unique index on `folder_id` where the status is `queued` or `running` allows one scan per folder at a time, whoever tries to queue a second one. Like an image, a scan moves by conditional UPDATEs: queued to running when the worker claims the oldest, running to succeeded or failed when it ends. The counts are written every 2 seconds while it runs. `files_new` includes files that came back after being gone. Finishing a scan deletes the folder's scans older than its newest 100, except its latest successful full scan: `Due` reads that one, and with quick scans every 5 minutes the newest 100 can all be quick. That is about four days of scans at the default intervals.

**Setting.** One row with typed columns, because the set of settings is closed: `workers` (optimizations at once; 0, the default, means the CPU count minus one, at least 1), `scan_workers` (scans at once, default 2), `nice` (tools run under `nice -n 10`, and `ionice -c3` where it exists, which is Linux, by default, so the projects on the server stay fast). On Ubuntu 24.04 the tools run at niceness 10 in the idle I/O class (`TestOptimizer_ToolsRunAtLowPriorityWhenTheSettingSaysSo`, and `ps` and `ionice -p` during the end-to-end run). The idle class only matters to an I/O scheduler that honors priorities, such as BFQ; under `none`, which the test VM's disks use, it changes nothing.

**Daily stat.** `folder_id`, `day` (the UTC date), `optimized`, `bytes_before`, `bytes_after`. One upsert per swap, in the transaction that records it. The folder card's bytes saved is the sum of `bytes_before - bytes_after`, which also counts files deleted since; the dashboard chart reads the same rows.

## Finding new images in a huge folder

Listing a directory does not change its modification time. Creating, deleting or renaming an entry in it does. A quick scan uses this:

1. Stat every known directory. That is one `stat` per directory, not per file.
2. List only the directories whose `mtime_ns` changed, and new ones. Compare their entries with the `images` rows of that directory: new names are inserted as `pending`, missing ones become `gone`, a changed size or mtime goes back to `pending`.
3. Store each listed directory's new `mtime_ns`.

On a tree of a million images in ten thousand directories, a quick scan with no changes is about ten thousand `stat` calls. Measured in phase 3 on 1,000,000 files in 10,111 directories ([docs/benchmarks/scanner.md](benchmarks/scanner.md), an M2 Mac, APFS, median of 7): 18 ms with the directories in the kernel's cache, and about 0.4 s right after a full scan has evicted them from macOS's 250,981-entry vnode cache. Either way it writes no row. A quick scan after 100 files were added and 50 deleted takes 79 ms and writes exactly those rows. A full scan with no change takes 12.6 s, nearly all of it `lstat` and `readdir`, and writes nothing. The first full scan takes 22 s, most of it SQLite writing 920,000 rows.

A quick scan misses a file rewritten in place under the same name, because that does not touch its directory. The full scan walks every file and compares size and mtime. It runs every `full_scan_interval_hours` and as the first scan of a new folder.

The scanner inserts rows in batches of 1,000, each in one transaction. The optimizer starts on a new folder's first images while its first full scan is still walking.

Both kinds of scan go through one step per directory, which reconciles the directory's entries with its rows. New image files are inserted as `pending`. A file whose size or mtime changed goes back to `pending`, keeping what the optimizer recorded. A row whose file is missing becomes `gone`, and a `gone` row whose file is back is `pending` again with its history, so an image optimized lossily before is still never compressed lossily twice. A full scan lists every directory from the root. A quick scan lstats every known directory and lists the changed ones and any new ones it finds. A folder whose root was never listed gets a full scan whatever kind was asked for.

The rules around that step:

- **The mtime tick.** A directory whose mtime is within 2 seconds of the moment it was listed is stored with `mtime_ns = 0`. A file created after the listing in the same timestamp tick would leave the mtime as stored, and the next quick scan would skip the directory.
- **Stopping anywhere.** A new directory gets its row, with `mtime_ns = 0`, before its parent's mtime is stored, and writes go out in the order they were made. A scan stopped at any point, by a crash, a stop or an error, leaves rows that the next quick scan completes.
- **Directories that went away.** A directory that is missing, now a file, now a symlink, newly excluded or on another filesystem takes every image under it to `gone`, and its `dirs` rows are deleted. A missing root fails the scan instead of marking everything gone.
- **What is indexed.** Regular files with an image extension. Symlinks, to files or directories, are never followed and never indexed. The profile's `excludes` match the path relative to the root, of files and of directories, and an excluded directory is not entered. A directory on another device than the root (`st_dev`) is skipped and counted in the log.
- **Temp files.** `.goptimizer-*.tmp` files are never indexed. The scan deletes the ones older than an hour by their inode change time: the optimizer gives a temp file the original's mtime (step 5 below), so the mtime says nothing about its age.
- **Stopping a scan.** It checks between directories whether the service is stopping and, at most once a second, whether its folder was paused or deleted. It then ends `failed` with "interrupted: the service stopped" or "the folder was paused". A deleted folder takes its scans with it.

A scan that finds no change writes nothing: no image row, no directory row.

## Not overlapping

- Scans and optimizations are rows in SQLite, not in-memory jobs. A restart resets `running` scans to `failed` and `optimizing` images to `pending`, and the work carries on from there.
- The scheduler ticks every minute. A folder that is due gets a queued scan, and the partial unique index makes a second one a no-op. An hourly tick during a scan that has run for three hours does nothing.
- Due means: nothing within `scan_interval_minutes` of the start of the folder's last finished scan, succeeded or failed, so a folder that cannot be read is retried on its interval and not every minute. Past that, a full scan when the folder never had a successful one or the last started `full_scan_interval_hours` ago, and a quick scan otherwise. Times count whole minutes, because a scan starts a moment after its tick and an hourly scan would otherwise run every 61 minutes.
- The scan worker lives in the process that runs the scheduler: the API, or `cmd/cron` with `ENABLE_SCHEDULER=false`. It runs at most `scan_workers` scans at once, reading the setting before each claim. It wakes at once for a scan the same process queued and polls every 5 seconds for the rest. A scan queued by hand (`POST /folders/:id/scans`) answers 409 while another is queued or running.
- Throughput, measured on 96 real and generated images on a busy 8-core laptop ([docs/benchmarks/optimizer.md](benchmarks/optimizer.md)): 0.85 images a second with 1 worker, 2.48 with 4, 2.75 with 7, median of 5 runs each. The tools take 92% of the CPU, and 7 workers kept 6.1 cores busy, so the rate is bound by cores; 4 workers already reach 90% of it because oxipng runs several threads.
- The worker pool is shared by all folders and lives next to the scan worker, in the process that runs the scheduler. It runs at most `workers` images at once, reading the setting before each claim, wakes when a scan finds new or changed images, and polls every 5 seconds for the rest. A claim is one write transaction. Failed images whose retry time has come go back to `pending` first, at most every 30 seconds. Then the folders are ordered by how many of their images are in flight, fewest first, and round-robin from the folder after the one claimed from last among equals, so with one worker the folders take turns and with many no folder holds them all. The first folder with a claimable image gives its pending image with the oldest mtime, in a format whose tools are installed for its mode, at least `min_age_minutes` old: one seek on `idx_images_claim` per format. The conditional UPDATE `pending` to `optimizing` takes it. A first run of five million images does not starve a small folder that gets ten new uploads: the small one gets every other claim (`TestClaim_ABigFolderDoesNotStarveASmallOne`).
- A paused folder queues no scans, a queued or running scan of it stops, and the pool skips its pending images. A folder whose pipeline needs a missing tool keeps its images of that format `pending`.
- The scanner leaves a row that is `optimizing` alone: its file may be mid-swap. Every other write it makes applies only if the row still has the status, size and mtime the scan read, so a scan that read a row before a swap and writes after the optimizer recorded it changes nothing. The optimizer's lstat just before the rename catches a real change while it worked.

## Optimizing one image

Formats are a registry. `folders.Profile.Need` maps each format and mode to a tool need, `tools.NeedSteps` lists the programs each need runs with their alternatives, and `internal/optimizer/pipelines.go` says how each program is called. Adding WebP later is one entry in each.

| Format | Lossy | Lossless |
|---|---|---|
| PNG | `pngquant --quality 65-80 --skip-if-larger`, then `oxipng -o 3 --keep display,gAMA,cHRM,eXIf` | `oxipng -o 3 --keep display,gAMA,cHRM,eXIf`, or `optipng -o2` when oxipng is missing |
| JPEG | `jpegoptim --max 82 --all-progressive --strip-com --strip-xmp --strip-iptc` | the same without `--max` |
| GIF | `gifsicle -O3 --lossy=30` | `gifsicle -O3` |

`jpegoptim` keeps the EXIF block and the ICC profile on purpose. Without the orientation tag a phone photo shows sideways, and without its profile a Display P3 photo looks washed out. oxipng's `--strip safe` drops `gAMA`, `cHRM` and `eXIf` (checked with oxipng 10.2.1), which change how a PNG shows or which way up, so the command keeps them. pngquant's exit codes 98 and 99 mean it left the file alone because the result would be larger or below the quality range; the next step runs on the copy as it is.

The steps, for one claimed image:

1. Open the image's directory one component at a time from the folder's root, refusing a symlink at every level, and `lstat` the file in it. Every later step works on that directory descriptor, so a directory renamed or replaced by a symlink meanwhile cannot redirect the swap. A symlink, a file that is not regular, a file with more than one hard link, one with a POSIX ACL (the swap could not give the ACL to the new file, and it may be what lets the web server read it) or one over 256 MB is skipped. A file younger than `min_age_minutes` goes back to `pending` with its new mtime. A missing file is `gone`.
2. Read the file, checking that what was opened is the file that was lstatted and that its size and mtime did not move during the read. Its first bytes give the real format: a JPEG named `.png` is skipped. An APNG (a PNG with an `acTL` chunk before `IDAT`) is skipped, because quantizing it would drop the animation. So is an image with data the tools drop and no check would notice: bytes after its end (where a motion photo keeps its video and an Ultra HDR photo its gain map), a JPEG MPF index, JUMBF box or XMP that points at a container, or a PNG `caBX` chunk (C2PA content credentials). The headers give the pixel count, frames included for a GIF, without decoding: over 50 million, the image is skipped, so a small file that claims to be huge cannot exhaust memory. 50 million covers the 48 MP photos of current phones; by arithmetic, verifying a JPEG that size holds about 650 MB while it runs (two decoded images and their 8-bit RGBA copies), not measured.
3. If the bytes are the optimizer's own output (see Image), stop there. Otherwise copy them into a new directory under `WORK_DIR` and run the steps on that copy, each under a time limit (5 minutes) and in its own process group, so a hung tool and anything it started are killed. No tool ever gets the project's file. A tool that crashes, exits with an error or runs past its limit fails the image.
4. Verify the result (below). A refused lossy result gets one more try: the lossless pipeline on the original, held to the lossless checks, recorded with `mode = lossless`. If that is refused too, or its tool fails, the image becomes `kept` with both reasons, and the original is not touched. A refusal or a failure is recorded against the file as it is now: if it changed while the tools ran, the image is `pending` for the new file instead.
5. Store the result's SHA-256 in `optimizer_outputs` and the row's `output_sha256`. Create `.goptimizer-<random>.tmp` next to the original with `O_EXCL`, so the rename stays on one filesystem, write the result, give it the original's owner, group, mode, access time and modification time (the owner first, because `chown` clears setuid bits), and fsync it.
6. `lstat` the original again, immediately before the rename, with nothing in between. If its size, mtime, inode, link count, inode change time, mode or owner changed while the tools ran, discard the result and set the image back to `pending` with the file as it is now. The inode change time catches a same-size rewrite that set the mtime back, and a `chmod` that would make the mode given to the result stale.
7. Rename the temp file over the original. That is atomic: a web server reads either the old file or the new one. Fsync the directory. Any error after the temp file exists and before the rename removes the temp file.
8. Record `optimized` with the new size, the original's mtime, `original_size`, `optimized_size`, `mode` and `optimized_at`, and add to the day's stats in the same transaction.

A stop of the service kills the tools and sets the image back to `pending` without counting an attempt. Once a result has passed verification, the swap and its record finish even during a stop. An image still `optimizing` at the next start was being worked on when the process died: it is `pending` again with that attempt counted, or `failed` at the third, so an image that kills the process every time is not first in line after every restart. If the process died between the rename and the record, the bytes are recognized as the optimizer's output by the hash stored in step 5; that swap's `original_size` and daily stats are lost.

A scan removes `.goptimizer-*.tmp` files older than an hour by their inode change time, which a crash between steps 5 and 7 can leave behind. At its start the optimizer deletes the `job-*` directories a previous run left in `WORK_DIR` (by default a directory under the OS temp dir named after the database), and refuses to start when the directory is not its own or other users can use it.

The original mtime is kept so tools that sort a project's files by date still see the upload date. nginx's ETag includes the size, so caches still notice the new file.

**Known risk.** Between the `lstat` of step 6 and the rename of step 7, another process can still write the original, and that write is lost when the rename replaces the file. Closing the window needs a lock every writer of the project takes, which a web application does not. The window is two system calls on an open directory descriptor; `min_age_minutes` keeps uploads in progress out of it.

## Verification

Every result has to pass all of these before it replaces a file, cheapest first (`internal/verify`):

- It is smaller by at least `min_saving_percent`, measured on the final bytes.
- Its first bytes say the same format.
- The metadata browsers render with is kept. A PNG keeps each of `cICP`, `iCCP`, `sRGB`, `gAMA`, `cHRM` and `pHYs` it has, with the same content (`iCCP` compared by its decompressed profile), adds no `cICP` or `iCCP`, and keeps the orientation of its `eXIf`. A JPEG keeps its ICC profile byte for byte and its EXIF orientation.
- It decodes with Go's own decoders (`image/png`, `image/jpeg`, `image/gif`), with the same width and height, read from the headers before the full decode. A GIF has the same number of frames, the same loop count and the same frame delays, and its frames compare after composing them by their disposal methods from a transparent start, as a browser shows them: optimizers crop frames to what changed.
- Lossless: every pixel equals the original's as 16-bit non-premultiplied RGBA, so a palette in place of truecolor is no difference. A fully transparent pixel equals any other, whatever color hides under it.
- Lossy: no 32×32 block's average Y, Cb or Cr moved more than 12 levels, which catches a result that turned gray or changed a flat color, which SSIM barely sees. And the SSIM of the lowest 64×64 tile is at least `min_ssim`. SSIM is computed in pure Go over 8×8 windows at every position, at full resolution. PNG and GIF are scored on red, green and blue composited on black and, when either image has transparency, on white, taking the worst, so a pixel that changes its alpha is a difference. JPEG is scored on luma, because a re-encode moves the color channels at the scale of chroma subsampling, which the eye does not resolve. The tile score catches a banded shadow or a shifted tint that the mean over the image hides. The default of 0.85 comes from 96 real and generated images ([docs/benchmarks/ssim.md](benchmarks/ssim.md)): results with visible damage scored 0.793 or less. It holds with Ubuntu 24.04's builds of the tools, whose gifsicle 1.94 takes more from GIFs and is refused more often.
- Bytes the optimizer wrote itself are never compressed again (see Image).

## Dashboard

The folders page leads with what every folder adds up to: bytes saved, images optimized, waiting (`pending`) and failed, and what runs now, scans queued or running and images optimizing. The figures come from `GET /folders`, which already carries each folder's counts by status and bytes saved. Under them, a chart of the bytes saved on each of the last 30 UTC days (`GET /daily-savings`, a day without optimizations as zero), then the folder cards. The page refreshes every 2 seconds while a scan or an optimization is in flight, every 10 seconds while an enabled folder with every tool has images waiting, and not at all otherwise.

A card's name opens the folder's page (`/folders/:id`): its settings and the same controls as the card, its figures and chart, its images and its scans, 10 to a page, newest first, with what each found and how long it took.

The images list has a tab per status with its count, a search by path and two orders: the latest change first, or, for optimized images, the most bytes saved first. Pages are keyset pages, never OFFSET: the cursor is the last row's sort key, and each order has its index (migration 007), so any page of a million images is one seek, about 0.2 ms on 900,000 ([docs/benchmarks/images.md](benchmarks/images.md)). A search matches any part of the path, ASCII letters in any case, and lists by path. It reads only the path index and reads a row only for a path that matches: about 0.3 s on 900,000 images when nothing matches, a few milliseconds when matches are common. Kept, skipped and failed images say why in plain words, from the reason's code, with what was measured under it; a lossy result that was refused also says how the lossless attempt ended.

Retry sets a failed image, or every failed image of a folder, back to `pending` with its attempts reset, one conditional UPDATE each. A kept image has no retry: the same bytes, tools and profile give the same verdict, and bytes the optimizer wrote are recognized by their hash at the claim whatever the status says.

## Settings page

One table of every tool the registry uses: the program, which formats and modes need it, whether it is required, whether it is installed, its path and version (`tools.Finder`, as in Gogobackup). A missing tool shows how to install it: the `apt` package for Ubuntu, `brew` on macOS. Folders whose pipeline needs a missing tool show a warning, and their images stay `pending` instead of failing.

Below the table, the global settings: workers, scan workers, nice and ionice.

## Deploy

Three playbooks, as in Gogobackup: `provision.yml` for an empty server, `install.yml` for a server that already runs other sites and has Go, Node.js, nginx and supervisor, and `deploy.yml` for every release after `provision.yml`. `make deploy` on the server is the release after `install.yml`. The changes for this service:

- **The image tools.** All three playbooks install `pngquant`, `jpegoptim`, `optipng` and `gifsicle` from apt, and `oxipng` 10.2.1, which no Ubuntu release packages, from its GitHub release (`roles/imagetools`): the static musl build for x86_64 or aarch64, checked against a pinned SHA-256, unpacked as root's into `/opt` and linked into `/usr/local/bin`. On another architecture `optipng` does its step. oxipng earns the download, measured on the calibration corpus on Ubuntu 24.04, median of 5 alternated runs ([optimizer.md](benchmarks/optimizer.md#on-linux)): the whole optimizer handles 1.63 images a second with it and 0.60 with `optipng` in its place, on half the CPU, and saves 25.6% of the bytes instead of 24.4%. On the 50 PNGs alone it saves 11.3 MB of 39.5 where `optipng -o2` saves 8.6, and after pngquant another 17% where optipng saves 0.8%.
- **root, on 127.0.0.1.** Supervisor runs the API, and `cmd/cron` when it runs the jobs, as root. The API listens on `BIND_ADDRESS`, which is `127.0.0.1` by default with `APP_ENV=production` and every interface otherwise, so nginx is the way in. `backend/.env` is root's, mode 0600, and a command that cannot read it stops instead of running on the defaults. The database folder is root's, mode 0700.
- **The work dir.** `WORK_DIR` is `/var/lib/<app>/work`, root's, mode 0700, created by the playbooks. The default under the OS temp dir is a directory another local user could create first; the optimizer then refuses to start, and so does the API.
- **Releases.** `deploy.yml` and `make deploy` pull, build, migrate, restart and wait for `/health`, and fail when it does not answer within a minute.

**Verified on Linux**, in Docker containers on an M2's Docker Desktop VM (arm64, kernel 5.15), not on a real server:

- The backend tests on Ubuntu 24.04 with the apt tools and oxipng: as root, every test passes but the two that need a non-root user, which skip; as a user, only the one that needs root skips. That covers the swap of a `www-data` file with mode 0640 and of one with setuid and setgid bits, the ACL skip, and nice with ionice.
- `provision.yml` on a bare Ubuntu 24.04 image with systemd and SSH, then again: the second run changed nothing. `deploy.yml` with new commits changed the five release steps, then nothing. `install.yml` on a server that already had Go, Node.js, nginx and supervisor; its second run changes only its three release steps (build, restart, nginx reload), which it always runs. The image tools role on x86_64 (emulated), twice: the second run changed nothing.
- Through nginx on both servers: an admin from the CLI, a sign-in, and a folder of 45 image files owned by `www-data:www-data` with mode 0640, from this repository and generated, among them a hard-linked pair, a file with an ACL and a JPEG named `.png`, next to a symlink. The scheduler's scan and the pool optimized 40, kept 1 and skipped 4, each for its reason. Every changed file decodes with the same dimensions and frame count and kept its owner, group, mode, mtime and atime; no temp file was left; the 5,775,877 bytes saved on disk are what `GET /folders`, `GET /daily-savings` and the image list report. A second full scan and pass changed no file. From another container the API's port refused connections (the firewall of the provisioned server also dropped them), while nginx answered, and `ps` showed the API as root and the tools as root at niceness 10 in the idle I/O class.

Not verified: a real server, Ubuntu 22.04, the whole stack on x86_64, and `tls_mode: letsencrypt`, which needs a public domain.

## Open decisions

- An exclude added to a profile takes effect at the next scan for the directories it matches, whose images become `gone`. For files it matches in a directory that did not change, it takes effect at the next full scan.
- A folder whose root is a mount point lists as an empty directory while the disk is unmounted, so a scan then marks all its images `gone`. They come back `pending` with their history when the disk is back, and an image optimized lossily is still never compressed lossily twice. Refusing a scan whose root changed device is the guard if this turns out to matter.
- A directory the scanner cannot read for any reason but "it is gone" fails the whole scan with that error, and the next scan tries again. Running as root makes this rare; a scan that skips such a directory and reports it is the alternative if it is not.
- pngquant strips metadata on both systems: 3.0.3 on macOS by default, and Ubuntu 24.04's 2.18.0 drops `cICP`, `iCCP`, `cHRM` and `eXIf` as well ([ssim.md](benchmarks/ssim.md#on-linux)). Its results for PNGs with an ICC profile or `cHRM` are refused, and the lossless pipeline runs instead. oxipng replaces a standard sRGB ICC profile with an `sRGB` chunk and drops `iCCP` from images it reduces to gray; the check refuses both, which costs savings on those PNGs. Accepting an `sRGB` chunk in place of an sRGB profile is the refinement if it matters.
- `gifsicle -O3` merges identical consecutive frames and adds up their delays. The check counts frames, so it refuses those results (2 of 13 GIFs in the calibration corpus). Comparing the frames as shown over time would accept them.
- SSIM does not see banding without dither: a gray gradient cut to 32 levels scores 0.917 on its worst tile, above 0.85. pngquant did not produce that case on the calibration corpus, because its `--quality` floor refuses to quantize smooth gradients, but another tool could.
- A lossy result that is refused falls back to the lossless pipeline in the same claim. On the calibration corpus that recovered 22% of the bytes of the PNGs whose lossy result was refused, which would otherwise be left as they were.
- The optimizer's memory grows with the image: about 13 bytes per pixel for a JPEG at the 50 MP limit, times the number of workers. A server with little memory and many workers should set fewer workers; a limit by memory is the alternative.
- The swap gives the result the original's owner, group, mode and times, but not its other extended attributes or file flags. A file with a POSIX ACL is skipped for that reason; an SELinux label is the new file's default for its directory, which is usually the same. A file with an ACL is skipped on Linux (`TestOptimizer_SkipsAFileWithAnACL` on Ubuntu 24.04, and a file given an ACL with `setfacl` in the end-to-end run).
- A lossless result compares fully transparent pixels as equal whatever color hides under them, as every color model with alpha does. A texture that a program reads without premultiplied alpha could show the difference at its edges.

- A search on a folder of a million images costs up to a pass over its path index, about 0.3 s, and 0.7 to 0.8 s in the corner case of a text that matches nearly every path in a status almost no image has. An FTS5 table with the trigram tokenizer would make any search fast, at the price of a second copy of every path kept in step with the scanner. Not worth it for a search an admin runs by hand, unless that changes.
- A kept image cannot be tried again from the folder page. After a change to the folder's profile, such as a lower `min_ssim`, its kept images would deserve another try; setting them back to `pending` when the profile's thresholds change is the way, if it is wanted.
- The chart's days are UTC dates, as `daily_stats` stores them, so near midnight a swap counts on the day UTC says.

- The web UI runs as root, listening on `127.0.0.1` behind nginx, so it is off the network but a bug in it is a bug in a root process. An option for later is a root worker process plus an unprivileged API, split the way Gogobackup's `cmd/cron` is.
- The deploy user owns and builds the code that root runs, so it is as powerful as root. It has no password and no SSH login. Building as root instead would run the frontend's `npm ci` scripts as root.
- The tools parse the uploads of every project, as root. A memory-safety bug in pngquant, libpng, jpegoptim, optipng or gifsicle that an uploaded file can trigger now gives root, where before phase 6 it gave the service's own user. Only the swap needs root: running each tool under an unprivileged uid, on a job dir that uid owns, would take that away.
- Every account can add a folder anywhere outside the system directories, and root rewrites its images. That is the decision in [Decisions](#decisions), with `allow_registration` off; with it on, anyone who registers could. A list of allowed roots set by the deploy, or folders for admins only, is the way if accounts are ever less trusted.
- `WORK_DIR` must sit in directories only root can write. The check at start covers the work dir itself, not its parents; the default `/var/lib/<app>/work` and the temp dir fallback are safe.
- A scan opens each directory by its path, refusing a symlink only as the last component. Someone who owns a project's tree can swap a directory for a symlink between the listing of its parent and its own, and file names from elsewhere on the filesystem then show in the panel. The optimizer refuses them, because it opens directories one component at a time from the root; the scanner doing the same is the fix.
- Email when a folder's images start failing, through the existing mail module. Not in v1 unless asked.
