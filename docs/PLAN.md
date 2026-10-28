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

**Folder.** `id`, `name`, `kind` (`local`), `path` (absolute, must exist and be a directory), `enabled`, `scan_interval_minutes` (quick scan, default 60), `full_scan_interval_hours` (default 24), `profile` (JSON, below), `created_at`, `updated_at`. Two folders cannot overlap: a path inside another folder's path is refused.

**Profile.** `mode` (`lossy` or `lossless`), `jpeg_max_quality` (82), `png_quality` (`65-80`), `gif_lossy` (30), `min_saving_percent` (3: a smaller saving is not worth a rewrite), `min_ssim` (lossy only, calibrated in phase 4), `min_age_minutes` (5: newer files may still be uploading), `excludes` (glob patterns, such as `cache/**`).

**Dir.** `folder_id`, `rel_path`, `mtime_ns`. One row per directory the scanner has listed.

**Image.** `id`, `folder_id`, `rel_path`, `format`, `size`, `mtime_ns`, `status`, `original_size`, `optimized_size`, `mode`, `reason`, `attempts`, `discovered_at`, `optimized_at`. Unique on `(folder_id, rel_path)`, indexed on `(folder_id, status)`. `size` and `mtime_ns` are the file as it is on disk now, so after a swap they are the optimized file's.

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

**Scan.** `id`, `folder_id`, `kind` (`quick` or `full`), `trigger` (`schedule` or `manual`), `status` (`queued`, `running`, `succeeded`, `failed`), `started_at`, `finished_at`, `dirs_listed`, `files_seen`, `files_new`, `files_changed`, `files_gone`, `error`. A partial unique index on `folder_id` where the status is `queued` or `running` allows one scan per folder at a time, whoever tries to queue a second one.

**Setting.** Key and value: `workers` (optimizations at once, default the CPU count minus one), `scan_workers` (scans at once, default 2), `nice` (tools run under `nice -n 10` and `ionice -c3` by default, so the projects on the server stay fast).

**Daily stat.** `folder_id`, `day`, `optimized`, `bytes_before`, `bytes_after`. One upsert per optimized image, read by the dashboard chart.

## Finding new images in a huge folder

Listing a directory does not change its modification time. Creating, deleting or renaming an entry in it does. A quick scan uses this:

1. Stat every known directory. That is one `stat` per directory, not per file.
2. List only the directories whose `mtime_ns` changed, and new ones. Compare their entries with the `images` rows of that directory: new names are inserted as `pending`, missing ones become `gone`, a changed size or mtime goes back to `pending`.
3. Store each listed directory's new `mtime_ns`.

On a tree of a million images in ten thousand directories, a quick scan with no changes is about ten thousand `stat` calls. That is an estimate to be measured in phase 3.

A quick scan misses a file rewritten in place under the same name, because that does not touch its directory. The full scan walks every file and compares size and mtime. It runs every `full_scan_interval_hours` and as the first scan of a new folder.

The scanner inserts rows in batches of 1,000, each in one transaction. The optimizer starts on a new folder's first images while its first full scan is still walking.

## Not overlapping

- Scans and optimizations are rows in SQLite, not in-memory jobs. A restart resets `running` scans to `failed` and `optimizing` images to `pending`, and the work carries on from there.
- The scheduler ticks every minute. A folder that is due gets a queued scan, and the partial unique index makes a second one a no-op. An hourly tick during a scan that has run for three hours does nothing.
- The worker pool is shared by all folders. It claims images round-robin across folders, so a first run of five million images does not starve a small folder that gets ten new uploads.
- A paused folder queues no scans, and the pool skips its pending images.

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

A scan removes `.goptimizer-*.tmp` files older than an hour, which a crash between steps 5 and 7 can leave behind.

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

- `provision.yml` also installs `pngquant`, `jpegoptim`, `optipng`, `gifsicle` and `oxipng` from apt. Whether Ubuntu 22.04 packages `oxipng` is to be checked in phase 6. Without it the lossless PNG step falls back to `optipng`.
- Supervisor runs the API as root, and the API listens on `127.0.0.1` only. Today it binds every interface.
- `install.yml` installs the app on a server that already has Go, Node.js, nginx and supervisor, and installs the image tools too, because the app cannot run without them.
- `deploy.yml` and `make deploy` pull, build, migrate, restart and wait for `/health`.

## Open decisions

- Users. Gogobackup has super admins, admins and users. A single admin role is probably enough, and phase 1 can drop the rest.
- The web UI runs as root. Binding to `127.0.0.1` behind nginx keeps it off the network. An option for later is a root worker process plus an unprivileged API, split the way Gogobackup's `cmd/cron` is.
- Email when a folder's images start failing, through the existing mail module. Not in v1 unless asked.
