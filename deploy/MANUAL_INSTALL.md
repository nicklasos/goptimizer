# Manual install

Installing by hand on a server that already has Go, Node.js, nginx and supervisor,
and usually other applications too. `ansible-playbook install.yml` does exactly this for you (see
[README.md](README.md)); this page is for doing it, or checking it, without Ansible.

The files to copy are in [`examples/`](examples): `backend.env`, `supervisor.conf` and `nginx.conf`.
The migrations create the SQLite database in `shared/database/`.

Once, as root. First the image tools. Ubuntu packages all of them but `oxipng`, which comes from its
release on GitHub; without it the lossless PNG step runs `optipng`.

```bash
apt install pngquant jpegoptim optipng gifsicle

v=10.2.1
build=oxipng-$v-$(uname -m)-unknown-linux-musl       # uname -m is x86_64 or aarch64
curl -fsSLO https://github.com/oxipng/oxipng/releases/download/v$v/$build.tar.gz
sha256sum $build.tar.gz
#   x86_64   1813750ef592c5350ca79c88f98b2c0876d05c826dc7156245256d4255c1ad17
#   aarch64  ff21aa2a005b324c8dfbdf13cd2f469fb6728b27dacded82635824ecd12e3b35
tar -C /opt --no-same-owner -xzf $build.tar.gz && rm $build.tar.gz
ln -sf /opt/$build/oxipng /usr/local/bin/oxipng
```

Compare the sum with the one for your architecture before the `tar` line. Then the application:

```bash
git clone git@github.com:nicklasos/goptimizer.git /var/www/goptimizer
cd /var/www/goptimizer

mkdir -p shared/database /var/log/goptimizer /var/lib/goptimizer/work
chmod 700 shared/database /var/lib/goptimizer/work

cp backend/.env.example backend/.env        # then apply deploy/examples/backend.env
chmod 600 backend/.env
printf 'VITE_API_URL=/api/v1\nVITE_APP_NAME="Goptimizer"\n' > frontend/.env

make deploy SUPERVISOR_PROGRAM=             # first build and migrations; nothing to restart yet

cp deploy/examples/supervisor.conf /etc/supervisor/conf.d/goptimizer.conf
supervisorctl update

cp deploy/examples/nginx.conf /etc/nginx/sites-available/goptimizer.conf
ln -s /etc/nginx/sites-available/goptimizer.conf /etc/nginx/sites-enabled/
nginx -t && systemctl reload nginx

cd backend && ./bin/goptimizer-cli create-user --email you@example.com --password "a-long-password" --admin
```

The service runs as root, so it can rewrite any project's images and give each one back its owner
and mode. It listens on `127.0.0.1` only (`BIND_ADDRESS`), behind nginx. The work dir must be
root's alone, mode 0700, or the API refuses to start.

`VITE_API_URL=/api/v1` makes the panel call its own origin, where nginx forwards `/api` to the API.
The examples assume the API on port 8747 and Cloudflare in front; change the port in
both `backend/.env` and `nginx.conf`.

## Releasing

```bash
cd /var/www/goptimizer && make deploy
```

It pulls, builds the API and the CLI, applies new migrations, builds the panel, restarts `goptimizer-api`
and waits until it answers `/health`.
The API runs the scheduled jobs itself (`ENABLE_SCHEDULER=true`). To run them as their own process, set it to false, build `bin/goptimizer-cron` and add a second supervisor program for it, also as root.
