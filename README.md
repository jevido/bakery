# Bakery

Bakery is a self-hosted platform for deploying applications, databases and
services on your own servers, in the spirit of [Coolify](https://coolify.io),
built on rootless Podman, Caddy, Goravel, Svelte 5 and Postgres.
`services/api` is the source of truth and drives Podman and Caddy;
`apps/web` is the dashboard.

## Layout

| Directory   | What goes there                                                  |
| ----------- | ---------------------------------------------------------------- |
| `apps/`     | Things a person opens and interacts with (sites, apps, CLIs).    |
| `services/` | Things that run unattended (APIs, workers, schedulers).          |
| `packages/` | Code shared by two or more units. Not before.                    |
| `infra/`    | Environments (`dev`, `next`, `prod`) and images. No app code.    |
| `docs/`     | The domain model: glossary, context map, one doc per context.    |

Each unit is one directory under its category (`apps/<name>`,
`services/<name>`) with its own `README.md` and tooling.

## Getting started

Requirements: Go 1.27, [bun](https://bun.sh), [Task](https://taskfile.dev),
git, and rootless [Podman](https://podman.io) with its API socket enabled for
your user:

```sh
systemctl --user enable --now podman.socket
```

Then:

```sh
task dev        # Postgres, the API (:4910, migrates first) and the dashboard (:4930)
task down       # stop the dev servers (Postgres and the proxy keep running)
task check      # format, lint, test and type-check every unit
task api:test:podman   # integration tests against the Podman socket
task images     # build localhost/bakery-api:dev and localhost/bakery-web:dev
task server:test       # install into a fake server container and check it end to end
```

A `services/api/.env` from before the admin API split needs
`BAKERY_PROXY_ADMIN_URL=http://127.0.0.1:4949` and
`BAKERY_PROXY_ADMIN_PUBLISH=127.0.0.1:4949` added (see `.env.example`).

On start the API creates the `bakery` Podman network and the `bakery-proxy`
Caddy container (127.0.0.1:4940 HTTP, 127.0.0.1:4943 HTTPS, admin API on
127.0.0.1:4949).

### Deploy your first application

1. Open <http://127.0.0.1:4930>. The first visit asks you to create the
   owner account.
2. Create a project. It comes with a `production` environment.
3. Add an application, for example `https://github.com/traefik/whoami`,
   branch `master`, port `80`. Its domain defaults to `whoami.localhost`.
4. Optionally add environment variables, then press **Deploy**. The build log
   streams live on the Deployments tab.
5. Open <https://whoami.localhost:4943>. `*.localhost` resolves to your
   machine in browsers and curl.

The certificate comes from Caddy's internal CA. To stop the browser warning,
trust its root:

```sh
podman exec bakery-proxy cat /data/caddy/pki/authorities/local/root.crt > bakery-root.crt
# then import bakery-root.crt into your browser or system trust store
```

### Build packs

Each application picks how it becomes an image:

- **Dockerfile**: builds the Dockerfile at a path in the repository.
- **Nixpacks**: no Dockerfile needed; [Nixpacks](https://nixpacks.com)
  detects the language (Node, Go, Python, PHP, Ruby, Rust, …) and Bakery
  builds the plan it writes. `task api:dev` downloads the pinned `nixpacks`
  binary into `services/api/bin/`.
- **Static site**: serves a directory of the repository (the publish
  directory, `.` by default) as files on port 80.
- **Image**: runs a prebuilt image from a registry, e.g.
  `ghcr.io/traefik/whoami:v1.10`, optionally with a registry username and
  token for private images. Every deploy pulls it again; a rollback starts
  the exact image pulled then.

`task buildpack:test` runs every build pack against the Forgejo stand-in and
its container registry.

### Private repositories

Use the repository's SSH URL, for example `git@github.com:you/app.git` or
`ssh://git@git.example.com:2222/you/app.git`. Bakery generates a deploy key
for the application; copy the public key from the **Source** tab and add it
to the repository as a read-only deploy key (GitHub: Settings → Deploy keys),
then press **Deploy**. The first clone from a host trusts its SSH host key;
later clones must see the same key. If a host's key changes on purpose,
forget it under **Settings → Known hosts**.

### Auto-deploy on push

The **Source** tab also shows the application's webhook: a URL and a secret.
Add them as a webhook in the repository (content type `application/json`,
push events). Every push to the application's branch then starts a
deployment, marked `webhook` in the list next to its commit. The git host
must be able to reach the URL, so this works on a server, or locally with
the Forgejo stand-in (`task git:test` runs the whole flow against it).

### Health checks, cancel and rollback

Under **General → Health check**, give the path your app answers on when it
is ready (e.g. `/health`). Bakery then requests it inside the new container
(with `curl` or `wget`, so the image needs one of them) and only moves
traffic once it answers 2xx or 3xx; a redeploy then serves every request. A
check that never passes fails the deployment and the running version stays.
An active deployment has a **Cancel** button, and every earlier finished one
a **Roll back** button that starts its image again without cloning or
building (`task deploy:test` runs all of this against the Forgejo stand-in).

### Variables

**Environment variables** are runtime only by default; tick **Build** to
hand one to the Dockerfile's `ARG` too (it then ends up in the image's
history, so keep secrets runtime only). **Shared variables** on the project
page, for the whole project or one environment, reach every application in
it; an application's own variable wins over its environment's, which wins
over the project's.

### Databases and backups

**New → Database** in an environment runs PostgreSQL, MySQL, MariaDB,
Redis, Valkey or MongoDB with generated credentials; applications reach it
on its internal URL, and a public port opens it from outside the server.
Its **Backups** tab backs up PostgreSQL, MySQL, MariaDB and MongoDB on a
schedule or with **Back up now**, to the server's disk and to an S3 storage
added under **Settings → S3 storages**, keeping the newest few. Any backup
can be downloaded or restored (from S3 when the local file is gone).
`task databases:test` and `task backups:test` (with `task s3:up`, the
Garage S3 stand-in) run all of this end to end.

### Services

**New service** in an environment runs multi-container software in one
step: pick Uptime Kuma, Umami (with its own PostgreSQL), n8n, Gitea or
whoami from the catalog, or paste a `compose.yml` of your own. Passwords
the file asks for as `${SERVICE_PASSWORD_<X>}` or `${SERVICE_USER_<X>}` are
generated once and shown under **Variables**; a service whose environment
names `SERVICE_FQDN_<NAME>_<PORT>` gets a domain with HTTPS on that port,
editable on the Service page, and every other `${VAR:-default}` is a
variable you can set. Compose files may use `image:` services with
`command`, `entrypoint`, `environment`, named volumes, `depends_on`,
`working_dir`, `user` and `labels`; `build:`, `ports:`, bind mounts,
`privileged` and the like are refused with the line they are on. The
Service page starts, stops, redeploys (pulling the images again, keeping
the volumes) and deletes it, and follows each component's logs.
`task services:test` runs all of this end to end.

## Install on a server

On a fresh Linux server with systemd, ports 80/443 open and DNS for your
dashboard domain (and `*.<domain>` for applications) pointing at it:

```sh
sudo bash infra/install/install.sh --domain bakery.example.com --email me@example.com \
  --api-image <registry>/bakery-api:<tag> --web-image <registry>/bakery-web:<tag>
```

It installs rootless Podman, runs Bakery as the `bakery` user and serves
the dashboard on `https://bakery.example.com` with a Let's Encrypt
certificate; applications get certificates the same way. Run it again to
upgrade. Images are not published yet: build them with `task images` and
push them to a registry of your own. See
[`infra/install`](infra/install/README.md) and the runbook in
[`infra/prod`](infra/prod/README.md).

`task server:test` proves the whole install locally, in a fake server
container with Pebble as the ACME CA.

Conventions for people and coding agents live in [`CLAUDE.md`](CLAUDE.md).
The project is built with domain-driven design; the model lives in
[`docs/domain`](docs/domain).
