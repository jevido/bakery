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
