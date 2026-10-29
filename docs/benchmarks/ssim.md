# SSIM threshold

**Verdict.** The default `min_ssim` is **0.85**, for every format, on the lowest SSIM of any 64×64
tile of the image. PNG and GIF are scored on each of red, green and blue, composited on black and
on white, and the worst counts. JPEG is scored on luma. On the 96 images below, every result with
visible damage scored 0.793 or less, and no result above 0.85 showed any. Some results under 0.85
look fine too, such as an aerial photo at 0.689: the threshold refuses them, on the side of keeping
the original.

A lossy result under the threshold is not the end for the image: the optimizer then runs the
lossless pipeline and replaces the file only if every pixel is the same. On this corpus that
fallback shrinks all 22 PNGs whose lossy result was refused, by 4.3 MB of 19.8 MB (22%).

## What the score is

`verify.Compare` scores a lossy result with SSIM (Wang et al., 2004) over 8×8 windows at every
position, at full resolution, and keeps two numbers: the mean over the image and the lowest mean
of a 64×64 tile. The check uses the tile. The mean hides local damage: one banded shadow in a
screenshot moved the mean from 1 to 0.97 while its tile scored 0.30.

The channels differ by format because the damage does:

- **PNG and GIF.** Palette quantization changes colors that luma does not see. Light blue tags
  that became gray score 0.985 on luma and 0.768 per channel. Each channel is composited on
  black and on white first, so a pixel that changes its alpha changes its color and counts.
- **JPEG.** A re-encode moves the color channels at the scale of chroma subsampling, which the eye
  does not resolve. Per channel, 24 of 24 JPEG photos that look the same as their originals scored
  between 0.44 and 0.99, which leaves no room for a threshold. On luma they score 0.875 or more.

Downscaling before scoring was not tried: the tile score already needs every pixel to see a
banded shadow 100 pixels wide.

## Corpus

96 images, copied into a scratch directory, never committed. Nothing comes from a personal
folder.

| Kind | PNG | JPEG | GIF | Source |
|---|---|---|---|---|
| Photos | 2 | 24 | | `/System/Library/Wallpapers` (5.6 MB aerial, also as a 2048 px JPEG and a 1200 px PNG), Photos and Automator resources, Numbers textures (paper, granite, wood) |
| Screenshots | 11 | | | `docs/images` of this repo, onboarding screenshots of an app in `/Applications` |
| UI art | 8 | | | Illustrated cards and backgrounds from app bundles in `/Applications` |
| Illustrations with gradients | 12 | 6 | | macOS desktop pictures (`iMac Yellow`, `Sonoma`, `Radial Sky Blue` and others, converted from HEIC by `sips` at 1600 px), banners from app bundles |
| Transparent PNGs | 12 | | | Icons and masks from `/System/Library/CoreServices` |
| Animated and still GIFs | | | 13 | App bundles, Tk and Ruby resources, Go's `image/testdata` |
| Generated | 5 | 2 | 1 | `ssimcal -generate`: gray 0 to 255, dark gray 16 to 48 (the classic banding case), a sky, a radial gradient, an alpha ramp, two of them as JPEG, an animated gradient |

## Results

The lossy pipeline with the default profile (pngquant `--quality 65-80` then oxipng, jpegoptim
`--max=82`, gifsicle `-O3 --lossy=30`), scored as above.

| Kind | Images | Lowest score | Median | Refused for metadata | Pass at 0.85 |
|---|---|---|---|---|---|
| Photos, JPEG | 24 | 0.875 | 0.965 | 0 | 24 |
| Illustrations, JPEG | 6 | 0.939 | 0.984 | 0 | 6 |
| Generated, JPEG | 2 | 0.990 | 1.000 | 0 | 2 |
| Screenshots, PNG | 11 | 0.303 | 0.768 | 0 | 3 |
| UI art, PNG | 8 | 0.347 | 0.635 | 2 | 1 |
| Illustrations, PNG | 12 | 0.021 | 0.389 | 7 | 0 |
| Transparent, PNG | 12 | 0.602 | 0.907 | 9 | 1 |
| Photos, PNG | 2 | 0.689 | 1.000 | 0 | 1 |
| Generated, PNG | 5 | 0.466 | 1.000 | 0 | 3 |
| GIF | 14 | 0.958 | 0.996 | 0 | 9 |

