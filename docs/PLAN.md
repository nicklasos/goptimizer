# Goptimizer plan

Goptimizer is a web app that runs on a server and keeps the images of the projects on that server optimized. You add a folder in the browser. The service finds new and changed images in it on a schedule, shrinks each one with command-line tools such as `pngquant`, `oxipng`, `jpegoptim` and `gifsicle`, and replaces the file only when the result is smaller and passes every check. The dashboard shows each folder, its queue and how many bytes it saved.

It is built from [Gogobackup](https://github.com/nicklasos/gogobackup), which is [Gogokit](https://github.com/nicklasos/gogokit) already converted to SQLite, with an in-process scheduler, a claim-based worker, a tool finder and Ansible deploy. This file is the plan. Each phase is one pull request that ends with `make test` green.

## Decisions

| Question | Decision |
|---|---|
| Where results go | In place. No backup of originals. Safety comes from the checks in [Verification](#verification). |
| Default compression | Lossy, as TinyPNG does: palette quantization for PNG, a capped quality for JPEG. Each folder can switch to lossless. |
| Process user | root, so it can rewrite any project's files and give each file back its owner and mode. |
| Sources | Local folders only. `kind` is a registry, so SSH, S3 and GCS are new entries later. |
| WebP and AVIF copies | Not in v1. |
| SVG | Not in v1. SVG optimizers rewrite markup and can change how a file renders. |
| Users | Two kinds: admins and users (`users.is_admin`). Every user can add, edit, pause and delete any folder and change the settings. Admins also manage accounts and can make other admins. |

## Phases

| # | Phase | Ends when |
|---|---|---|
| 1 | Base | Gogobackup copied in and renamed, the backup domain removed, `make test` green |
| 2 | Folders and settings | Folders can be added, edited, paused. The settings page lists every tool, installed or not, with how to install it |
| 3 | Scanner | Quick and full scans fill the image index. A scan of 1,000,000 synthetic files is measured |
| 4 | Optimizer | Pending images are optimized, verified and swapped in by a worker pool shared fairly by all folders |
| 5 | Dashboard | Folder cards with stats and a 30-day chart, a folder page with scans and images filtered by status, README screenshots |
| 6 | Deploy | `make provision`, `make install-remote` and `make deploy-remote` install the tools and the app and release it |

Phase 1 removes storages, plans, runs, restores, archives, self-backup and the secret box from both halves before anything is added, so phases 2 to 5 build on a smaller base.

## Data model

**Folder.** `id`, `name` (unique, case-insensitive), `kind` (`local`), `path` (absolute, must exist and be a readable directory, stored with symlinks resolved), `enabled`, `scan_interval_minutes` (quick scan, default 60, 5 to 10080), `full_scan_interval_hours` (default 24, 1 to 720), `profile` (JSON, below), `created_at`, `updated_at`. Two folders cannot overlap: a path inside another folder's path, or around it, is refused. `/` and the system directories (`/proc`, `/sys`, `/dev`, `/boot`, `/etc`, `/bin`, `/sbin`, `/usr`, `/lib*`, `/var/lib`) and everything inside them are refused too.

**Profile.** `mode` (`lossy` or `lossless`), `jpeg_max_quality` (82), `png_quality_min` and `png_quality_max` (65 and 80, pngquant's `--quality 65-80`), `gif_lossy` (30), `min_saving_percent` (3: a smaller saving is not worth a rewrite), `min_ssim` (lossy only, 0.98 until phase 4 calibrates it), `min_age_minutes` (5: newer files may still be uploading), `excludes` (glob patterns with `**`, such as `cache/**`). A field left out of a request or of a stored profile takes its default.

**Dir.** `folder_id`, `rel_path` (`''` is the folder's root), `mtime_ns`. One row per directory the scanner has listed. `mtime_ns = 0` means "list it again": a directory found but not listed yet, or one that changed within 2 seconds of its listing.

**Image.** `id`, `folder_id`, `dir` (relative, `''` for the root), `name`, `format`, `size`, `mtime_ns`, `status`, `original_size`, `optimized_size`, `mode`, `reason`, `attempts`, `discovered_at`, `optimized_at`, `updated_at`. Unique on `(folder_id, dir, name)`, which also serves the scanner's lookups of one directory, and indexed on `(folder_id, status)`. `size` and `mtime_ns` are the file as it is on disk now, so after a swap they are the optimized file's. Phase 3 created every column, so phase 4 needs no migration for them. The format comes from the extension (`.png`, `.jpg`, `.jpeg`, `.gif`, in any case); the optimizer checks the content.

Image status is a state machine. Each step is one conditional UPDATE on the current status, as in Gogobackup's runner:

| Status | Meaning |
|---|---|
| `pending` | New, or changed on disk since it was last handled |
| `optimizing` | A worker claimed it |
| `optimized` | Replaced with a smaller file that passed every check |
| `kept` | Left as it was, with the reason: no saving, failed a check, already optimized |
| `skipped` | Never tried, with the reason: APNG, symlink, hard link, unreadable, wrong content for its extension |
| `failed` | A tool failed. Retried up to 3 times, then left for the admin |
| `gone` | Deleted from disk |

An image that is `optimized` or `kept` and then changes on disk goes back to `pending`. One that was optimized lossily keeps `mode = lossy`, and the optimizer never runs lossy compression on it again.

**Scan.** `id`, `folder_id`, `kind` (`quick` or `full`), `trigger` (`schedule` or `manual`), `status` (`queued`, `running`, `succeeded`, `failed`), `queued_at`, `started_at`, `finished_at`, `dirs_seen`, `dirs_listed`, `files_seen`, `files_new`, `files_changed`, `files_gone`, `error`. A partial unique index on `folder_id` where the status is `queued` or `running` allows one scan per folder at a time, whoever tries to queue a second one. Like an image, a scan moves by conditional UPDATEs: queued to running when the worker claims the oldest, running to succeeded or failed when it ends. The counts are written every 2 seconds while it runs. `files_new` includes files that came back after being gone.

**Setting.** One row with typed columns, because the set of settings is closed: `workers` (optimizations at once; 0, the default, means the CPU count minus one, at least 1), `scan_workers` (scans at once, default 2), `nice` (tools run under `nice -n 10` and `ionice -c3` by default, so the projects on the server stay fast).

**Daily stat.** `folder_id`, `day`, `optimized`, `bytes_before`, `bytes_after`. One upsert per optimized image, read by the dashboard chart.

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
- The worker pool is shared by all folders. It claims images round-robin across folders, so a first run of five million images does not starve a small folder that gets ten new uploads.
- A paused folder queues no scans, a queued or running scan of it stops, and the pool skips its pending images.

## Optimizing one image

Formats are a registry: `map[Format]pipeline`, with a lossy and a lossless list of steps each. Adding WebP later is one entry.

| Format | Lossy | Lossless |
|---|---|---|
| PNG | `pngquant --quality 65-80 --skip-if-larger`, then `oxipng -o 3 --strip safe` | `oxipng -o 3 --strip safe`, or `optipng -o2` when oxipng is missing |
| JPEG | `jpegoptim --max 82 --all-progressive --strip-com --strip-xmp --strip-iptc` | the same without `--max` |
| GIF | `gifsicle -O3 --lossy=30` | `gifsicle -O3` |

`jpegoptim` keeps the EXIF block and the ICC profile on purpose. Without the orientation tag a phone photo shows sideways, and without its profile a Display P3 photo looks washed out.

The steps, for one claimed image:

1. `lstat` the file. A symlink, a file with more than one hard link or a file younger than `min_age_minutes` is skipped or left for later.
2. Read the first bytes to know the real format. A JPEG named `.png` is skipped. An APNG (a PNG with an `acTL` chunk) is skipped, because quantizing it would drop the animation.
3. Copy it into `WORK_DIR` and run the steps there. No tool ever writes to the project's file.
4. Verify the result (below). On any failure the image becomes `kept` with the reason, and the original is not touched.
5. Copy the result into a temp file next to the original, `.goptimizer-<random>.tmp`, so the rename stays on one filesystem. Give it the original's owner, group, mode and mtime.
6. `lstat` the original again. If its size or mtime changed while the tools ran, discard the result and set the image back to `pending`.
7. Rename the temp file over the original. That is atomic: a web server reads either the old file or the new one.
8. Record the new size and mtime, and add to the day's stats.

A scan removes `.goptimizer-*.tmp` files older than an hour by their inode change time, which a crash between steps 5 and 7 can leave behind.

The original mtime is kept so tools that sort a project's files by date still see the upload date. nginx's ETag includes the size, so caches still notice the new file.

## Verification

Every result has to pass all of these before it replaces a file:

- It is smaller by at least `min_saving_percent`.
- It decodes with Go's own decoders (`image/png`, `image/jpeg`, `image/gif`) in the same format, with the same width and height, and a GIF has the same number of frames.
- Lossless: every pixel equals the original's. These tools are lossless by design, so this rejects only a bug.
- Lossy: SSIM against the original is at least `min_ssim`. That catches the case where quantization bands a gradient. The default is set in phase 4 from a sample of real photos, screenshots and illustrations.
- A lossy result for an image already optimized lossily is never produced.

## Settings page

One table of every tool the registry uses: the program, which formats and modes need it, whether it is required, whether it is installed, its path and version (`tools.Finder`, as in Gogobackup). A missing tool shows how to install it: the `apt` package for Ubuntu, `brew` on macOS. Folders whose pipeline needs a missing tool show a warning, and their images stay `pending` instead of failing.

Below the table, the global settings: workers, scan workers, nice and ionice.

## Deploy

Same three playbooks as Gogobackup, with these changes:

- `provision.yml` also installs `pngquant`, `jpegoptim`, `optipng` and `gifsicle` from apt. No Ubuntu release packages `oxipng` (checked on Launchpad in phase 2), so phase 6 either installs its release binary or leaves the lossless PNG step to `optipng`.
- Supervisor runs the API as root, and the API listens on `127.0.0.1` only. Today it binds every interface.
- `install.yml` installs the app on a server that already has Go, Node.js, nginx and supervisor, and installs the image tools too, because the app cannot run without them.
- `deploy.yml` and `make deploy` pull, build, migrate, restart and wait for `/health`.

## Open decisions

- Scans are never deleted. At the default intervals that is about 9,000 rows a year per folder. A retention rule, such as the last 100 per folder, can come with the scans page in phase 5.
- An exclude added to a profile takes effect at the next scan for the directories it matches, whose images become `gone`. For files it matches in a directory that did not change, it takes effect at the next full scan.
- A folder whose root is a mount point lists as an empty directory while the disk is unmounted, so a scan then marks all its images `gone`. They come back `pending` with their history when the disk is back, and an image optimized lossily is still never compressed lossily twice. Refusing a scan whose root changed device is the guard if this turns out to matter.
- A directory the scanner cannot read for any reason but "it is gone" fails the whole scan with that error, and the next scan tries again. Running as root makes this rare; a scan that skips such a directory and reports it is the alternative if it is not.
- The scanner and the optimizer will both write an image row. Phase 4 decides how a scan that finds a file mid-swap, between the rename and the recorded size, keeps the row from going back to `pending`.

- The web UI runs as root. Binding to `127.0.0.1` behind nginx keeps it off the network. An option for later is a root worker process plus an unprivileged API, split the way Gogobackup's `cmd/cron` is.
- Email when a folder's images start failing, through the existing mail module. Not in v1 unless asked.
