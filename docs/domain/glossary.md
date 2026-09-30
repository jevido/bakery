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
| Member | identity | A person who may sign in to this Bakery, with a name, an email, a password hash and a Role. | User, account |
| Role | identity | What a Member may do: `viewer` reads everything except Secrets and changes nothing; `member` also changes Projects, Environments, Applications, Deployments, Databases, Backups, Services and their routing settings; `admin` also manages Servers, S3 storages, Known hosts, Members and Invitations; `owner` has every right of admin and is the Owner. | Permission, team |
| Owner | identity | The Member with the `owner` Role. Exactly one, created by Setup; can be neither demoted nor removed. | Admin, account |
| Setup | identity | The one-time step that creates the Owner. Refused once an Owner exists. | Install |
| Invitation | identity | An email and a Role (admin, member or viewer) that an admin or the Owner invites, with a link that is good once and for 7 days, until revoked. Accepting it creates the Member. | Notification |
| Session | identity | Proof that a Member signed in: a JWT carried in the `bakery_session` cookie. | API token |
| API token | identity | A named secret (`bky_…`) of one Member, sent as `Authorization: Bearer`, acting with that Member's Role, or as viewer when it is *read-only*. Shown once, stored hashed, revocable; it dies with its Member. | Session, Deploy key |
| Secret | identity (used by every context) | A value a viewer may not read: Env var and Shared variable values, Database credentials and URLs, Service variable values, the Webhook secret, Backup contents. | Env var |
| Project | projects | A named group of Environments, usually one product. | Repository |
| Environment | projects | A stage inside a Project (`production` is created with every Project). Holds Applications and Databases. | Env var, the dev/next/prod environments of this repo |
| Application | projects | Something Bakery builds from a Source (or pulls as an Image reference) and runs as one Container behind its Domains. | Service, Container |
| Slug | projects | The URL-safe, unique short name of an Application, used in its default Domain and image name. | Name |
| Source | projects | Where an Application's code comes from, for every Build pack except `image`: a git URL and a branch. Either a public `https://` URL, or an SSH URL (`ssh://git@host/path` or `git@host:path`) that always has a Deploy key. | Repository on disk |
| Deploy key | projects | The SSH key pair Bakery generates for one Application. The Owner adds the public half to the repository as a read-only deploy key; the private half is encrypted at rest. | Server key, API token |
| Build pack | projects | How an Application becomes an Image: `dockerfile` (build the Dockerfile at a path in the Source), `nixpacks` (Nixpacks detects the language and writes the Dockerfile), `static` (serve the Publish directory of the Source as files on port 80), or `image` (pull an Image reference from a registry; no Source). | Buildpacks (Heroku/CNB) |
| Image reference | projects | The registry reference an `image` Application runs, always with its registry host, e.g. `docker.io/traefik/whoami:v1.10`. | Image (the Deployment's own tag) |
| Registry credentials | projects | The username and password or token an `image` Application pulls its Image reference with. The password is encrypted at rest and never shown. | Deploy key |
| Publish directory | projects | The directory of the Source a `static` Application serves, relative to the repository root; `.` by default. | Dockerfile path |
| Env var | projects | A name and value of one Application, handed to its build, its Container or both (see Variable scope). The value is encrypted at rest. | Environment |
| Variable scope | projects | Where a variable reaches: *build* (a build arg of the Image) and/or *runtime* (the Container's environment). Runtime only by default. | Environment |
| Shared variable | projects | A variable set on a Project or an Environment and inherited by every Application in it. An Application's own Env var wins over its Environment's, which wins over its Project's. | Env var |
| Health check | projects, deployments | An HTTP path probed inside a new Container until it answers 2xx or 3xx, with an interval, a timeout, a number of retries and a start period. Off by default. The Route only moves to a Container that passed it. | Container state |
| Domain | projects, routing, services | One of the hostnames an Application or a Public Component is reached on. An Application has 1 to 10; the first is its *primary* Domain (shown in lists, used for links). Each is unique across Bakery (Applications and Services share one namespace), and never the dashboard domain. The default is one Domain `<slug>.<domain suffix>` (`localhost` in development). | URL |
| Persistent storage | projects, deployments | A named volume of one Application (`bakery-app-<application-id>-<name>`), mounted at its mount path in every Container of that Application, so what is written there survives redeploys and rollbacks. | Bind mount (later), Image |
| Resource limits | projects, deployments, databases | The memory (MB) and CPU (cores) each Container of an Application, or the Container of a Database, may use. Empty means unlimited. Applied from an Application's next Deployment, and to a Database at once (its Container is recreated). | Server capacity |
| Deployment | deployments | One attempt to turn an Application's Source into its running Container. | Release, build |
| Deploy trigger | deployments | What started a Deployment: `manual` (the Owner pressed Deploy), `webhook` (a push) or `rollback` (the Owner rolled back to an earlier Deployment). | Build pack |
| Deployment status | deployments | Where a Deployment is: `queued`, `cloning`, `building`, `starting`, then `finished`, `failed` or `cancelled`. An `image` Deployment skips `cloning` and pulls during `building`. The first four are *active*; `cloning`, `building` and `starting` are *running*. | Container state |
| Cancel | deployments | The Owner ending an active Deployment before it finishes. It ends `cancelled`, what it made is removed, and the running version stays. | Fail, rollback |
| Rollback | deployments | A Deployment that starts an earlier finished Deployment's Image again, without cloning or building. | Revert (git), redeploy |
| Deployment log | deployments | The ordered lines a Deployment wrote: its own `info` lines and the `out`/`err` output of git and the build. | Container logs |
| Container logs | deployments, databases, services | What a running Application's, Database's or Component's Container writes to stdout and stderr. | Deployment log |
| Image | deployments | The result of a Deployment, built or pulled and re-tagged as `localhost/bakery/<slug>:<deployment-id>`. | Container |
| Source image | deployments | The reference with digest an `image` Deployment pulled, e.g. `docker.io/traefik/whoami@sha256:…`. Shown where git Deployments show their commit. | Image reference |
| Container | deployments, services | A running instance of an Image, named `bakery-app-<application-id>-<deployment-id>` for an Application and `bakery-svc-<service-id>-<component>` for a Component. | Application |
| Webhook | deployments | The URL and secret a git host calls on every push, so a push to an Application's branch deploys it. | Notification webhook (later) |
| Auto-deploy | deployments | Whether a verified push to the Application's branch queues a Deployment. On by default. | Redeploy |
| Known host | deployments | A git host's SSH host key, remembered on the first clone from that host and required to match on every later one. | Host key (Servers) |
| Server | servers | A machine Bakery runs Containers on: the Local server, through the rootless Podman socket, or a Remote server over SSH. Applications run on their Target server; Databases and Services still run on the Local server. | Proxy |
| Target server | projects, deployments, routing | The Server an Application's Images are built on and its Containers run on. Chosen when the Application is added and fixed afterwards; the Local server when none was chosen. | Server connection |
| Server connection | servers | A pooled way for other contexts to reach one Server: its Podman API and any unix socket on it. One SSH connection per Remote server, redialled when it drops. | Validation |
| Local server | servers | The Server Bakery itself runs on (`localhost`). Always exists; can be neither edited nor deleted. | Remote server |
| Remote server | servers | A Server reached over SSH as a given user, its rootless Podman socket tunnelled through that connection. | Local server |
| Server key | servers | The SSH key pair Bakery generates for one Remote server. The Owner adds the public half to the user's `authorized_keys`; the private half is encrypted at rest. | Deploy key |
| Host key | servers | A Remote server's SSH host key, pinned on the first connection and required to match on every later one until the Owner forgets it. | Known host (git hosts) |
| Validation | servers | The checks run against a Server (`ssh`, `podman`, `socket`, `linger`, `ports`) and their outcome; it sets the Server status. | Health check |
| Server status | servers | `unvalidated`, `reachable` (every required check passed) or `unreachable`. | Container state |
| Server metrics | servers | CPU, memory and disk use of a Server, read live from Podman. | Container metrics |
| Container metrics | servers | CPU and memory use of each Bakery Container on a Server, read live. | Server metrics, Container logs |
| Cleanup | servers | Freeing a Server's disk: dangling Bakery images and build layers, plus Image retention for the Applications that run on it. Daily and on demand. | Delete |
| Image retention | deployments | Per Application, the Images of the running Deployment and of the last five finished Deployments are kept; older ones are removed during Cleanup, and a Rollback to one of those is refused ("the image is gone"). | Cleanup |
| Service | services | Multi-container software described by a Compose file and run in an Environment, created from a Service template or a pasted Compose file. | Application, a compose `services:` entry (that is a Component), the repo's `services/` directory |
| Compose file | services | The `compose.yml` text a Service is made of, in the subset Bakery supports; anything else is refused with its line. | Containerfile |
| Component | services | One entry under a Compose file's `services:`, run as one Container `bakery-svc-<service-id>-<name>`. *Public* when it has a port and Domains. | Service, Application |
| Service variable | services | A `${NAME}` a Compose file refers to, with a value stored encrypted: set by the Owner, or generated when it is a Magic variable. | Env var, Shared variable |
| Magic variable | services | A Service variable Bakery fills in, named as in Coolify's templates: `SERVICE_PASSWORD_*`, `SERVICE_USER_*`, `SERVICE_BASE64_*` (generated once), `SERVICE_FQDN_<NAME>` / `SERVICE_URL_<NAME>` (the Component's primary Domain). A `_<PORT>` suffix makes the Component public on that port. | Database credentials |
| Service template | services | A named, described Compose file embedded in Bakery; the catalog under New → Service. | Build pack |
| Service status | services | `running`, `stopped`, `deploying`, `degraded` or `failed`, summed up from each Component's status (read from Podman like a Database status). Only the desired state is stored. | Deployment status |
| Service route | routing | A Public Component's Domains pointed at its Container and port. One per Public Component, rendered with the Routes. | Route |
| Proxy | routing | The Caddy container `bakery-proxy` on a Server, configured only through its admin API. One per Server that has Routes; the Local server's also serves the Dashboard Route and Service routes. | Server |
| Route | routing | An Application's Domains pointed at one Container and port on its Target server, served by that Server's Proxy. One per Application. | Endpoint |
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
