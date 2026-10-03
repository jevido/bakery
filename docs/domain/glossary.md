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
| Member | identity | A person who may sign in to this installation of The Bakery, with a name, an email, a password hash and a Role. | User, account |
| Role | identity | What a Member may do: `viewer` reads everything except Secrets and changes nothing; `member` also changes Projects, Environments, Applications, Deployments, Databases, Scheduled backups, Backup executions, Services and their routing settings; `admin` also manages Servers, S3 storages, Known hosts, Members and Invitations; `owner` has every right of admin and is the Owner. | Permission, team |
| Owner | identity | The Member with the `owner` Role. Exactly one, created by Setup; can be neither demoted nor removed. | Admin |
| Setup | identity | The one-time step that creates the Owner. Refused once an Owner exists. | Install |
| Invitation | identity | An email and a Role (admin, member or viewer) that an admin or the Owner invites, with a link that is good once and for 7 days, until revoked. Accepting it creates the Member. | Notification |
| Session | identity | Proof that a Member signed in: a JWT carried in the `bakery_session` cookie. It stops counting when the Member changes their password or signs out everywhere else. | API token |
| Profile | identity | A Member's own name, password, Sessions and Two-factor authentication, changed only by that Member on their Profile page. | Member (as others see them) |
| Two-factor authentication | identity | A TOTP secret on a Member. When it is on, signing in needs an Authenticator code or a Recovery code after the password. | Deploy key, API token |
| Authenticator code | identity | The 6 digits the Member's authenticator app shows (RFC 6238, 30 second steps). Accepted for the current step or one either side, and only once. | Recovery code |
| Recovery code | identity | One of 10 single-use codes handed out when Two-factor authentication is switched on, for signing in without the phone. Stored hashed. | Authenticator code, API token |
| Login challenge | identity | The 5 minutes between a correct password and the second sign-in step of a Member with Two-factor authentication; at most 5 wrong codes. | Session |
| API token | identity | A named secret (`bky_…`) of one Member, sent as `Authorization: Bearer`, acting with that Member's Role, or as viewer when it is *read-only*. Shown once, stored hashed, revocable; it dies with its Member. | Session, Deploy key |
| Secret | identity (used by every context) | A value a viewer may not read: Environment variable and Shared variable values, Database credentials and URLs, Service variable values, the Webhook secret, the Git host token, Backup execution contents. | Environment variable |
| Project | projects | A named group of Environments, usually one product. | Repository |
| Environment | projects | A stage inside a Project (`production` is created with every Project; more can be added). Holds its Resources. | Environment variable, the dev/next/prod environments of this repo |
| Application | projects | Something The Bakery builds from a Git repository (or pulls as a Docker image) and runs as one Container behind its Domains. | Service, Container |
| Resource | projects (used by every context) | An Application, Database or Service in an Environment; Coolify's word for all three, as in an Environment's list of Resources. | Resource limits |
| Slug | projects | The URL-safe, unique short name of an Application, used in its default Domain and image name. | Name |
| Git repository | projects | Where an Application's code comes from, for every Build pack except `dockerimage`: a git URL and its *Git branch*. Either a public `https://` URL, or an SSH URL (`ssh://git@host/path` or `git@host:path`) that always has a Deploy key. The Application tab that edits it is labelled "Source", as in Coolify; otherwise *Source* is kept for a GitHub App or GitLab connection, which The Bakery does not have yet. | Repository on disk, Source |
| Deploy key | projects | The SSH key pair The Bakery generates for one Application. The Owner adds the public half to the repository as a read-only deploy key; the private half is encrypted at rest. | Private key, API token |
| Build pack | projects | How an Application becomes an Image: `dockerfile` (build the Dockerfile at a path in the Git repository), `nixpacks` (Nixpacks detects the language and writes the Dockerfile), `static` (serve the Publish directory of the Git repository as files on port 80), or `dockerimage` (pull a Docker image from a registry; no Git repository). | Buildpacks (Heroku/CNB) |
| Docker image | projects | The registry reference a `dockerimage` Application runs, always with its registry host, e.g. `docker.io/traefik/whoami:v1.10`. | Image (the Deployment's own tag) |
| Registry credentials | projects | The username and password or token a `dockerimage` Application pulls its Docker image with. The password is encrypted at rest and never shown. | Deploy key |
| Publish directory | projects | The directory of the Git repository a `static` Application serves, relative to the repository root; `.` by default. | Dockerfile path |
| Environment variable | projects | A name and value of one Application, handed to its build, its Container or both (see Variable scope). The value is encrypted at rest. | Environment |
| Variable scope | projects | Where a variable reaches: *build* (a build arg of the Image) and/or *runtime* (the Container's environment). Runtime only by default. | Environment |
| Shared variable | projects | A variable set on a Project or an Environment and inherited by every Application in it. An Application's own Environment variable wins over its Environment's, which wins over its Project's. | Environment variable |
| Healthcheck | projects, deployments | An HTTP path probed inside a new Container until it answers 2xx or 3xx, with an interval, a timeout, a number of retries and a start period. Off by default. The Route only moves to a Container that passed it. | Container state |
| Domain | projects, routing, services | One of the hostnames an Application or a Public Component is reached on. An Application has 1 to 10; the first is its *primary* Domain (shown in lists, used for links). Each is unique across The Bakery (Applications and Services share one namespace), and never the dashboard domain. The default is one Domain `<slug>.<domain suffix>` (`localhost` in development). | URL |
| Persistent storage | projects, deployments | A named volume of one Application (`bakery-app-<application-id>-<name>`), mounted at its mount path in every Container of that Application, so what is written there survives redeploys and rollbacks. | Bind mount (later), Image |
| Resource limits | projects, deployments, databases | The memory (MB) and CPU (cores) each Container of an Application, or the Container of a Database, may use. Empty means unlimited. Applied from an Application's next Deployment, and to a Database at once (its Container is recreated). | Server capacity |
| Deployment | deployments | One attempt to turn an Application's Git repository (or Docker image) into its running Container. | Release, build |
| Deploy trigger | deployments | What started a Deployment: `manual` (the Owner pressed Deploy), `webhook` (a push) or `rollback` (the Owner rolled back to an earlier Deployment). | Build pack |
| Deployment status | deployments | Where a Deployment is: `queued`, `cloning`, `building`, `starting`, then `finished`, `failed` or `cancelled`. A `dockerimage` Deployment skips `cloning` and pulls during `building`. The first four are *active*; `cloning`, `building` and `starting` are *running*. | Container state |
| Cancel | deployments | The Owner ending an active Deployment before it finishes. It ends `cancelled`, what it made is removed, and the running version stays. | Fail, rollback |
| Rollback | deployments | A Deployment that starts an earlier finished Deployment's Image again, without cloning or building. | Revert (git), redeploy |
| Deployment log | deployments | The ordered lines a Deployment wrote: its own `info` lines and the `out`/`err` output of git and the build. | Container logs |
| Container logs | deployments, databases, services | What a running Application's, Database's or Component's Container writes to stdout and stderr. | Deployment log |
| Image | deployments | The result of a Deployment, built or pulled and re-tagged as `localhost/bakery/<slug>:<deployment-id>`. | Container |
| Source image | deployments | The reference with digest a `dockerimage` Deployment pulled, e.g. `docker.io/traefik/whoami@sha256:…`. Shown where git Deployments show their commit. | Docker image |
| Container | deployments, services | A running instance of an Image, named `bakery-app-<application-id>-<deployment-id>` for an Application and `bakery-svc-<service-id>-<component>` for a Component. | Application |
| Webhook | deployments | The URL and secret a git host calls on every push, so a push to an Application's branch deploys it. | Notification webhook (later) |
| Pull request | deployments | A request on a git host to merge a branch into the Application's branch; a GitLab merge request counts as one. Known to The Bakery only through Webhook calls. | Deployment |
| Previews | deployments | The switch on an Application's Webhook that lets Pull requests start Previews. Off until switched on. | Preview |
| Preview | deployments | A running copy of an Application built from the head branch of one open Pull request in the Application's own repository, with its own Containers, Volumes and Preview route. `open` until the Pull request closes, then `closed` with nothing left running. | Environment, Rollback |
| Preview number | deployments | The Pull request's number (GitLab: its `iid`); names the Preview, its Containers, Volumes and Preview domain. | Deployment id |
| Preview domain | deployments, routing | `pr-<number>.<primary Domain>`, where a Preview is served. | Domain |
| Preview route | routing | Where a Preview's Preview domain is served from: one per Application and Preview number, on the Application's Server, with the Application's headers and basic auth. | Route |
| Preview comment | deployments | The one comment The Bakery keeps on a Pull request, edited after every Preview Deployment and when the Preview is removed. Needs a Git host token. | Notification |
| Git host token | deployments | An access token for the git host, stored encrypted on the Webhook, that lets The Bakery write the Preview comment. A Secret. | Deploy key, API token |
| Auto-deploy | deployments | Whether a verified push to the Application's branch queues a Deployment. On by default. | Redeploy |
| Known host | deployments | A git host's SSH host key, remembered on the first clone from that host and required to match on every later one. | Host key (Servers) |
| Server | servers | A machine The Bakery runs Containers on: the Local server, through the rootless Podman socket, or a Remote server over SSH. Applications run on their Target server; Databases and Services still run on the Local server. | Proxy |
| Target server | projects, deployments, routing | The Server an Application's Images are built on and its Containers run on. Chosen when the Application is added and fixed afterwards; the Local server when none was chosen. | Server connection |
| Server connection | servers | A pooled way for other contexts to reach one Server: its Podman API and any unix socket on it. One SSH connection per Remote server, redialled when it drops. | Validation |
| Local server | servers | The Server The Bakery itself runs on (`localhost`). Always exists; can be neither edited nor deleted. | Remote server |
| Remote server | servers | A Server reached over SSH as a given user, its rootless Podman socket tunnelled through that connection. | Local server |
| Private key | servers | The SSH key pair The Bakery generates for one Remote server. The Owner adds the public half to the user's `authorized_keys`; the private half is encrypted at rest. | Deploy key |
| Host key | servers | A Remote server's SSH host key, pinned on the first connection and required to match on every later one until the Owner forgets it. | Known host (git hosts) |
| Validation | servers | The checks run against a Server (`ssh`, `podman`, `socket`, `linger`, `ports`) and their outcome; it sets the Server status, as does the Server probe. | Healthcheck, Server probe |
| Server status | servers | `unvalidated`, `reachable` (every required check passed) or `unreachable`. | Container state |
| Server metrics | servers | CPU, memory and disk use of a Server, read live from Podman. | Container metrics |
| Container metrics | servers | CPU and memory use of each Bakery Container on a Server, read live. | Server metrics, Container logs |
| Server probe | servers | The check every 5 minutes of every Server whose latest Validation passed: can The Bakery reach it, and how full is its disk. Two failed probes in a row make a Reachable Server Unreachable, one good probe makes it Reachable again; *high disk usage* is raised at 90 % used and cleared below 85 %. Only a change is announced. | Healthcheck, Validation |
| Cleanup | servers | Freeing a Server's disk: dangling Bakery images and build layers, plus Image retention for the Applications that run on it. Daily and on demand. | Delete |
| Image retention | deployments | Per Application, the Images of the running Deployment and of the last five finished Deployments are kept; older ones are removed during Cleanup, and a Rollback to one of those is refused ("the image is gone"). | Cleanup |
| Service | services | Multi-container software described by a Compose file and run in an Environment, created from a Service template or a pasted Compose file. | Application, a compose `services:` entry (that is a Component), the repo's `services/` directory |
| Compose file | services | The `compose.yml` text a Service is made of, in the subset The Bakery supports; anything else is refused with its line. | Containerfile |
| Component | services | One entry under a Compose file's `services:`, run as one Container `bakery-svc-<service-id>-<name>`. *Public* when it has a port and Domains. | Service, Application |
| Service variable | services | A `${NAME}` a Compose file refers to, with a value stored encrypted: set by the Owner, or generated when it is a Magic variable. | Environment variable, Shared variable |
| Magic variable | services | A Service variable The Bakery fills in, named as in Coolify's templates: `SERVICE_PASSWORD_*`, `SERVICE_USER_*`, `SERVICE_BASE64_*` (generated once), `SERVICE_FQDN_<NAME>` / `SERVICE_URL_<NAME>` (the Component's primary Domain). A `_<PORT>` suffix makes the Component public on that port. | Database credentials |
| Service template | services | A named, described Compose file embedded in The Bakery; the catalog under New → Service. | Build pack |
| Service status | services | `running`, `stopped`, `deploying`, `degraded` or `failed`, summed up from each Component's status (read from Podman like a Database status). Only the desired state is stored. | Deployment status |
| Service route | routing | A Public Component's Domains pointed at its Container and port. One per Public Component, rendered with the Routes. | Route |
| Proxy | routing | The Caddy container `bakery-proxy` on a Server, configured only through its admin API. One per Server that has Routes; the Local server's also serves the Dashboard Route and Service routes. | Server |
| Route | routing | An Application's Domains pointed at one Container and port on its Target server, served by that Server's Proxy. One per Application. | Endpoint |
| Route settings | routing | How the Proxy treats one Application's traffic: its Redirect, Response headers and Basic auth. Applied at once, without a Deployment. | Application settings in projects |
| Redirect | routing | `both` ("Allow www & non-www", the default: no redirect), `non-www` ("Redirect to non-www") or `www` ("Redirect to www"): with `non-www` or `www`, for every Domain the counterpart with `www.` removed or added answers with a permanent (308) redirect to the chosen form, keeping path and query. | Domain |
| Response header | routing | A header name and value the Proxy sets on every response of an Application. | Environment variable |
| Basic auth | routing | One username and password the Proxy asks for before any request reaches the Application. Only a bcrypt hash of the password is kept. | Owner, Session |
| Dashboard Route | routing | The Route to The Bakery's own dashboard and API on the configured dashboard domain: `/api/*` to the API, the rest to the dashboard. Derived from configuration, never stored. | Route |
| Database | databases | A data store of one Database type that The Bakery runs as one Container (`bakery-db-<slug>`) in an Environment, with its data in one volume. Created in one step; never built or deployed. | Application, Postgres (The Bakery's own database) |
| Database type | databases | What kind of Database it is: `postgresql`, `mysql`, `mariadb`, `redis`, `valkey` or `mongodb`. Fixed once the Database exists. | Build pack |
| Database version | databases | The image tag of the Database type a Database runs, e.g. `18-alpine`. Each Database type has a default. | Deployment |
| Database credentials | databases | The username (`bakery`), password (and for MySQL/MariaDB a root password) and database name (the Slug with `_` for `-`) of a Database. Generated by The Bakery, encrypted at rest, shown to the Owner. | Registry credentials, Owner |
| Internal URL | databases | The connection URL of a Database on the `bakery` network, with host `bakery-db-<slug>`. What Applications use. | Public URL, Domain |
| Public port | databases | A host port (1024–65535, unique among Databases) a Database is published on, so it can be reached from outside the Server. None by default. | Port (of an Application) |
| Public URL | databases | The connection URL of a Database through its Public port on the Server's public host. Only exists with a Public port. | Internal URL, Domain |
| Database status | databases | What a Database's Container is doing, read from Podman: `starting` (running, not answering yet), `running` (answers its Database type's readiness probe), `stopped` (the Owner stopped it), `exited` (stopped on its own) or `missing` (no Container). Not stored; the Owner's wish (running or stopped) is its *desired state*. | Deployment status |
| Backup execution | databases | One logical dump of a Database, written to the Backups directory and, when its Scheduled backup names an S3 storage, uploaded there. Its *status* is `running`, `succeeded` or `failed` (with a reason); its *trigger* is `manual` (Back up now) or `scheduled`. Only PostgreSQL, MySQL, MariaDB and MongoDB have Backup executions. | Snapshot, volume copy |
| Scheduled backup | databases | When a Database is backed up by itself: on or off, a five-field cron expression in UTC, a Retention and optionally an S3 storage. One per Database. | Scheduled command |
| Retention | databases | How many of a Database's newest succeeded Backup executions are kept (1–100, default 7); older ones are deleted from the Backups directory and the S3 storage alike. | Expiry |
| Backups directory | databases | The directory on the Server The Bakery writes Backup execution files to, `<database id>/<UTC timestamp>.<ext>` below it. The volume `bakery-backups` on a server. | Persistent storage |
| S3 storage | databases | An S3-compatible bucket Backup executions are uploaded to: endpoint, region, bucket, optional prefix, access key and secret key. The secret key is encrypted at rest and never shown. | Persistent storage, Backups directory |
| Restore | databases | Replacing the data of a running Database with the contents of one of its succeeded Backup executions, from the Backups directory or, when the file is gone, from its S3 storage. Anything written since the Backup execution is lost. | Rollback |
| Notification channel | notifications | A named place Notifications go to: a Channel kind, its settings (secrets encrypted, never shown) and the Event kinds it is subscribed to. Managed by admins. | Webhook (inbound, deployments) |
| Channel kind | notifications | `email`, `discord`, `slack`, `telegram`, `ntfy` or `webhook`: how a Notification channel is reached. | Event kind |
| Event kind | notifications | What a Notification is about, named as in Coolify: `deployment_failure`, `deployment_success`, `backup_failure`, `backup_success`, `server_unreachable`, `server_reachable`, `server_disk_usage`. | Domain event |
| Notification | notifications | A message about one thing that happened: Event kind, title, body, a link into the dashboard. Sent to every Notification channel subscribed to its Event kind. | Invitation, log line |
| Delivery | notifications | One Notification sent to one Notification channel: `pending`, `sent` or `failed`, with up to 3 attempts and the last error. The 50 newest per channel are kept. | Deployment |
