<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/images/logo-wordmark-dark.svg">
    <img src="docs/images/logo-wordmark.svg" alt="Goptimizer" width="420">
  </picture>
</p>

<p align="center">
  Self-hosted image optimization for the folders on your server.
</p>

Goptimizer is a web app that runs on a server and keeps the images of the projects on that server
optimized. You add a folder in the browser. The service finds new and changed images in it on a
schedule, shrinks each one with command-line tools such as `pngquant`, `oxipng`, `jpegoptim` and
`gifsicle`, and replaces the file only when the result is smaller and passes every check.

## Status

Phase 1 of [the plan](docs/PLAN.md): the base. The repository is
[Gogobackup](https://github.com/nicklasos/gogobackup) renamed, with its backup domain removed. What
works today is sign-in, password reset, email verification, user management with roles, and an empty
dashboard. Folders, scanning and optimizing arrive in phases 2 to 4.

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

## Test

`make test` from the root runs the backend's Go unit and integration tests, then the frontend's
typecheck, ESLint, unit tests and the Playwright suite against a real API. Run
`make playwright-browsers` in `frontend/` once before the first run.

## Layout

| Folder | What it is |
|---|---|
| [backend/](backend) | The Go API: Gin, SQLite through sqlc, JWT auth with roles, an in-process scheduler. |
| [frontend/](frontend) | The admin panel: React, TypeScript, Ant Design and TanStack Query. |
| [deploy/](deploy) | Ansible playbooks to provision a server, install on an existing one and release. |
| [docs/](docs) | The plan and the logo. |

[AGENTS.md](AGENTS.md) is the guide for contributors and coding agents. Each folder has its own
`AGENTS.md` or README with its conventions. [deploy/README.md](deploy/README.md) covers deployment.

## Credits

Goptimizer is built on [Gogokit](https://github.com/nicklasos/gogokit), a Go API and React admin panel
starter kit.

The Go gopher was designed by Renée French and is licensed under
[CC BY 4.0](https://creativecommons.org/licenses/by/4.0/).

## License

[WTFPL](LICENSE).
