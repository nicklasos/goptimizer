# Manual install

Installing by hand on a server that already has Go, Node.js, nginx and supervisor,
and usually other applications too. `ansible-playbook install.yml` does exactly this for you (see
[README.md](README.md)); this page is for doing it, or checking it, without Ansible.

The files to copy are in [`examples/`](examples): `backend.env`, `supervisor.conf` and `nginx.conf`.
The migrations create the SQLite database in `shared/database/`.

Once, as root:

```bash
git clone git@github.com:nicklasos/goptimizer.git /var/www/goptimizer
cd /var/www/goptimizer

mkdir -p shared/database /var/log/goptimizer
chown www-data: shared/database

cp backend/.env.example backend/.env        # then apply deploy/examples/backend.env
chgrp www-data backend/.env && chmod 640 backend/.env
printf 'VITE_API_URL=/api/v1\nVITE_APP_NAME="Goptimizer"\n' > frontend/.env

make deploy SUPERVISOR_PROGRAM=             # first build and migrations; nothing to restart yet

cp deploy/examples/supervisor.conf /etc/supervisor/conf.d/goptimizer.conf
supervisorctl update

cp deploy/examples/nginx.conf /etc/nginx/sites-available/goptimizer.conf
ln -s /etc/nginx/sites-available/goptimizer.conf /etc/nginx/sites-enabled/
nginx -t && systemctl reload nginx

cd backend && ./bin/goptimizer-cli create-user --email you@example.com --password "a-long-password" --admin
```

`VITE_API_URL=/api/v1` makes the panel call its own origin, where nginx forwards `/api` to the API.
The examples assume the API on port 8747, the user `www-data` and Cloudflare in front; change the port in
both `backend/.env` and `nginx.conf`.

## Releasing

```bash
cd /var/www/goptimizer && make deploy
```

It pulls, builds the API and the CLI, applies new migrations, builds the panel and restarts `goptimizer-api`.
The API runs the scheduled jobs itself (`ENABLE_SCHEDULER=true`). To run them as their own process, set it to false, build `bin/goptimizer-cron` and add a second supervisor program for it.
