# Optimizer throughput

**Verdict.** On 96 real and generated images (70 MB: photos, screenshots, illustrations, icons and
animated GIFs), the optimizer handles **0.85 images per second with 1 worker, 2.48 with 4 and 2.75
with 7**, median of 5 alternated runs each. The tools use 92% of the CPU time; Go's decoding and SSIM
the rest. From 4 workers to 7 the rate grows by 11%, because 4 workers already keep 5.3 cores busy:
oxipng runs several threads per image, and the machine has 8 cores and other load.

Measured on macOS, on a laptop that was busy with other work. A Linux server was not measured.

## What runs

`backend/cmd/optbench` copies the corpus into a fresh folder for every run, with a day-old mtime so
`min_age_minutes` lets every file through, migrates a fresh SQLite database, sets
`settings.workers`, scans the folder in full, then starts the real worker pool, as the scheduler
does, and stops the clock when no image is pending or optimizing.

```bash
cd backend && go build -o /tmp/optbench ./cmd/optbench
/tmp/optbench -corpus <dir> -work <scratch dir> -workers 1,4,7 -runs 5
/tmp/optbench -corpus <dir> -work <scratch dir> -workers 1 -runs 1 -log debug   # every image with how long it took
```

- **Corpus.** The 96 images of the SSIM calibration ([ssim.md](ssim.md)): 50 PNG, 32 JPEG and 14 GIF,
  69.8 MB, among them a 5.6 MB aerial photo and four GIFs of 1.8 to 6.5 MB with 100 frames or more.
- **Settings.** The default profile (lossy) and the default `nice`: the tools run under `nice -n 10`.
  macOS has no `ionice`.
- **Rounds.** 1, 4 and 7 workers, in that order, five times over, so drift and the machine's other
  load fall on every count alike.
- **CPU** is user plus system time of the benchmark process and of the tools it waited for
  (`getrusage` of self and children), over the run.

## Machine

Apple M2 (4 performance and 4 efficiency cores), 16 GB, macOS 26.6.2, Go 1.25.4, pngquant 3.0.3,
oxipng 10.2.1, jpegoptim 1.5.6, gifsicle 1.98 from Homebrew. The one-minute load average was 7.5
when the runs started, from processes that could not be stopped (Spotlight, other development
servers), and 21 at the end, when a test run of the same session overlapped the last round.

## Results

| Workers | Runs | Median images/s | Range | Median wall time | CPU time | CPU per wall second |
|---|---|---|---|---|---|---|
| 1 | 5 | 0.85 | 0.83 to 0.87 | 112.3 s | 182 to 185 s | 1.63 |
| 4 | 5 | 2.48 | 2.27 to 2.51 | 38.7 s | 201 to 207 s | 5.26 |
| 7 | 5 | 2.75 | 2.52 to 2.81 | 35.0 s | 211 to 215 s | 6.10 |

Every run ended the same way, which is the check that every run did all the work: 78 images
optimized, 13 kept, 5 skipped, 69,782,498 bytes before and 52,028,565 after (25% saved), and no
image failed.

## What limits it

**The tools, on the CPU.** In every run the tools took 92% of the CPU time (168 to 197 s of 182 to
215 s). With 7 workers the run used 6.1 cores for its whole length, on a machine of 8 cores with
other load, so it was bound by cores, not by the pool, the claim or SQLite.

**Why 4 workers already get most of it.** One worker keeps 1.6 cores busy, because oxipng compresses
with several threads. Four keep 5.3 busy. Three more workers can add at most the 2 or 3 cores that
are left, and they add less, because the CPU time of the same work grows with contention, from 183 s
with 1 worker to 213 s with 7.

**Where the time goes.** With one worker and the debug log, the slowest images are the large PNGs
whose lossy result was refused and which then went through the lossless pipeline as well: 7.4 s for
a 1.9 MB card, 4.7 to 5.9 s for the 1600-pixel desktop pictures. No GIF took more than 3.4 s,
the 6 MB ones included. The longest single image is a fifth of the 7-worker run, so the order of the claims does not
set the wall time.

**What it is not.** The claim is one write transaction of a few index seeks; on this corpus it runs
96 times in a run of 35 s. Go's own work, decoding both images, the SSIM and the color check, is the
8% of CPU time the tools do not use.

## What it means for a server

On a server like this, with the default of one worker per core but one, expect about 2.5 images a
second while folders are being worked through for the first time, about 9,000 an hour, and a
backlog of 100,000 images in about 11 hours. `nice` keeps the sites on the same server ahead of the
tools, so the rate drops when the sites are busy. Fewer workers leave more cores to the sites at
little cost in rate: 4 workers did 90% of what 7 did.
