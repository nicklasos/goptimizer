<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/images/logo-wordmark-dark.svg">
    <img src="docs/images/logo-wordmark.svg" alt="Goptimizer" width="420">
  </picture>
</p>

<p align="center">
  Self-hosted image optimization for every project on your server.<br>
  Add a folder, and Goptimizer keeps its images small, TinyPNG-style, on a schedule.
</p>

<p align="center">
  <a href="https://github.com/nicklasos/goptimizer/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/nicklasos/goptimizer/actions/workflows/ci.yml/badge.svg"></a>
  <img alt="Go version" src="https://img.shields.io/github/go-mod/go-version/nicklasos/goptimizer?filename=backend%2Fgo.mod&style=flat">
  <img alt="React 19" src="https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=black&style=flat">
  <img alt="SQLite" src="https://img.shields.io/badge/SQLite-one%20file-003B57?logo=sqlite&logoColor=white&style=flat">
  <a href="LICENSE"><img alt="License: WTFPL" src="https://img.shields.io/badge/license-WTFPL-lightgrey?style=flat"></a>
  <img alt="Self-hosted" src="https://img.shields.io/badge/self--hosted-yes-2ea44f?style=flat">
</p>

> [!WARNING]
> **Goptimizer is in early development and does not optimize images yet.** Today it has sign-in, user
> management, folders and the settings page with the image tools. Scanning and optimizing are being built
> in the order of the [roadmap](#roadmap).
> Screenshots come with the dashboard.

## Why

One server often hosts many projects, and each one collects images that nobody optimized: uploads from
users, photos straight from a phone, screenshots, generated thumbnails. A cron job per project with a few
`pngquant` and `jpegoptim` lines works until there are a dozen of them, each slightly different, and no
one knows how much they save or whether they still run.

Goptimizer replaces those scripts with one service and a web page. You add each project's image folder
once. The service finds new and changed images on its own, shrinks them with the best command-line tools
installed on the server, and shows per folder how many images it handled and how many bytes it saved.

## How it works

- **Folders.** Add any folder on the server from the browser, pause it, or change its settings. Each
  folder has its own scan interval and compression profile, and one folder never sits inside another.
- **TinyPNG-like compression.** By default PNGs go through `pngquant` palette quantization (the same
  approach as TinyPNG) and then `oxipng`. JPEGs are re-encoded by `jpegoptim` with a quality cap. GIFs go
  through `gifsicle`. Any folder can switch to lossless, where every pixel stays the same.
- **Fast scans of huge folders.** A quick scan checks only each directory's modification time and lists
  just the directories that changed, so a folder with a million images costs about one `stat` per
  directory when nothing is new. A full scan, once a day by default, catches files overwritten in place.
- **No overlapping work.** Scans and images are rows in SQLite, not jobs in memory. A folder has at most one
  scan at a time, so an hourly check never starts on top of a long first run. One pool of workers takes
  images from every folder in turn, so a first run of millions of images does not hold up a small folder's
  new uploads. After a restart, everything carries on where it stopped.
- **Light on the server.** You choose how many images are optimized at once. The tools run under `nice`
  and `ionice` by default, so the sites on the same server stay fast.
- **Tools page.** The settings page lists every tool Goptimizer can use, whether it is installed, its
  version, and the command that installs it.
- **Statistics.** Each folder shows its images by status, the bytes saved, the last scans and a 30-day
  chart.

Local folders come first. SSH servers, Amazon S3 and Google Cloud Storage are planned as more kinds of
folder.

## Safety

Goptimizer replaces images **in place and keeps no copy of the original**. Every result has to pass all of
these checks first, or the original stays as it was:

| Check | Why |
|---|---|
| Tools only ever work on a copy in a work directory | A killed or failed tool cannot truncate the original |
| The result decodes in the same format, with the same width, height and frame count | No broken or cropped images |
| Lossless results have exactly the same pixels | Lossless means lossless |
| Lossy results stay above a similarity (SSIM) threshold | Catches banding on gradients and similar visible damage |
| The result is at least 3% smaller | No rewrite for nothing |
| EXIF orientation and ICC color profiles are kept | Phone photos stay upright and wide-gamut photos keep their colors |
| Files changed in the last 5 minutes are left for later, and each file is checked again just before the swap | An upload in progress or a newer version is never overwritten |
| The swap is an atomic rename, with the original's owner, group, mode and modification time | Web servers see the old file or the new one, never half of one, and can still read it |
| Animated PNGs, symlinks, hard-linked files and files whose content does not match their extension are skipped | The tools would damage them, or the change would reach outside the folder |
| An image is never compressed lossily twice | No slow loss of quality over repeated runs |

Still, lossy compression changes pixels by design. Try a folder on a copy first if its images matter.

## Roadmap

| # | Phase | Status |
|---|---|---|
| 1 | Base: sign-in, users, deploy, on [Gogokit](https://github.com/nicklasos/gogokit) | Done |
| 2 | Folders and the tools page | Done |
| 3 | Scanner: quick and full scans, measured on a million files | Next |
| 4 | Optimizer: pipelines, verification, the shared worker pool | Planned |
| 5 | Dashboard: folder cards, statistics, screenshots in this README | Planned |
| 6 | Deploy: image tools installed by the playbooks | Planned |

The design, the data model and the reasoning behind each decision are in [docs/PLAN.md](docs/PLAN.md).

## Requirements

- A Linux server. The playbooks target Ubuntu 22.04 and 24.04.
- The image tools: `pngquant`, `oxipng` (or `optipng`), `jpegoptim` and `gifsicle`. On Ubuntu:
  `sudo apt install pngquant optipng jpegoptim gifsicle`; Ubuntu has no `oxipng` package, and `optipng`
  does its work. The settings page shows which ones the API finds.
- Goptimizer runs as root, because it rewrites files that belong to other users and gives each one back its
  owner and mode. nginx serves the admin panel, and phase 6 binds the API to `127.0.0.1` so only nginx
  can reach it.

## Install on a server

Three commands, with Ansible from your machine. [deploy/README.md](deploy/README.md) explains the settings.

```bash
make provision        # a fresh server: packages, Go, Node.js, nginx, TLS, then the app
make install-remote   # a server that already runs other apps: only the app
make deploy-remote    # release new code: pull, build, migrate, restart, health check
```

On the server itself, `make deploy` does the same release without Ansible.

## Run it locally

You need Go 1.25 or newer, Node.js 22 or newer, and Make.

```bash
cp backend/.env.example backend/.env
cp frontend/.env.example frontend/.env

make cli migrate up             # creates backend/data/goptimizer.db
make seed                       # admin@example.com / password123, and a few more accounts

cd backend && make run          # API on http://localhost:8181
cd frontend && npm install && npm run dev   # in a second terminal: http://localhost:5173
```

Sign in at http://localhost:5173 as `admin@example.com` with the password `password123`. The API
documentation is at http://localhost:8181/swagger/index.html. With `APP_DEBUG=true`, as in
`.env.example`, emails go to the API's log.

## Tests

`make test` runs the backend's Go unit and integration tests, then the frontend's typecheck, ESLint, unit
tests and the Playwright suite against a real API. Run `make playwright-browsers` in `frontend/` once
before the first run.

## Project layout

| Folder | What it is |
|---|---|
| [backend/](backend) | Go API: Gin, SQLite through sqlc, JWT auth with roles, an in-process scheduler |
| [frontend/](frontend) | Admin panel: React, TypeScript, Ant Design, TanStack Query |
| [deploy/](deploy) | Ansible: provision a server, install on an existing one, release |
| [docs/](docs) | The plan and the logo |

## Contributing

Issues and pull requests are welcome. [AGENTS.md](AGENTS.md) is the guide for contributors and coding
agents, and each folder has its own `AGENTS.md` with its conventions. A change ends with `make test`
green, and `ansible-playbook --syntax-check` for anything under `deploy/`.

## Credits

Built on [Gogokit](https://github.com/nicklasos/gogokit), a Go API and React admin panel starter kit.

The Go gopher was designed by Renée French and is licensed under
[CC BY 4.0](https://creativecommons.org/licenses/by/4.0/). The logo is a gopher drawn after it.

## License

[WTFPL](LICENSE).