JPEG saved 69% (7.4 of 10.7 MB). Every GIF that gifsicle shrank by at least 3% passed; three more
were refused before scoring, two because `-O3` merged identical frames (91 frames became 81, 145
became 108) and one that Go's decoder cannot read.

## Where damage starts

Each result below was looked at next to its original, at 1:1, on the 192-pixel square around its
worst tile (`ssimcal` writes these crops).

| Score | Image | What changed |
|---|---|---|
| 0.303 | screenshot, blurred leaderboard | A soft round shadow became visible rings |
| 0.466 | generated radial gradient | Smooth color became blotchy dither |
| 0.479 | desktop picture `iMac Yellow` | Contour lines in the gradient |
| 0.494 | `docs/images/dashboard.png` | The light blue "Lossy" tag turned gray |
| 0.570 | desktop picture `iMac Green` | Dither texture on the gradient |
| 0.642 | illustrated card | Blotchy noise on fabric |
| 0.676 | `docs/images/settings.png` | The pale pink "Missing" tag turned gray |
| 0.689 | aerial photo as PNG | No visible change |
| 0.768 | `docs/images/users.png` | The light blue "Admin" tags turned gray |
| 0.790 | Pages welcome screen | No visible change |
| 0.793 | `docs/images/folder-editor.png` | The faint shadow under the Save button is gone |
| 0.831 | folder icon | Faint dither on flat blue, hard to see |
| 0.864 | `docs/images/mobile-dashboard.png` | No visible change |
| 0.866 | app preview screenshot | No visible change |
| 0.875 | paper texture, JPEG | Grain differs pixel by pixel, looks the same |
| 0.877 | noise fill, JPEG | Grain differs pixel by pixel, looks the same |

The highest score with damage a reader would notice is 0.768, and with any damage 0.793 to 0.831.
The lowest score that looks the same in every case above it is 0.864. 0.85 sits between, on the
side of refusing: when in doubt the original stays. The aerial photo at 0.689 shows that the score
also refuses results that look fine; for those the lossless fallback still applies.

## What SSIM does not see

Banding without dither scores high. A gray gradient from 0 to 255, 1024 pixels wide, cut to 32
levels has visible steps every 32 pixels and scores 0.917 on its worst tile:

| Gradient | 64 levels | 32 levels | 16 levels | 8 levels |
|---|---|---|---|---|
| Gray 0 to 255, worst tile | 0.973 | 0.917 | 0.816 | 0.732 |
| Dark gray 16 to 48, worst tile | 0.992 | 0.972 | 0.922 | 0.752 |

pngquant does not produce that case on its own: the generated gray gradients fit its 256-color
palette exactly, and on the sky gradient its `--quality 65` floor refused to quantize, so only the
lossless step ran. A result banded that way by another tool would pass.

## Metadata, not SSIM

18 of 50 PNG results were refused before scoring because a chunk changed. On macOS, pngquant 3.0.3
strips metadata by default (`--strip` is "default on Mac"), so it dropped `iCCP`, `cICP` and `gAMA`.
On Linux it does not strip by default; the Linux behavior was not measured. oxipng 10.2.1 replaces
a standard sRGB ICC profile with an `sRGB` chunk and drops `iCCP` from an image it reduces to gray;
the check refuses both, so those PNGs stay as they are.

## Rerun

```bash
cd backend
go run ./cmd/ssimcal -corpus <dir of kind directories> -out <dir> -generate > ssim.tsv
go run ./cmd/ssimcal -corpus <dir> -out <dir> -mode lossless > lossless.tsv
go run ./cmd/ssimcal -compare original.jpg result1.jpg result2.jpg
```

The TSV is sorted by score. `-out` holds every result and, in `crops/`, the original and the result
side by side around the worst tile. Measured on an Apple M2, macOS 26.6.2, with pngquant 3.0.3,
oxipng 10.2.1, jpegoptim 1.5.6 and gifsicle 1.98 from Homebrew.
