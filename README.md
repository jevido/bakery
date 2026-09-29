# Project name

> TODO: one or two sentences on what this project is and who it is for.

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

## Using this starter

1. Create a repository from this template.
2. Replace the title and TODO above, and the "What this is" section in
   `CLAUDE.md`.
3. Name the core domain and first bounded contexts in
   `docs/domain/context-map.md`, and start the glossary.
4. Copy `docs/domain/contexts/_template` for each context.
5. Add the first unit that hosts them, and the stack rules for it in
   `CLAUDE.md`.
