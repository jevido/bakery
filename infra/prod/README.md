# prod

Production: one Linux server running Bakery, installed and upgraded with
[`infra/install/install.sh`](../install/install.sh). Everything runs as
rootless Podman containers of the `bakery` user on the `bakery` network.

## Prerequisites

- A Linux server with systemd (Fedora/RHEL or Debian/Ubuntu), reachable on
  ports 80 and 443.
- DNS: an A/AAAA record for the dashboard domain, and one for every
  application domain (applications default to `<name>.<dashboard domain>`,
  so a wildcard record `*.<dashboard domain>` covers them).
- Images `bakery-api` and `bakery-web` in a registry the server can pull
  from. None are published yet: build them with `task images`, push them
  yourself and pass `--api-image`/`--web-image`.

## Install and upgrade

```sh
sudo bash install.sh --domain bakery.example.com --email me@example.com \
  --api-image <registry>/bakery-api:<tag> --web-image <registry>/bakery-web:<tag>
```

Running the same command again upgrades: it pulls the images and recreates
`bakery-api`, `bakery-web` and `bakery-proxy`, keeping the database,
secrets and certificates.

**Roll back** by running it again with the previous image tags. Migrations
only move forward, so a rollback across a migration needs a database
restore (see Backups).

## Resources

| Container | Image | Port (on the `bakery` network) | Health | Volumes |
| --------- | ----- | ------------------------------ | ------ | ------- |
| `bakery-proxy` | `docker.io/library/caddy:2` | 80, 443 published on the host; admin API 2019 not published | serves `https://<domain>/` | `bakery-proxy-data` (certificates), `bakery-proxy-config` |
| `bakery-api` | `bakery-api` | 4910 | `GET /api/health` → `{"ok":true}` | `bakery-backups` (Backup files of Databases, at `/var/lib/bakery/backups`); mounts the Podman socket |
| `bakery-web` | `bakery-web` | 80 | `GET /` | none |
| `bakery-postgres` | `docker.io/library/postgres:18` | 5432 | `pg_isready -U bakery` | `bakery-postgres` |

The API creates `bakery-proxy` itself on start. Every container has
`--restart=always`; `podman-restart.service` (user unit of `bakery`) starts
them at boot, and linger keeps them running without a login.

## Configuration

Secrets, generated once by the install script, live in
`/var/lib/bakery/bakery.env` (mode 600): `APP_KEY`, `JWT_SECRET`,
`DB_PASSWORD`. The install script passes everything else to `bakery-api`:

`APP_URL`, `DB_HOST`, `DB_PORT`, `DB_DATABASE`, `DB_USERNAME`,
`BAKERY_NETWORK`, `BAKERY_DOMAIN_SUFFIX`, `BAKERY_DASHBOARD_DOMAIN`,
`BAKERY_PROXY_HTTP_PORT`, `BAKERY_PROXY_HTTPS_PORT`,
`BAKERY_PROXY_INTERNAL_TLS=false`, `BAKERY_PROXY_ADMIN_URL`,
`BAKERY_ACME_CA`, `BAKERY_ACME_EMAIL`, `BAKERY_ACME_CA_ROOT`,
`BAKERY_BACKUPS_DIR=/var/lib/bakery/backups`. Their meaning
is in `services/api/.env.example`. `BAKERY_NIXPACKS` and
`BAKERY_INSECURE_REGISTRIES` keep their defaults: the image's own `nixpacks`,
and TLS verified for every registry.

## Operating it

Run Podman as the `bakery` user:

```sh
sudo -u bakery XDG_RUNTIME_DIR=/run/user/$(id -u bakery) podman ps
sudo -u bakery XDG_RUNTIME_DIR=/run/user/$(id -u bakery) podman logs -f bakery-api
```

(`cd /var/lib/bakery` first if your working directory is not readable by
`bakery`.)

## Backups

**Databases** (the one-click ones on the dashboard) are backed up from
their page: on a schedule or with Back up now. Every Backup is written to
the `bakery-backups` volume, mounted on `bakery-api` at
`/var/lib/bakery/backups`, as `<database id>/<UTC timestamp>.<ext>`, and
uploaded to an S3 storage (added under Settings) when the Database's
schedule names one. Retention prunes both. Deleting a Database removes its
Backups from the volume; copies in S3 storage stay, on purpose.

A copy on the same server does not survive losing the server: name an S3
storage in the schedule, or copy the files off yourself. They can be
downloaded from the dashboard, or read from the volume:

```sh
sudo -u bakery XDG_RUNTIME_DIR=/run/user/$(id -u bakery) \
  podman volume inspect bakery-backups --format '{{.Mountpoint}}'
```

Redis and Valkey are not backed up.

**Bakery's own database** (`bakery-postgres`) is not backed up
automatically yet. By hand:

```sh
sudo -u bakery XDG_RUNTIME_DIR=/run/user/$(id -u bakery) \
  podman exec bakery-postgres pg_dump -U bakery bakery > bakery-$(date +%F).sql
```

Restore into a fresh database with `psql -U bakery bakery < dump.sql` inside
`bakery-postgres`. Keep a copy of `/var/lib/bakery/bakery.env` too: without
`APP_KEY` the encrypted env vars, database passwords and S3 secret keys in
the dump cannot be read.
