# Manual install

Installing by hand on a server that already has Go, Node.js, nginx and supervisor,
and usually other applications too. `ansible-playbook install.yml` does exactly this for you (see
[README.md](README.md)); this page is for doing it, or checking it, without Ansible.

The files to copy are in [`examples/`](examples): `backend.env`, `supervisor.conf` and `nginx.conf`.
The migrations create the SQLite database in `shared/database/`.

Once, as root:

```bash
git clone git@github.com:nicklasos/gogobackup.git /var/www/gogobackup
cd /var/www/gogobackup

mkdir -p shared/database /var/log/gogobackup
chown www-data: shared/database

cp backend/.env.example backend/.env        # then apply deploy/examples/backend.env
chgrp www-data backend/.env && chmod 640 backend/.env
printf 'VITE_API_BASE_URL=\nVITE_APP_NAME="Gogobackup"\n' > frontend/.env

make deploy SUPERVISOR_PROGRAM=             # first build and migrations; nothing to restart yet

cp deploy/examples/supervisor.conf /etc/supervisor/conf.d/gogobackup.conf
supervisorctl update

cp deploy/examples/nginx.conf /etc/nginx/sites-available/gogobackup.conf
ln -s /etc/nginx/sites-available/gogobackup.conf /etc/nginx/sites-enabled/
nginx -t && systemctl reload nginx

cd backend && ./bin/gogo-cli create-user --email you@example.com --password "a-long-password"

# The plan "Gogobackup itself": the database every day at 03:15 UTC, into shared/backups
install -d -o www-data -g www-data -m 750 /var/www/gogobackup/shared/backups
sudo -u www-data ./bin/gogo-cli self-backup --dir /var/www/gogobackup/shared/backups
```

`self-backup` is safe to run again: it finds the plan by the database it backs up and leaves it as it is.
The archives share the server's disk, so add an S3 or GCS storage to the plan in the panel, and keep
`ENCRYPTION_KEY` somewhere off the server ([README.md](README.md#backing-up-gogobackup-itself)).

The empty `VITE_API_BASE_URL` makes the panel call its own origin, where nginx forwards `/api` to the API.
The examples assume the API on port 8181, the user `www-data` and Cloudflare in front; change the port in
both `backend/.env` and `nginx.conf`.

## Releasing

```bash
cd /var/www/gogobackup && make deploy
```

It pulls, builds the API and the CLI, applies new migrations, builds the panel and restarts `gogobackup-api`.
The API runs the scheduled jobs and the backups itself (`ENABLE_SCHEDULER=true`). To run them as their own process, set it to false, build `bin/gogo-cron` and add a second supervisor program for it.

Backups run the dump programs of their sources on this server: `sqlite3`, `pg_dump` (`apt install postgresql-client`)
and `mysqldump` (`apt install default-mysql-client`). `pg_dump` must be at least the major version of the
PostgreSQL server it dumps; [README.md](README.md#dump-tools) shows how to get a newer one. Restores use `pg_restore` and `mysql` from the same packages, and
`WORK_DIR` (default `./data/work`, see [Restores](README.md#restores)), which needs room for the largest PostgreSQL archive.
