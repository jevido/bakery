# Glossary

The ubiquitous language. These are the words used in conversation, in issues,
in the docs **and in the code**: class, type, function, table and event names
follow this list. If a term is missing or wrong, fix it here first, then in the
code.

A term that means different things in different contexts gets one row per
context. Terms used only inside one context can also live in that context's
document; list them here when people outside the context use them too.

| Term | Context | Meaning | Not to be confused with |
| ---- | ------- | ------- | ----------------------- |
| Owner | identity | The one person who administers this Bakery installation. Created by setup on first run. | User, admin, account |
| Session | identity | Proof that the Owner signed in: a JWT carried in the `bakery_session` cookie. | Token (API tokens come later) |
| Setup | identity | The one-time step that creates the Owner. Refused once an Owner exists. | Install |
| Project | projects | A named group of Environments, usually one product. | Repository |
| Environment | projects | A stage inside a Project (`production` is created with every Project). Holds Applications and Databases. | Env var, the dev/next/prod environments of this repo |
| Application | projects | Something Bakery builds from a Source (or pulls as an Image reference) and runs as one Container behind its Domains. | Service (templates, later), Container |
| Slug | projects | The URL-safe, unique short name of an Application, used in its default Domain and image name. | Name |
| Source | projects | Where an Application's code comes from, for every Build pack except `image`: a git URL and a branch. Either a public `https://` URL, or an SSH URL (`ssh://git@host/path` or `git@host:path`) that always has a Deploy key. | Repository on disk |
| Deploy key | projects | The SSH key pair Bakery generates for one Application. The Owner adds the public half to the repository as a read-only deploy key; the private half is encrypted at rest. | The Server's SSH key, API token |
| Build pack | projects | How an Application becomes an Image: `dockerfile` (build the Dockerfile at a path in the Source), `nixpacks` (Nixpacks detects the language and writes the Dockerfile), `static` (serve the Publish directory of the Source as files on port 80), or `image` (pull an Image reference from a registry; no Source). | Buildpacks (Heroku/CNB) |
| Image reference | projects | The registry reference an `image` Application runs, always with its registry host, e.g. `docker.io/traefik/whoami:v1.10`. | Image (the Deployment's own tag) |
| Registry credentials | projects | The username and password or token an `image` Application pulls its Image reference with. The password is encrypted at rest and never shown. | Deploy key |
| Publish directory | projects | The directory of the Source a `static` Application serves, relative to the repository root; `.` by default. | Dockerfile path |
| Env var | projects | A name and value of one Application, handed to its build, its Container or both (see Variable scope). The value is encrypted at rest. | Environment |
| Variable scope | projects | Where a variable reaches: *build* (a build arg of the Image) and/or *runtime* (the Container's environment). Runtime only by default. | Environment |
| Shared variable | projects | A variable set on a Project or an Environment and inherited by every Application in it. An Application's own Env var wins over its Environment's, which wins over its Project's. | Env var |
| Health check | projects, deployments | An HTTP path probed inside a new Container until it answers 2xx or 3xx, with an interval, a timeout, a number of retries and a start period. Off by default. The Route only moves to a Container that passed it. | Container state |
| Domain | projects, routing | One of the hostnames an Application is reached on. An Application has 1 to 10; the first is its *primary* Domain (shown in lists, used for links). Each is unique across Bakery, and never the dashboard domain. The default is one Domain `<slug>.<domain suffix>` (`localhost` in development). | URL |
| Persistent storage | projects, deployments | A named volume of one Application (`bakery-app-<application-id>-<name>`), mounted at its mount path in every Container of that Application, so what is written there survives redeploys and rollbacks. | Bind mount (later), Image |
| Resource limits | projects, deployments, databases | The memory (MB) and CPU (cores) each Container of an Application, or the Container of a Database, may use. Empty means unlimited. Applied from an Application's next Deployment, and to a Database at once (its Container is recreated). | Server capacity |
| Deployment | deployments | One attempt to turn an Application's Source into its running Container. | Release, build |
| Deploy trigger | deployments | What started a Deployment: `manual` (the Owner pressed Deploy), `webhook` (a push) or `rollback` (the Owner rolled back to an earlier Deployment). | Build pack |
| Deployment status | deployments | Where a Deployment is: `queued`, `cloning`, `building`, `starting`, then `finished`, `failed` or `cancelled`. An `image` Deployment skips `cloning` and pulls during `building`. The first four are *active*; `cloning`, `building` and `starting` are *running*. | Container state |
| Cancel | deployments | The Owner ending an active Deployment before it finishes. It ends `cancelled`, what it made is removed, and the running version stays. | Fail, rollback |
| Rollback | deployments | A Deployment that starts an earlier finished Deployment's Image again, without cloning or building. | Revert (git), redeploy |
| Deployment log | deployments | The ordered lines a Deployment wrote: its own `info` lines and the `out`/`err` output of git and the build. | Container logs |
| Container logs | deployments, databases | What a running Application's or Database's Container writes to stdout and stderr. | Deployment log |
| Image | deployments | The result of a Deployment, built or pulled and re-tagged as `localhost/bakery/<slug>:<deployment-id>`. | Container |
| Source image | deployments | The reference with digest an `image` Deployment pulled, e.g. `docker.io/traefik/whoami@sha256:…`. Shown where git Deployments show their commit. | Image reference |
| Container | deployments | A running instance of an Image, named `bakery-app-<application-id>-<deployment-id>`. | Application |
| Webhook | deployments | The URL and secret a git host calls on every push, so a push to an Application's branch deploys it. | Notification webhook (later) |
| Auto-deploy | deployments | Whether a verified push to the Application's branch queues a Deployment. On by default. | Redeploy |
| Known host | deployments | A git host's SSH host key, remembered on the first clone from that host and required to match on every later one. | Server |
| Server | deployments | A machine Bakery runs Containers on. Only the local one (the rootless Podman socket) exists. | Proxy |
| Proxy | routing | The Caddy container `bakery-proxy`, configured only through its admin API. | Server |
| Route | routing | An Application's Domains pointed at one Container and port. One per Application. | Endpoint |
| Route settings | routing | How the Proxy treats one Application's traffic: its Www redirect, Response headers and Basic auth. Applied at once, without a Deployment. | Application settings in projects |
| Www redirect | routing | `off`, `to_apex` or `to_www`: for every Domain, the counterpart with `www.` added or removed answers with a permanent (308) redirect to it, keeping path and query. | Domain |
| Response header | routing | A header name and value the Proxy sets on every response of an Application. | Env var |
| Basic auth | routing | One username and password the Proxy asks for before any request reaches the Application. Only a bcrypt hash of the password is kept. | Owner, Session |
| Dashboard Route | routing | The Route to Bakery's own dashboard and API on the configured dashboard domain: `/api/*` to the API, the rest to the dashboard. Derived from configuration, never stored. | Route |
| Database | databases | A data store of one Engine that Bakery runs as one Container (`bakery-db-<slug>`) in an Environment, with its data in one volume. Created in one step; never built or deployed. | Application, Postgres (Bakery's own database) |
| Engine | databases | What kind of Database it is: `postgresql`, `mysql`, `mariadb`, `redis`, `valkey` or `mongodb`. Fixed once the Database exists. | Build pack |
| Database version | databases | The image tag of the Engine a Database runs, e.g. `18-alpine`. Each Engine has a default. | Deployment |
| Database credentials | databases | The username (`bakery`), password (and for MySQL/MariaDB a root password) and database name (the Slug with `_` for `-`) of a Database. Generated by Bakery, encrypted at rest, shown to the Owner. | Registry credentials, Owner |
| Internal URL | databases | The connection URL of a Database on the `bakery` network, with host `bakery-db-<slug>`. What Applications use. | Public URL, Domain |
| Public port | databases | A host port (1024–65535, unique among Databases) a Database is published on, so it can be reached from outside the Server. None by default. | Port (of an Application) |
| Public URL | databases | The connection URL of a Database through its Public port on the Server's public host. Only exists with a Public port. | Internal URL, Domain |
| Database status | databases | What a Database's Container is doing, read from Podman: `starting` (running, not answering yet), `running` (answers its Engine's readiness probe), `stopped` (the Owner stopped it), `exited` (stopped on its own) or `missing` (no Container). Not stored; the Owner's wish (running or stopped) is its *desired state*. | Deployment status |
| Backup | databases | One logical dump of a Database, written to the Backups directory and, when its Backup schedule names an S3 storage, uploaded there. Its *status* is `running`, `succeeded` or `failed` (with a reason); its *trigger* is `manual` (Back up now) or `scheduled`. Only PostgreSQL, MySQL, MariaDB and MongoDB have Backups. | Snapshot, volume copy |
| Backup schedule | databases | When a Database is backed up by itself: on or off, a five-field cron expression in UTC, a Retention and optionally an S3 storage. One per Database. | Scheduled command |
| Retention | databases | How many of a Database's newest succeeded Backups are kept (1–100, default 7); older ones are deleted from the Backups directory and the S3 storage alike. | Expiry |
| Backups directory | databases | The directory on the Server Bakery writes Backup files to, `<database id>/<UTC timestamp>.<ext>` below it. The volume `bakery-backups` on a server. | Persistent storage |
| S3 storage | databases | An S3-compatible bucket Backups are uploaded to: endpoint, region, bucket, optional prefix, access key and secret key. The secret key is encrypted at rest and never shown. | Persistent storage, Backups directory |
| Restore | databases | Replacing the data of a running Database with the contents of one of its succeeded Backups, from the Backups directory or, when the file is gone, from its S3 storage. Anything written since the Backup is lost. | Rollback |
