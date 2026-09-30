# api

Bakery's JSON API: the source of truth for the Owner, projects, applications
and deployments, and the unit that drives Podman and Caddy. Goravel v1.18 on
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

Other tasks: `task api:check` (gofmt, go vet, go test), `task api:migrate`.
Artisan runs as `go run . artisan ...`.

## Layout

- `contexts/<name>/` holds each bounded context; see
  [`contexts/README.md`](contexts/README.md) for the layers inside one.
- `routes/api.go` mounts every context's routes under `/api`.
- `config/`, `bootstrap/`, `database/migrations/` are Goravel's.
