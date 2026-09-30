# databases

- Subdomain: supporting
- Hosted in: `services/api` (module `contexts/databases`)

## Purpose

Runs one-click Databases: a PostgreSQL, MySQL, MariaDB, Redis, Valkey or
MongoDB in an Environment, reachable by Applications on the `bakery`
network and, when the Owner wants, from outside on a Public port. Owns the
Database, its Container and its volume. It is **not** responsible for
Applications or their Deployments, for Projects and Environments (projects
owns those), and not yet for backups.

## Language

| Term | Meaning |
| ---- | ------- |
| Database | One Engine running as the Container `bakery-db-<slug>`, its data in the volume `bakery-db-<id>-data`. |
| Engine | `postgresql`, `mysql`, `mariadb`, `redis`, `valkey` or `mongodb`, with its image, port, data path and readiness probe. |
| Database version | The image tag of the Engine, e.g. `18-alpine`. |
| Database credentials | Generated username, password, root password (MySQL/MariaDB) and database name. |
| Internal URL | The connection URL on the `bakery` network (host `bakery-db-<slug>`). |
| Public port | Host port the Database is published on; none by default. |
| Public URL | The connection URL through the Public port on the public host. |
| Desired state | `running` or `stopped`: what the Owner asked for. |
| Database status | What the Container is doing, read from Podman: `starting`, `running`, `stopped`, `exited`, `missing`. |
| Readiness probe | The Engine's own client run inside the Container (`pg_isready`, `mysqladmin ping`, `redis-cli ping`, `mongosh` ping); `running` means it passed. |
| Recreate | Stop and remove the Container and create it again from the current settings, with the same volume. |

## Model

### Aggregates

| Aggregate | Invariants |
| --------- | ---------- |
| Database | Belongs to one Environment of one Project. Its Engine and Database credentials are fixed at creation; credentials are generated, never typed. Name 1–100 characters; Slug unique among Databases. Database version is a valid image tag. Public port is none or 1024–65535 and unique among Databases. Resource limits: memory 16–65536 MB, CPU 0.1–64 cores, empty is unlimited. Desired state is `running` or `stopped`. |

### Commands

- `CreateDatabase(environment, name, engine, version?, public port?, limits?)`: generate credentials, store, start.
- `UpdateDatabase(name, version, public port, limits)`: store; if version, Public port or limits changed and it should run, Recreate.
- `StartDatabase`, `StopDatabase`, `RestartDatabase`: set the desired state and act on the Container.
- `DeleteDatabase`: remove the Container, then the volume, then the row.
- `Recover()`: at API start, start every Database whose desired state is `running` and whose Container is missing.

### Domain events

None published yet.

## Integration

- **Publishes:** the Databases API (`/api/environments/{id}/databases`,
  `/api/projects/{id}/databases`, `/api/databases/{id}` and its actions and
  log stream) for the dashboard.
- **Consumes:** `projects.Environment(id)` to place a new Database (and
  learn its Project), translated into its own `Environment`; registers
  `projects.OnProjectDeleting`, answering "in use" while the Project has
  Databases.

## Why it's shaped this way

- **Its own context, not part of deployments or projects.** A Database has
  no Source, no build, no Route and no Deployments; its lifecycle is
  start/stop/restart. Putting it in deployments would bend the core model
  around something it does not do, and putting it in projects would give
  projects a Podman runtime. A Database only references its Environment and
  Project by id (no foreign key, like `routes`). Cost: the Project page asks
  two contexts for its contents.
- **projects decides whether a Project can be deleted, and a Project with
  Databases cannot be.** projects asks every registered in-use check before
  deleting and refuses with the same error as for Applications. Deleting a
  Project never cascades into Databases: silently deleting data is worse
  than a refusal.
- **The Container name is the hostname.** `bakery-db-<slug>` on the
  `bakery` network is stable across restarts and Recreates, so the Internal
  URL an Application was given never changes. The `bakery-db-` prefix keeps
  a Database's slug from colliding with an Application's Container.
- **Any change to the Container recreates it.** Podman cannot change the
  published ports or limits of an existing container; the data is in the
  volume, so a Recreate costs seconds of downtime, which the dashboard
  says before saving.
- **Status is read from Podman, only the desired state is stored.** Stored
  status goes stale the moment a Container crashes or the host reboots.
  The API asks Podman (and the readiness probe) whenever it shows a
  Database. Restart policy `always` and `podman-restart.service` bring
  Databases back after a reboot; `Recover` covers a Container that is gone.
- **Readiness is probed inside the Container** with the Engine's own
  client through the Podman exec API, as Health checks are: in development
  the API runs on the host and cannot reach the `bakery` network, and the
  image already carries the right client.
- **Credentials are generated and shown.** The Owner needs them to connect,
  so unlike Registry credentials they are returned by the API; at rest they
  are encrypted like Env vars. Redis and Valkey get their password from an
  environment variable read by `sh -c`, so `podman inspect` shows no secret
  on the command line.
- **One volume per Database, removed only with the Database.** Postgres
  keeps its data in `PGDATA=/var/lib/postgresql/data/pgdata` on a volume
  mounted at `/var/lib/postgresql/data`, which works for version 18 (whose
  image moved its default data path) and older alike.
- **The Public port is published on a configured address**
  (`BAKERY_DATABASES_PUBLIC_BIND`: every interface on a server, `127.0.0.1`
  in development). Bakery does not manage a firewall; a port another
  process holds fails the start with Podman's reason.
