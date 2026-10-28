<p align="center">
  <img src="public/favicon.svg" alt="Gogobackup" width="72">
</p>

<h1 align="center">Gogobackup admin panel</h1>

<p align="center">
  The React admin panel of Gogobackup, for the Go API in <a href="../backend">../backend</a>.<br>
  Dashboard, backup plans, storages, runs, restores and users, typed end to end and covered by tests.
</p>

<p align="center">
  <img alt="React" src="https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=black">
  <img alt="TypeScript" src="https://img.shields.io/badge/TypeScript-strict-3178C6?logo=typescript&logoColor=white">
  <img alt="Vite" src="https://img.shields.io/badge/Vite-7-646CFF?logo=vite&logoColor=white">
  <img alt="Ant Design" src="https://img.shields.io/badge/Ant%20Design-5-0170FE?logo=antdesign&logoColor=white">
  <img alt="TanStack Query" src="https://img.shields.io/badge/TanStack%20Query-5-FF4154?logo=reactquery&logoColor=white">
  <img alt="Playwright" src="https://img.shields.io/badge/E2E-Playwright-2EAD33?logo=playwright&logoColor=white">
</p>

<p align="center">
  <a href="#quick-start">Quick start</a> ·
  <a href="#features">Features</a> ·
  <a href="#screenshots">Screenshots</a> ·
  <a href="#adding-a-module">Add a module</a> ·
  <a href="ROADMAP.md">Roadmap</a>
</p>

![The dashboard: one card per backup plan](../docs/images/dashboard.png)

---

The [root README](../README.md) covers running and deploying the whole service. This file is about working on the panel.

## Quick start

### Prerequisites

