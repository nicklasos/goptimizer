# Deploy

Ansible playbooks that turn a fresh Ubuntu server into a running deployment, and update it afterwards.

| Playbook | What it does |
|---|---|
| `provision.yml` | Everything, from an empty server: packages, firewall, Go, Node.js, the application, nginx and TLS. Safe to run again. |
| `deploy.yml` | Releases new code: pull, build, migrate, restart, health check. |
| `install.yml` | Adds the application to a server that already has everything. Installs no packages. |

A server that already runs other applications gets `install.yml` instead: see [On a server that already has everything](#on-a-server-that-already-has-everything).

> These playbooks pass Ansible's syntax check and the nginx configuration they generate has been
> validated with `nginx -t` for every TLS mode. They have **not yet been run against a real server**.
> Do the first run on a server you can throw away.

## What ends up on the server

```
/var/www/<app>/
  repo/                        the git checkout
  backend  -> repo/backend     Go API; binaries in backend/bin
  frontend -> repo/frontend    admin panel; nginx serves frontend/dist
  shared/database/<app>.db     the SQLite database, outside the checkout
/var/log/<app>/                api.log, api-error.log
/etc/supervisor/conf.d/<app>.conf
/etc/nginx/sites-available/<app>.conf
```

- **Processes**: supervisor runs `<app>-api` as the unprivileged `deploy` user. The API runs the scheduled jobs itself. Set `scheduler_in_api: false` to run them in a separate `<app>-cron` program.
- **nginx**: `domain` serves the admin panel and proxies `/api/` and `/health` to the API on `127.0.0.1:8181`. An optional `api_domain` proxies everything to the API.
- **SQLite** is a file the API opens directly; there is no database server. The firewall allows SSH, 80 and 443.

## Requirements

- On your machine: Ansible 2.15 or newer (`brew install ansible` or `pipx install --include-deps ansible`).
- A server with Ubuntu 22.04 or 24.04 that you can reach over SSH as a user who can `sudo`.
- DNS for `domain` (and `api_domain`) pointing at the server **before** provisioning when `tls_mode` is `letsencrypt`: the certificate request needs it.

## First deployment

```bash
cd deploy
ansible-galaxy collection install -r requirements.yml

cp inventory/hosts.example.yml inventory/hosts.yml          # the server's address and SSH user
cp secrets.example.yml group_vars/all/secrets.yml           # jwt_secret: openssl rand -hex 32
$EDITOR group_vars/all/main.yml                             # app_name, domain, repo_url, tls_mode, ...

ansible-playbook provision.yml
```

Two things the first run will tell you about:

1. **Repository access.** If the clone fails, the playbook prints a public key. Add it to the repository as a read-only deploy key (GitHub: Settings → Deploy keys) and run the playbook again.
2. **The first account.** When the application is up and has no users, the playbook prints the command that creates the first super admin.

`hosts.yml` and `secrets.yml` are ignored by git. To keep the secrets in the repository anyway, encrypt them with `ansible-vault encrypt group_vars/all/secrets.yml` and add `--ask-vault-pass` to the commands.

## Releasing new code

Push to `repo_branch`, then:

```bash
ansible-playbook deploy.yml        # or, from the project root: make deploy-remote
```

It rebuilds only what changed, applies new migrations, restarts the API and waits for `/health` to answer. If the API does not come back healthy, the play fails and says so.

## Settings

All in `group_vars/all/main.yml`, each with a comment. The ones you will set for every project:

| Variable | Meaning |
|---|---|
| `app_name` | Short name: database file, log folder, supervisor programs, nginx site |
| `repo_url`, `repo_branch` | Where the code comes from |
| `domain`, `api_domain` | Hostnames. `api_domain` is optional |
| `tls_mode` | `letsencrypt`, `cloudflare` or `none` (below) |
| `letsencrypt_email` | Required for `letsencrypt` |
| `mail_host` and friends | SMTP, for password reset and verification emails |

Secrets (`group_vars/all/secrets.yml`): `jwt_secret`, `mail_password`.

### TLS modes

| Mode | What nginx does | Use it when |
|---|---|---|
| `letsencrypt` | certbot gets and renews the certificates; port 80 redirects to 443 | The server faces the internet directly |
| `cloudflare` | Listens on port 80 and restores the visitor's IP from `CF-Connecting-IP` | Cloudflare is in front and terminates TLS |
| `none` | Plain HTTP on port 80 | A test server |

The Cloudflare address ranges are in `roles/nginx/defaults/main.yml`; refresh them from <https://www.cloudflare.com/ips/> when they change.

## Day-to-day on the server

```bash
sudo supervisorctl status                       # is it running
sudo supervisorctl restart <app>-api
sudo tail -f /var/log/<app>/api.log             # application log (JSON)
sudo -u deploy sh -c 'cd /var/www/<app>/backend && ./bin/goptimizer-cli help'
```

## On a server that already has everything

For a server that already runs other applications, with Go, Node.js, nginx and
supervisor installed. `provision.yml` is not for it: it installs packages and a firewall, replaces
nginx's default site. `install.yml` only adds this application:
clone into `/var/www/<app>` (no `repo/` folder and links), the two `.env` files, build, migrations
(which create the SQLite database in `shared/database/`), one supervisor program and one nginx site.

On your machine:

```bash
cd deploy
cp inventory/hosts.example.yml inventory/hosts.yml          # address, and domain / api_port / tls_mode for this host
cp secrets.example.yml group_vars/all/secrets.yml           # jwt_secret
ansible-playbook install.yml                                # or, from the project root: make install-remote
```

- The API runs as `www-data` (`install_user`) and runs the scheduled jobs itself (`scheduler_in_api: true`, the default).
- `tls_mode` is `cloudflare` or `none`: the site listens on port 80 and certificates are not handled.
- `install_nginx: print` shows the nginx site instead of writing it, to paste into a file nginx already loads, such as `sites-enabled/default`. Add `cloudflare_real_ip: false` when that file already has the `set_real_ip_from` lines.
- root clones the repository, so root on the server needs access to it.
- If the build cannot find `go` or `npm`, add their folder: `-e install_extra_path=/path/to/bin`.

Every release after that, on the server:

```bash
cd /var/www/<app> && make deploy
```

It pulls, builds the API and the CLI, applies new migrations, builds the panel and restarts `<app>-api`.

The same steps as commands to run yourself, without Ansible, are in [MANUAL_INSTALL.md](MANUAL_INSTALL.md).

## What is not handled

- **Several servers.** Everything, database included, is on one machine; SQLite is a local file.
- **Zero-downtime releases.** A deploy restarts the API; requests in flight are finished first, new ones wait a moment.
- **Rollback.** To go back, revert the commit and deploy again. Migrations are not rolled back automatically.
