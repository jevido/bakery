# databases

- Subdomain: supporting
- Hosted in: `services/api` (module `contexts/databases`)

## Purpose

Runs one-click Databases: a PostgreSQL, MySQL, MariaDB, Redis, Valkey or
MongoDB in an Environment, reachable by Applications on the `bakery`
network and, when a member wants, from outside on a Public port. Owns the
Database, its Container and its volume, its Scheduled backups, their Backup
executions and the S3 storages they are uploaded to. It is **not** responsible for Applications or their
Deployments, for Projects and Environments (projects owns those), or for
backing up Redis and Valkey (see below).

## Language

| Term | Meaning |
| ---- | ------- |
| Database | One Database type running as the Container `bakery-db-<slug>`, its data in the volume `bakery-db-<id>-data`. |
| Database type | `postgresql`, `mysql`, `mariadb`, `redis`, `valkey` or `mongodb`, with its image, port, data path and readiness probe. |
| Database version | The image tag of the Database type, e.g. `18-alpine`. |
| Image | Coolify's Image field, `<repository>:<Database version>` with the repository as Docker Hub names it (`postgres`, `valkey/valkey`). |
| Description | Free text about a Database, at most 255 characters, empty for none. |
| Database credentials | Generated username, password, root password (MySQL/MariaDB) and database name. |
| Internal URL | The connection URL on the `bakery` network (host `bakery-db-<slug>`). |
| Public port | Host port the Database is published on; none by default. |
| Public URL | The connection URL through the Public port on the public host. |
| Desired state | `running` or `stopped`: what a member asked for. |
| Database status | What the Container is doing, read from Podman: `starting`, `running`, `stopped`, `exited`, `missing`. |
| Readiness probe | The Database type's own client run inside the Container (`pg_isready`, `mysqladmin ping`, `redis-cli ping`, `mongosh` ping); `running` means it passed. |
| Recreate | Stop and remove the Container and create it again from the current settings, with the same volume. |
| Backup execution | One logical dump of a Database for one of its Scheduled backups, in the Backups directory, and in an S3 storage when that Scheduled backup names one. |
| Execution status | `running`, `succeeded` or `failed` (with a reason). |
| Execution trigger | `manual` (Back up now) or `scheduled`. |
| Execution location | Where a Backup execution is: `local` (the Backups directory), `s3`, or both. |
| Scheduled backup | On/off, a Frequency (a five-field cron expression in UTC, or one of Coolify's shortcuts: `every_minute`, `hourly`, `daily`, `weekly`, `monthly`, `yearly`, with or without `@`), a Retention and an optional S3 storage, with its own Backup executions; any number per Database. |
| Retention | How many of a Scheduled backup's newest succeeded Backup executions are kept, 1–100, default 7. |
| Backups directory | Where Backup execution files are written: `<database id>/<UTC timestamp>.<ext>`. |
| S3 storage | An S3-compatible bucket: endpoint, region, bucket, prefix, access key, secret key (write-only). |
| Restore | Replace a running Database's data with one of its succeeded Backup executions. |

## Model

### Aggregates

| Aggregate | Invariants |
| --------- | ---------- |
| Database | Belongs to one Environment of one Project. Its Database type and Database credentials are fixed at creation; credentials are generated, never typed. Name 1–100 characters; Description at most 255 characters; one created without a name gets `<type>-database-<random>` (8 lowercase letters and digits), as Coolify names it. Slug unique among Databases. Database version is a valid image tag; an Image given instead must name the Database type's repository (with or without `docker.io/` and `library/`, no digest), and its tag becomes the Database version (none is `latest`). Public port is none or 1024–65535 and unique among Databases. Resource limits: memory 16–65536 MB, CPU 0.1–64 cores, empty is unlimited. Desired state is `running` or `stopped`. |
| Scheduled backup | Belongs to one Database, of a Database type with backups (not Redis, Valkey). Cron is a valid five-field expression or shortcut, Retention 1–100, the S3 storage exists and belongs to the Database's Guild. Switching it on remembers when; it first fires at the next time after that. Cannot be deleted while one of its Backup executions runs. |
| Backup execution | Belongs to one Scheduled backup of one Database, and takes that Scheduled backup's S3 storage when it starts. Starts `running` and ends once, `succeeded` or `failed`. At most one Backup execution or Restore runs per Database at a time. Only a `running` Database is backed up or restored; only a succeeded Backup execution is restored. |
| S3 storage | Belongs to one Guild. Name 1–100, unique within the Guild. Endpoint is an http(s) URL without a path; bucket follows S3 naming (3–63 lowercase letters, digits, `-`, `.`); region defaults to `us-east-1`; prefix optional. The secret key is never returned. Cannot be deleted while a Scheduled backup uses it. |

### Commands

- `CreateDatabase(environment, name, type, version?, public port?, limits?)`: generate credentials, store, start.
- `UpdateDatabase(name, description, version or image, public port, limits)`: store; if version, Public port or limits changed and it should run, Recreate.
- `StartDatabase`, `StopDatabase`, `RestartDatabase`: set the desired state and act on the Container.
- `DeleteDatabase(database, delete volumes?)`: remove the Container, then the volume (unless delete volumes is off), then the row.
- `DeleteDatabase` also removes its Backup execution files and rows and its Scheduled backups; objects in S3 storage stay.
- `CreateDatabase` of a Database type with backups also adds its first Scheduled backup: off, `0 3 * * *`, Retention 7.
- `CreateScheduledBackup(database, enabled, cron, retention, s3 storage?)`, `UpdateScheduledBackup(...)`: validate and store; switching it on remembers when, so it first fires at the next time after that.
- `DeleteScheduledBackup(scheduled backup, local?, s3?)`: remove its Backup execution rows with their files (unless local is off) and their S3 objects (only when s3 is on), then it.
- `BackUp(scheduled backup, trigger)`: dump the Database inside its Container into the Backups directory, upload to the Scheduled backup's S3 storage if any, then prune that Scheduled backup's Backup executions by its Retention, locally and in S3.
- `Restore(backup execution)`: copy the Backup execution's file (downloaded from S3 if the local file is gone) into the Container and run the Database type's restore.
- `DeleteBackupExecution(backup execution, s3?)`: remove its file and its row, and its S3 object when s3 is on.
- `CreateS3Storage`, `UpdateS3Storage` (empty secret keeps it), `DeleteS3Storage`, `TestS3Storage` (can The Bakery reach the bucket with these keys).
- `Recover()`: at API start, start every Database whose desired state is `running` and whose Container is missing, mark Backup executions still `running` as failed (interrupted), and start the scheduler, which once a minute starts a Backup execution for every Scheduled backup that is due; one whose Database is busy (another Scheduled backup due at the same time, a Restore) stays due until the next minute.

### Domain events

- `BackupExecutionFinished { backup execution, database, name, type, succeeded, reason,
  trigger, size, off-site }`: a Backup execution ended. Not for one marked failed by
  `Recover` after a restart. Registered with `OnBackupExecutionFinished(f)`; each
  subscriber runs in its own goroutine.

## Integration

- **Publishes:** the Databases API (`/api/environments/{id}/databases`,
  `/api/projects/{id}/databases`, `/api/databases/{id}` and its actions,
  log stream, `scheduled-backups` and `backup-executions`,
  `/api/scheduled-backups/{id}` with its `backup-executions`, and
  `/api/backup-executions/{id}` with its download and restore) for the
  dashboard (S3 storages only the Current guild's: listed, changed and
  tested there, 404 for another Guild's), and `OnBackupExecutionFinished`
  (notifications), carrying the Database's Guild.
- **Talks to:** S3-compatible storage (AWS S3, Garage, and the like) over
  its HTTP API, for S3 storages.
- **Consumes:** `servers.OnServerResources` (a Server's Resources list shows the Local server's Databases with their Container's state, without the readiness probe); `projects.ProjectOf` (through `guilds.InProject`), so every route keyed by an Environment, Project, Database, Scheduled backup or Backup execution answers 404 outside the Current guild or a Project the request may not view, and its Permissions count that Project's overrides (S3 storages are the Guild's, behind `guilds.Owns`); `projects.Environment(id)` to place a new Database (and
  learn its Project and Guild), translated into its own `Environment`; registers
  `projects.OnProjectDeleting`, answering "in use" while the Project has
  Databases.

## Why it's shaped this way

- **The Image field changes only the tag.** Coolify lets any image be
  typed. The Bakery knows each Database type's port, data path, environment,
  readiness probe and dump and restore commands by its image, so another
  repository would run something those do not fit. The repository stays the
  Database type's and the API refuses another with an `image` field error;
  the tag is stored as the Database version, as before.
- **Credentials are read-only on General.** Coolify's Credentials section
  edits the values and asks the person to change them inside the database
  first. The Bakery generates them once and its backups, restores and
  readiness probes rely on them, so General shows them with copy buttons and
  no Save. A viewer sees "Hidden" in their place and in the URLs.
- **Public access is the Public port.** Coolify keeps an `is_public` flag
  and a port and exposes the database through a TCP proxy container with a
  timeout. The Bakery publishes the Container's port on the host, so Access
  Public saves the typed Public port and Private clears it; there is no
  Proxy timeout and no proxy logs. The Bakery keeps no port while private.
- **Persistent Storage shows only the data volume, read-only.** Coolify
  lets a Database gain extra volume, file and directory mounts. A Database
  here has exactly its one volume, so the page lists it (name and mount
  path) with no Add mount and no actions column, as Coolify shows a
  Database's default volume.
- **Resource Limits are memory and CPUs only**, the two the Database's
  Resource limits hold, on the same page as the Application's. Coolify's CPU
  set, CPU weight, memory reservation, swap and swappiness are left out.
  Saving recreates the Container with the new limits.
- **Danger Zone offers only "delete volumes".** Coolify's Danger Zone, the
  same one the Application has, also offers deleting connected networks and
  configuration files and a Docker cleanup; a Database here has no network,
  file or image of its own to remove, so only the volume checkbox is shown,
  ticked by default as in Coolify. A kept volume is left for an admin to
  copy data out of by hand: The Bakery never reattaches it, because volume
  names carry the Database's id and no new Database gets that id. Local
  Backup executions go with the Database either way, as before. Its button
  says "Delete resource", Coolify's own word for a standalone database.
- **The sidebar lists only what The Bakery has behind it.** Coolify's
  Database sidebar also has Environment Variables, Import Backup, Terminal,
  Webhooks, Healthcheck, Resource Operations, Metrics and Tags. A Database
  here has no environment variables of its own to edit, no upload to import,
  no terminal, no deploy webhook (it is never deployed), a fixed readiness
  probe instead of a Healthcheck setting, no clone or move between
  Environments, no metrics and no tags; each is a missing feature, built
  later or never, and gets its sidebar item when it exists. A Restore of the
  Database's own Backup executions is on the Executions section of its
  Scheduled backup instead of Import Backup.
- **No configuration checker.** Coolify compares the running container
  with the saved settings and offers a restart when they differ. Here every
  saved change recreates the Container at once, so the two never differ.
- **General has only the fields The Bakery has a setting for.** Coolify's
  General also has custom Docker options, port mappings, initialisation
  arguments and scripts, custom configuration files (`postgresql.conf`,
  `redis.conf` and so on), and the log drain toggle. The
  Bakery runs each Database type with its image's defaults and its one
  published port, so those sections are left out until a setting exists.
- **Valkey uses Redis's page.** Coolify has no Valkey; its General page is
  Redis's (password, no username) with Valkey's name and image.
- **A Scheduled backup's settings are Enabled, Frequency, S3 storage and
  Retention by count.** Coolify also offers which databases inside the
  server to dump (or all of them), a timeout, a missing-backup alert, turning
  the local copy off, and Retention by days and by total size, locally and on
  S3 apart. The Bakery dumps the one database it created, always keeps a
  local copy first (an upload can fail), and keeps the last "Backups to
  keep" Backup executions; Timezone is shown and is always UTC.

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
- **Readiness is probed inside the Container** with the Database type's own
  client through the Podman exec API, as Healthchecks are: in development
  the API runs on the host and cannot reach the `bakery` network, and the
  image already carries the right client.
- **Credentials are generated and shown.** Members need them to connect,
  so unlike Registry credentials they are returned by the API; at rest they
  are encrypted like Environment variables. Redis and Valkey get their password from an
  environment variable read by `sh -c`, so `podman inspect` shows no secret
  on the command line.
- **One volume per Database, removed only with the Database.** Postgres
  keeps its data in `PGDATA=/var/lib/postgresql/data/pgdata` on a volume
  mounted at `/var/lib/postgresql/data`, which works for version 18 (whose
  image moved its default data path) and older alike.
- **The Public port is published on a configured address**
  (`BAKERY_DATABASES_PUBLIC_BIND`: every interface on a server, `127.0.0.1`
  in development). The Bakery does not manage a firewall; a port another
  process holds fails the start with Podman's reason.
- **Backups live in databases, not in a context of their own.** How to dump
  and restore is Database type knowledge; a separate backups context would need a
  contract as wide as the Database type catalog. S3 storage is an aggregate here
  for now; when a second consumer appears (volume backups, The Bakery's own
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
- **Restore copies the file in, then runs the Database type's restore through
  `sh -c`**, and removes the file afterwards. libpod only takes exec stdin
  over a hijacked connection, which the thin Podman client does not speak.
  Restore is destructive and not preceded by a Backup execution of its own; the
  dashboard says so and a member can press Back up now first. What the
  last Restore did is kept in memory only: an API restart forgets the note,
  never data.
- **Local disk always, S3 optionally.** Every Backup execution is written to the
  Backups directory first, then uploaded, so a failed upload still leaves
  a Backup execution to restore from. S3 objects outlive their Database: deleting a
  Database removes its local Backup executions, not the off-site copies, which are
  there for exactly the mistake of deleting the wrong thing.
- **Scheduled backups are their own aggregate, many per Database**, as
  Coolify's scheduled database backups are: Coolify's Backups pages list
  them and open one at a time, and its API has one resource per Scheduled
  backup. Each Backup execution belongs to one, so Retention prunes per
  Scheduled backup and an hourly local schedule never prunes the dumps of a
  weekly S3 one. The migration gave every Database of a Database type with
  backups exactly the one it had, switched off ones too, so its earlier
  Backup executions keep an owner. Backup executions of one Database still
  run one at a time; two Scheduled backups due in the same minute run one
  after the other. Deleting a Scheduled backup or a Backup execution asks,
  as Coolify's dialogs do, whether its local files and its S3 copies go
  too; S3 copies stay unless asked. A shortcut Frequency is stored as
  typed and read as its five-field expression (`daily` is midnight UTC);
  other descriptors (`@every 1h`) and a `TZ=` prefix are refused. Cron in UTC so a server's time zone never shifts it. Missed runs while the API was down produce
  one Backup execution, not a burst. The scheduler runs in the API process and reads
  schedules from the database each minute, rather than Goravel's compiled
  schedule. Cron parsing uses `robfig/cron`, a pure parser, the one
  dependency the domain has outside the standard library.
- **Our own thin S3 client** (Signature V4, path-style, single PUTs up to
  5 GiB), for the same reason as the Podman client: a small dependency
  tree. Path-style URLs work with AWS, Garage and other S3-compatible
  stores alike.
- **`valkey` is a Database type Coolify does not have.** Valkey is the
  open-source fork of Redis and runs the same way; it stays as a Bakery
  extra. Like Redis it has no Backup executions.