- Node.js 22+
- A running [gogo](https://github.com/nicklasos/gogo) API with at least one user

### Setup

```bash
git clone git@github.com:nicklasos/gogo-front.git my-admin && cd my-admin

cp .env.example .env    # VITE_API_BASE_URL points at gogo, http://localhost:8181 by default
npm install
npm run dev             # http://localhost:5173
```

Create the first super admin in the gogo repo, then sign in with it:

```bash
make cli-create-user EMAIL=admin@example.com PASSWORD=password123
```

### Tests

```bash
make playwright-browsers   # once
make test                  # typecheck, lint, unit tests, Playwright
```

The E2E run starts its own gogo API against the test database and its own Vite server, so it needs the gogo repo next to this one (`../backend`) and `TEST_DATABASE_PATH` in `../backend/.env`.

## Features

- **Auth**: email/password login, automatic token refresh, session kept across reloads and tabs
- **Password reset and email verification**: forgot-password, reset and confirm pages for the emailed links, and a "try again in N minutes" message when the API throttles sign-in
- **Policies**: rules (`canManageUser(actor, target)`) that mirror the API's and decide what the UI offers
- **Agent recipes**: step-by-step procedures in `docs/recipes/` for the common changes, also available as Claude Code skills
- **Roles**: `super-admin`, `admin`, `user`, with route guards and role-filtered menus
- **Navigation**: a top bar with the logo, the sections and a user menu (profile, user management, sign out); on phones the sections move into a drawer
- **User management**: one page from the user menu, with a tab per role the signed-in user may manage
- **Profile**: name, email, language and password
- **Backup plans**: an editor with a source picker, a pasted connection URL filled into the fields, schedule rows with a live next-run line, retention switches and a connection test
- **Storages**: a server-paginated list and an editor with a kind picker, a connection test and an unsaved-changes guard, as the pattern to copy
- **Generated API types**: `make api-types` turns the backend's OpenAPI file into `src/api/schema.ts`
- **Typed API client**: `ApiError`, server validation errors mapped onto form fields, translated error keys
- **Resilient**: a page that crashes shows an error screen inside the shell instead of blanking the app, unknown URLs get a not-found page, and every page is a lazily loaded chunk
- **Responsive**: tables become cards and the primary action becomes a bottom bar on phones
- **i18n**: English and Ukrainian
- **Docker and CI**: an nginx image that serves the build and proxies `/api`, and a GitHub Actions workflow that runs the whole suite
- **Tests**: unit tests on `node:test`, Playwright E2E with page objects, everything in TypeScript

## Screenshots

| A plan's runs | The plan editor |
|---|---|
| ![A plan's runs](../docs/images/plan-runs.png) | ![The plan editor](../docs/images/plan-editor.png) |

| A restore | On a phone |
|---|---|
| ![A restore](../docs/images/restore.png) | ![The dashboard on a phone](../docs/images/mobile-dashboard.png) |

`node ../scripts/screenshots/shoot.mjs` regenerates them.

## Make targets

```bash
make help
make typecheck
make lint            # typecheck + eslint
make test-unit       # node:test
make test-e2e        # Playwright (starts gogo --test-db + Vite on the E2E ports)
make test            # everything
make build
make api-types       # regenerate src/api/schema.ts from ../backend/docs/swagger.json
make api-types-check # fail if it is out of date (CI runs this)
make playwright-browsers
```

## Env

| Variable | Purpose |
|----------|---------|
| `VITE_API_BASE_URL` | gogo API origin, e.g. `http://localhost:8181` (the client appends `/api/v1`) |
| `PORT` | Dev server port (default `5173`) |
| `TEST_FRONTEND_PORT` | E2E Vite port (default `5175`) |
| `TEST_BACKEND_PORT` | E2E gogo port (default `8184`) |
| `VITE_E2E_API_BASE_URL` | API base used by E2E helpers (default `http://localhost:$TEST_BACKEND_PORT/api/v1`) |
| `TEST_DATABASE_PATH` | gogo's SQLite test database for E2E seeding, relative to `../backend`; the file name must end in `_test.db`. Read from `../backend/.env` unless set here |

The E2E ports are specific to this project so Playwright never attaches to another project's servers. Change them when you copy the skeleton.

## Project structure

```
src/
  app/        App, providers, queryClient, router, theme, i18n
  api/        client (fetch + refresh-on-401), ApiError, shared types, query keys
  auth/       authStore (persist key: gogo-auth), roles, guards, LoginPage
  layout/     AppShell, nav registry
  shared/     components, hooks, utils, lib/unsavedChanges
  features/   dashboard, plans, storages, users, account
  locales/{en,uk}/
tests/e2e/    Playwright specs, page objects, helpers
```

## Adding a module

Follow [docs/recipes/add-feature.md](docs/recipes/add-feature.md), or [full-stack-feature.md](docs/recipes/full-stack-feature.md) when the API side is new too. In short: `src/features/<module>/{types,api,hooks,policy}.ts` and `pages/`, query keys, a route, a menu entry and locale keys. `features/storages` is the reference.

## API contract (gogo)

- Success: `{ "data": ... }`; paginated lists: `{ "data": [...], "pagination": { total, current_page, last_page, per_page } }`
- Errors: `{ "error_key", "message", "status" }`; validation: `{ "error_key": "validation.failed", "errors": { "field": ["validation.field.rule"] } }`
- `/auth/login` returns `{ access_token, refresh_token, user }`; `user` is `{ id, email, name, roles }`

## Docker

```bash
docker build -t gogo-front .
docker run -p 8080:80 -e API_UPSTREAM=http://host.docker.internal:8181 gogo-front
```

The image is the production build behind nginx. With `VITE_API_BASE_URL` left empty at build time the app calls its own origin, and nginx forwards `/api` and `/health` to `API_UPSTREAM`.

## CI

`.github/workflows/ci.yml` checks out this repository and the backend side by side and runs `make api-types-check`, `make test` and the build. The backend repository is named in the workflow; change it when you copy the skeleton.

## Roadmap

See `ROADMAP.md`.
