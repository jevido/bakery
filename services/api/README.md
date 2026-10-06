# api

The Bakery's JSON API: the source of truth for the Members, projects, applications,
deployments, databases, services and servers, and the unit that drives Podman and Caddy. Goravel v1.18 on
Postgres 18, listening on `127.0.0.1:4910`.

## Run

```sh
task db:up                           # from the repo root: dev Postgres on :4920
task api:dev                         # creates .env with fresh keys on first run
curl -s 127.0.0.1:4910/api/health    # {"ok":true}
```

`task api:dev` also runs `task api:tools`, which downloads the pinned
`nixpacks` binary into `bin/` for the nixpacks Build pack
(`BAKERY_NIXPACKS=bin/nixpacks` in `.env`; the API image has it on `PATH`).
`BAKERY_INSECURE_REGISTRIES` lists registries pulled from over plain http;
`.env` lists the Forgejo stand-in's (`127.0.0.1:4950`).
A Database's Public port is published on `BAKERY_DATABASES_PUBLIC_BIND`
(`127.0.0.1` in `.env`; empty, on a server, means every interface), and its
Public URL names `BAKERY_DATABASES_PUBLIC_HOST` (empty: the dashboard domain,
else `localhost`).

Notifications link to the dashboard at `BAKERY_DASHBOARD_URL` (empty:
`https://$BAKERY_DASHBOARD_DOMAIN`, or `http://localhost:4930` without one).
`BAKERY_TELEGRAM_API_URL` (default `https://api.telegram.org`) and
`BAKERY_SERVER_PROBE_INTERVAL` (default `5m`) only change for tests.

Backups of Databases are written to `BAKERY_BACKUPS_DIR` (`storage/backups`
here); S3 storages are tested against the Garage stand-in on
`127.0.0.1:4960` (`task s3:up` from the repo root writes its key to
`infra/dev/state/garage.env`).

Services are run from their Compose file through the Podman API (no
`podman compose`); the template catalog is the YAML files embedded from
`contexts/services/templates/`, one per template.

Servers are the Local server (the Podman socket above) and Remote servers,
reached over SSH with a key The Bakery generates; the Podman API is tunnelled
through that connection (`app/podman/ssh.go`). `task remote:up` from the repo
root starts a stand-in on `127.0.0.1:4972` for `task api:test:podman`,
`task servers:test` and `task remote-deploy:test`. Other contexts reach a
Server through `servers.Connect` (one pooled SSH connection per Server);
an Application's Deployments run on its Target server, served by that
Server's own Proxy, whose admin API is a unix socket opened over SSH.

Members sign in with a Session cookie (the dashboard) or send an API token
as `Authorization: Bearer bky_…` (scripts; made under Keys & Tokens in the
dashboard or `POST /api/api-tokens` with `{name, permissions,
expires_in_days}`; Coolify's permissions `root`, `write`, `deploy`, `read`
and `read:sensitive` limit what a token may do, and `{name, read_only}`
still works). Each Member has a Role (owner, admin,
member, viewer) that `identity.Auth`, `identity.Admin` and
`identity.Secrets` enforce on every route; admins invite people with
`POST /api/invitations`, which answers a link that is good once, for 7
days. `task access:test` checks it all end to end.

Other tasks: `task api:check` (gofmt, go vet, go test), `task api:migrate`,
`task api:test:podman` (against the rootless Podman socket),
`task api:test:s3` (the S3 client against Garage).
Artisan runs as `go run . artisan ...`. `go run . artisan
identity:reset-two-factor <email>` switches a Member's two-factor
authentication off (the Owner's included) and signs them out, for someone
who lost their phone and their recovery codes. Profile and two-factor are
under `/api/me` (Session only) and `POST /api/login/two-factor`; `task
account:test` checks them end to end.
Previews of pull requests are under `/api/applications/{id}/previews`
and switched on with `PATCH /api/applications/{id}/webhook`; `task
previews:test` checks their life cycle end to end against Forgejo.

## Layout

- `contexts/<name>/` holds each bounded context; see
  [`contexts/README.md`](contexts/README.md) for the layers inside one.
- `routes/api.go` mounts every context's routes under `/api`.
- `config/`, `bootstrap/`, `database/migrations/` are Goravel's.
