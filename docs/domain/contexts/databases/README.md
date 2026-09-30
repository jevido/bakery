# databases

- Subdomain: supporting
- Hosted in: `services/api` (module `contexts/databases`)

## Purpose

Runs one-click Databases: a PostgreSQL, MySQL, MariaDB, Redis, Valkey or
MongoDB in an Environment, reachable by Applications on the `bakery`
network and, when the Owner wants, from outside on a Public port. Owns the
Database, its Container and its volume, its Backups and the S3 storages
they are uploaded to. It is **not** responsible for Applications or their
Deployments, for Projects and Environments (projects owns those), or for
backing up Redis and Valkey (see below).

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
| Backup | One logical dump of a Database in the Backups directory, and in an S3 storage when the schedule names one. |
| Backup status | `running`, `succeeded` or `failed` (with a reason). |
| Backup trigger | `manual` (Back up now) or `scheduled`. |
| Backup location | Where a Backup is: `local` (the Backups directory), `s3`, or both. |
| Backup schedule | On/off, a five-field cron expression (UTC), a Retention and an optional S3 storage; one per Database. |
| Retention | How many newest succeeded Backups are kept, 1–100, default 7. |
| Backups directory | Where Backup files are written: `<database id>/<UTC timestamp>.<ext>`. |
| S3 storage | An S3-compatible bucket: endpoint, region, bucket, prefix, access key, secret key (write-only). |
| Restore | Replace a running Database's data with one of its succeeded Backups. |

## Model

### Aggregates

| Aggregate | Invariants |
| --------- | ---------- |
| Database | Belongs to one Environment of one Project. Its Engine and Database credentials are fixed at creation; credentials are generated, never typed. Name 1–100 characters; Slug unique among Databases. Database version is a valid image tag. Public port is none or 1024–65535 and unique among Databases. Resource limits: memory 16–65536 MB, CPU 0.1–64 cores, empty is unlimited. Desired state is `running` or `stopped`. Backup schedule: cron is a valid five-field expression, Retention 1–100, the S3 storage exists, and only an Engine with backups (not Redis, Valkey) has one switched on. |
| Backup | Belongs to one Database. Starts `running` and ends once, `succeeded` or `failed`. At most one Backup or Restore runs per Database at a time. Only a `running` Database is backed up or restored; only a succeeded Backup is restored. |
| S3 storage | Name 1–100, unique. Endpoint is an http(s) URL without a path; bucket follows S3 naming (3–63 lowercase letters, digits, `-`, `.`); region defaults to `us-east-1`; prefix optional. The secret key is never returned. Cannot be deleted while a Backup schedule uses it. |

### Commands

- `CreateDatabase(environment, name, engine, version?, public port?, limits?)`: generate credentials, store, start.
- `UpdateDatabase(name, version, public port, limits)`: store; if version, Public port or limits changed and it should run, Recreate.
- `StartDatabase`, `StopDatabase`, `RestartDatabase`: set the desired state and act on the Container.
- `DeleteDatabase`: remove the Container, then the volume, then the row.
- `DeleteDatabase` also removes its Backup files and rows; objects in S3 storage stay.
- `SetBackupSchedule(database, enabled, cron, retention, s3 storage?)`: validate and store; switching it on remembers when, so it first fires at the next time after that.
- `BackUpDatabase(database, trigger)`: dump the Database inside its Container into the Backups directory, upload to the S3 storage if any, then prune by Retention locally and in S3.
- `RestoreBackup(backup)`: copy the Backup file (downloaded from S3 if the local file is gone) into the Container and run the Engine's restore.
- `DeleteBackup(backup)`: remove its file, its S3 object and its row.
- `CreateS3Storage`, `UpdateS3Storage` (empty secret keeps it), `DeleteS3Storage`, `TestS3Storage` (can Bakery reach the bucket with these keys).
- `Recover()`: at API start, start every Database whose desired state is `running` and whose Container is missing, mark Backups still `running` as failed (interrupted), and start the scheduler, which backs up every Database whose Backup schedule is due, once a minute.

### Domain events

None published yet.

## Integration

- **Publishes:** the Databases API (`/api/environments/{id}/databases`,
  `/api/projects/{id}/databases`, `/api/databases/{id}` and its actions and
  log stream) for the dashboard.
- **Talks to:** S3-compatible storage (AWS S3, Garage, and the like) over
  its HTTP API, for S3 storages.
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
- **Backups live in databases, not in a context of their own.** How to dump
  and restore is Engine knowledge; a separate backups context would need a
  contract as wide as the Engine catalog. S3 storage is an aggregate here
  for now; when a second consumer appears (volume backups, Bakery's own
  database) it moves out and that move is recorded here.
- **Logical dumps, run inside the Container.** `pg_dump -Fc`,
  `mysqldump`/`mariadb-dump --single-transaction` (gzipped by the API) and
  `mongodump --archive --gzip` run through Podman exec with credentials
  from the Container's own environment, so no secret is on a command line;
  the API streams stdout to a file. A logical dump is consistent without
  stopping the Database, and restores across minor versions, which a copy
  of the volume is not.
- **Redis and Valkey are not backed up.** They run with an append-only
  file; restoring one means replacing files under a running server.
  Coolify does not back them up either. The API refuses with a reason.
- **Restore copies the file in, then runs the Engine's restore through
  `sh -c`**, and removes the file afterwards. libpod only takes exec stdin
  over a hijacked connection, which the thin Podman client does not speak.
  Restore is destructive and not preceded by a Backup of its own; the
  dashboard says so and the Owner can press Back up now first. What the
  last Restore did is kept in memory only: an API restart forgets the note,
  never data.
- **Local disk always, S3 optionally.** Every Backup is written to the
  Backups directory first, then uploaded, so a failed upload still leaves
  a Backup to restore from. S3 objects outlive their Database: deleting a
  Database removes its local Backups, not the off-site copies, which are
  there for exactly the mistake of deleting the wrong thing.
- **One Backup schedule per Database, part of the Database.** It is
  settings of the Database, changed with it. Cron in UTC so a server's
  time zone never shifts it. Missed runs while the API was down produce
  one Backup, not a burst. The scheduler runs in the API process and reads
  schedules from the database each minute, rather than Goravel's compiled
  schedule. Cron parsing uses `robfig/cron`, a pure parser, the one
  dependency the domain has outside the standard library.
- **Our own thin S3 client** (Signature V4, path-style, single PUTs up to
  5 GiB), for the same reason as the Podman client: a small dependency
  tree. Path-style URLs work with AWS, Garage and other S3-compatible
  stores alike.
