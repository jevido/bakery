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

Requirements: [Task](https://taskfile.dev), [Podman](https://podman.io), plus
whatever each unit's `README.md` lists.

```sh
task            # list tasks
task dev        # run every unit's dev server
task check      # format, lint, test and type-check every unit
task down       # stop every dev server this repo started
```

Conventions for people and coding agents live in [`CLAUDE.md`](CLAUDE.md).
The project is built with domain-driven design; the model lives in
[`docs/domain`](docs/domain).
