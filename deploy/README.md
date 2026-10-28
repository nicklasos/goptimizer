# Deploy

Ansible playbooks that turn a fresh Ubuntu server into a running deployment, and update it afterwards.

| Playbook | What it does |
|---|---|
| `provision.yml` | Everything, from an empty server: packages, firewall, Go, Node.js, the application, nginx, TLS, and the plan that backs up Gogobackup itself. Safe to run again. |
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
  shared/backups/              archives of the plan "Gogobackup itself" (self_backup_dir)
/var/log/<app>/                api.log, api-error.log
/etc/supervisor/conf.d/<app>.conf
/etc/nginx/sites-available/<app>.conf
```

- **Processes**: supervisor runs `<app>-api` as the unprivileged `deploy` user. The API runs the scheduled jobs and the backups itself. Set `scheduler_in_api: false` to run them in a separate `<app>-cron` program. `runner_concurrency` (default 2) is how many backups and restores run at once.
- **Dump tools**: `sqlite3`, `pg_dump` and `pg_restore` (`postgresql-client`), `mysqldump` and `mysql` (`default-mysql-client`) from the distribution. `install_postgres_client: false` or `install_mysql_client: false` leaves one out. See [Dump tools](#dump-tools).
- **nginx**: `domain` serves the admin panel and proxies `/api/` and `/health` to the API on `127.0.0.1:8181`. An optional `api_domain` proxies everything to the API.
- **SQLite** is a file the API opens directly; there is no database server. The firewall allows SSH, 80 and 443.
- **Its own backup**: every deploy runs `gogo-cli self-backup`, which makes sure the plan "Gogobackup itself" backs up the database to `shared/backups/`. See [Backing up Gogobackup itself](#backing-up-gogobackup-itself).

## Requirements

- On your machine: Ansible 2.15 or newer (`brew install ansible` or `pipx install --include-deps ansible`).
- A server with Ubuntu 22.04 or 24.04 that you can reach over SSH as a user who can `sudo`.
- DNS for `domain` (and `api_domain`) pointing at the server **before** provisioning when `tls_mode` is `letsencrypt`: the certificate request needs it.

## First deployment

```bash
cd deploy
ansible-galaxy collection install -r requirements.yml

cp inventory/hosts.example.yml inventory/hosts.yml          # the server's address and SSH user
cp secrets.example.yml group_vars/all/secrets.yml           # jwt_secret: openssl rand -hex 32, encryption_key: openssl rand -base64 32
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

Secrets (`group_vars/all/secrets.yml`): `jwt_secret`, `encryption_key`, `mail_password`.

`encryption_key` encrypts the storage credentials (S3 keys, GCS service-account keys) in the database.
Keep a copy somewhere other than the server, next to your other secrets. If it is lost or changed, the
stored credentials cannot be decrypted: every S3 and GCS storage fails its connection test until its keys
are entered again. Local storages keep working.

### TLS modes

| Mode | What nginx does | Use it when |
|---|---|---|
| `letsencrypt` | certbot gets and renews the certificates; port 80 redirects to 443 | The server faces the internet directly |
| `cloudflare` | Listens on port 80 and restores the visitor's IP from `CF-Connecting-IP` | Cloudflare is in front and terminates TLS |
| `none` | Plain HTTP on port 80 | A test server |

The Cloudflare address ranges are in `roles/nginx/defaults/main.yml`; refresh them from <https://www.cloudflare.com/ips/> when they change.

### Backing up Gogobackup itself

After migrations, `provision.yml`, `deploy.yml` and `install.yml` run
`./bin/gogo-cli self-backup --dir {{ self_backup_dir }}` as the API user. On the first run it creates
a local storage, "Gogobackup itself (local)", in `self_backup_dir` (default `/var/www/<app>/shared/backups`),
and a plan, "Gogobackup itself", that backs up the SQLite database every day at 03:15 UTC and keeps
30 days of archives, then 8 weekly and 12 monthly ones. Later runs find the plan by the database it
backs up and change nothing, so rename it, reschedule it or change its retention in the panel and
the next deploy leaves it as it is. `self_backup_enabled: false` stops the task, for example after you
delete the plan on purpose: otherwise the next deploy creates it again.

Two things the local copy does not cover:

- **It is on the same disk.** A lost server takes the database and its archives with it. Add an S3 or
  GCS storage under **Storages** and tick it in the "Gogobackup itself" plan for a copy off the server.
