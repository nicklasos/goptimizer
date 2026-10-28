# Goptimizer – Agent Guide

Goptimizer is a self-hosted service that keeps the images in folders on a server optimized.
[docs/PLAN.md](docs/PLAN.md) is the product plan and lists the phases. It is built on
[Gogokit](https://github.com/nicklasos/gogokit): a Go API, a React admin panel and server provisioning
in one repository.

## Layout

| Folder | What it is |
|---|---|
| `backend/` | Go API: Gin, SQLite through sqlc, an in-process cache and scheduler, JWT auth, admins and users |
| `frontend/` | Admin panel: React, TypeScript, Ant Design, TanStack Query |
| `deploy/` | Ansible: provision a server or install on an existing one, deploy and update the application |

Each folder has its own `AGENTS.md` (or README) with its conventions. Read the one for the folder you are working in.

This project owns all three folders: edit them directly. There is no sync with the Gogokit template.

## Working across the folders

- The contract between `backend/` and `frontend/` is the OpenAPI file: after changing request or response
  types run `make swagger` in `backend/`, then `make api-types` in `frontend/`.
- A feature that touches both follows `frontend/docs/recipes/full-stack-feature.md`. The backend recipes
  are in `backend/docs/recipes/`.
- The frontend's E2E suite starts the API from `../backend`, so `make test` in `frontend/` tests both halves together.
- `deploy/` reads nothing from the other folders except their structure: `backend/cmd/{api,cron,cli}`,
  `backend/migrations` and `frontend/dist`. Renaming any of those means updating `deploy/roles/` and
  `deploy/install.yml`.

## Commands

```bash
make help            # everything below
make up              # local Mailpit
make cli migrate up  # a console command; flags go after -- (make cli -- create-user --email a@b.c --password secret123)
make test            # backend tests, then frontend typecheck, lint, unit and Playwright
make provision       # set up a new server (see deploy/README.md)
make install-remote  # add the app to a server that already has everything (deploy/install.yml)
make deploy-remote   # release the current code to the server with Ansible
make deploy          # run on the server (install.yml or by hand): pull, build, migrate, restart
node scripts/screenshots/shoot.mjs   # regenerate the README screenshots after a UI change (demo data from backend/cmd/screenshots-seed)
node scripts/logo/build.mjs          # regenerate the logo files from docs/images/logo.svg
```

## Rules

- No narrating comments. A comment says why, not what.
- Do not start dev servers; the owner runs them.
- Finish a change with the tests of the folder it touched, and `ansible-playbook --syntax-check`
  for anything under `deploy/`.
- Never commit `deploy/inventory/hosts.yml`, `deploy/group_vars/all/secrets.yml` or any `.env`.