- **`encryption_key` is not in the archive.** A restored database still holds the storage credentials
  sealed with it, and they cannot be opened without it. Keep `encryption_key` somewhere off the server,
  such as a password manager or an encrypted `secrets.yml` (`ansible-vault`).

To restore the database from an archive (`<plan>-<time>-<run>.db.gz`), as the API user (`deploy`, or `www-data` after `install.yml`):

```bash
sudo supervisorctl stop <app>-api                     # and <app>-cron, if scheduler_in_api is false
cd /var/www/<app>/shared/database
sudo -u deploy sh -c 'gunzip -c /path/to/archive.db.gz > <app>.db.restore && sqlite3 <app>.db.restore "PRAGMA integrity_check"'
sudo -u deploy sh -c 'rm -f <app>.db-wal <app>.db-shm && mv <app>.db.restore <app>.db'
sudo supervisorctl start <app>-api
```

Put the same `encryption_key` in `secrets.yml` (or `ENCRYPTION_KEY` in `backend/.env`) as the server the archive came from.

### Restores

A restore runs on this server too, with the restore program of its source: `pg_restore`, `mysql`, or none
for SQLite and files. It works in `work_dir` (`WORK_DIR`, default `<app_root>/shared/work`), one directory per
restore, removed after it ends.

- **Disk space.** A PostgreSQL restore downloads the whole archive into `work_dir` first, because
  `pg_restore --jobs` reads it in parallel from a file, so `work_dir` needs room for the largest PostgreSQL
  archive. A files restore downloads its archive there too, to check it before writing anything, and
  extracts into `work_dir/restores/` unless the admin names another directory. A SQLite restore writes
  next to the target file and keeps the replaced one as `<file>.before-restore-<time>`. MySQL downloads first too, so a damaged archive never reaches `mysql`.
- **Speed.** `restore_jobs` (`RESTORE_JOBS`, default 4) is how many parallel jobs `pg_restore` runs; up
  to the number of cores helps a large database.
- `pg_restore` must be at least as new as the `pg_dump` that made the archive, and the target server at
  least as new as the one dumped; the restore fails saying which. The PGDG client below covers both.
- A restore and a backup of the same plan never run at once: whichever is queued second waits.

### Dump tools

Backups of a PostgreSQL or MySQL plan run `pg_dump` or `mysqldump` on this server, as the API user,
and a SQLite plan runs `sqlite3`. A plan's connection test in the admin panel shows the version it finds.

`pg_dump` refuses to dump a server of a newer major version than itself, and the run fails saying so.
Ubuntu 24.04's `postgresql-client` is 16 and 22.04's is 14, so a PostgreSQL 17 server needs the
client from the PostgreSQL apt repository (PGDG), which installs beside the distribution's:

```bash
sudo install -d /usr/share/postgresql-common/pgdg
sudo curl -fsSL -o /usr/share/postgresql-common/pgdg/apt.postgresql.org.asc https://www.postgresql.org/media/keys/ACCC4CF8.asc
echo "deb [signed-by=/usr/share/postgresql-common/pgdg/apt.postgresql.org.asc] https://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" | sudo tee /etc/apt/sources.list.d/pgdg.list
sudo apt-get update && sudo apt-get install -y postgresql-client-17
```

A newer client dumps an older server without trouble, so the newest one serves every plan. Ubuntu's
`/usr/bin/pg_dump` is a wrapper that picks among the installed clients; check which one runs with
`sudo -u <api user> pg_dump --version`, or with a plan's connection test. The playbooks do not add the repository.

## Day-to-day on the server

```bash
sudo supervisorctl status                       # is it running
sudo supervisorctl restart <app>-api
sudo tail -f /var/log/<app>/api.log             # application log (JSON)
sudo -u deploy sh -c 'cd /var/www/<app>/backend && ./bin/gogo-cli help'
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
cp secrets.example.yml group_vars/all/secrets.yml           # jwt_secret, encryption_key
ansible-playbook install.yml                                # or, from the project root: make install-remote
```

- The API runs as `www-data` (`install_user`) and runs the scheduled jobs and the backups itself (`scheduler_in_api: true`, the default).
- `install.yml` installs no packages, so the dump tools the plans need (`sqlite3`, `pg_dump`, `mysqldump`, and `pg_restore`, `mysql` for restores) must already be on the server: see [Dump tools](#dump-tools).
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
- **Restoring Gogobackup's own database from its panel.** The panel restores every other plan, but refuses to overwrite the database it runs on. Restore that one by hand: see [Backing up Gogobackup itself](#backing-up-gogobackup-itself).
