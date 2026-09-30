# deployments

- Subdomain: core
- Hosted in: `services/api` (module `contexts/deployments`)

## Purpose

Turns an Application into a running Container on its Target server: clone the Source and build
the Image with Podman on that Server (from its Dockerfile, a Dockerfile Nixpacks writes, or
a generated static file server), or pull its Image reference, then start the
Container, move the Route to it, and remove the
Container it replaces once the new one passes its Health check, writing
every step to the Deployment log. Also cancels a Deployment and rolls back to
an earlier one's Image, and runs Previews: a copy of the Application per open
Pull request, deployed from its head branch and removed when it closes. Also follows
a running Application's Container logs. It is **not** responsible for what an
Application is (projects) or for the Caddy configuration (routing).

## Language

| Term | Meaning |
| ---- | ------- |
| Deployment | One attempt on one Server (the Application's Target server when it started), with a status, its trigger, the branch and commit (SHA, subject, author) it built or the Source image (reference with digest) it pulled, its Image and Container. |
| Source image | The pulled reference with its digest, e.g. `docker.io/traefik/whoami@sha256:…`, recorded by an `image` Deployment. |
| Health check | Probed inside the new Container (`curl`, else `wget`) before the Route moves. |
| Cancelled | The final status of a Deployment the Owner cancelled. |
| Rollback | A Deployment that starts an earlier finished Deployment's Image, skipping clone and build. |
| Active | A Deployment in `queued`, `cloning`, `building` or `starting`. |
| Running | A Deployment in `cloning`, `building` or `starting`. |
| Deploy trigger | `manual`, `webhook` or `rollback`. |
| Webhook | The URL and secret a git host calls on push and on Pull request events, with Previews on or off and the Git host token. |
| Pull request | A git host's request to merge a branch into the Application's branch (a GitLab merge request too), as its Webhook calls describe it. |
| Preview | A copy of the Application built from one open Pull request's head branch, with its own Containers (`bakery-app-<id>-pr<n>-<deployment>`, labelled `bakery.preview=<n>`), Volumes (`bakery-app-<id>-pr<n>-<storage>`) and Preview route on the Preview domain `pr-<n>.<primary Domain>`. |
| Preview Deployment | A Deployment that belongs to a Preview. The history shows it next to the Application's own, marked with its Preview number. |
| Preview comment | The one comment on the Pull request Bakery posts after the first Preview Deployment and edits after each later one and when the Preview goes. |
| Git host token | The git host access token the Preview comment is written with. Encrypted at rest, never returned. |
| Known host | A git host's SSH host key, trusted on first use. |
| Worker | The loop inside the API that claims queued Deployments and runs them. |
| Volume | The Podman volume `bakery-app-<application-id>-<storage name>` behind one Persistent storage, labelled `bakery.managed=true` and `bakery.application=<id>`. |
| Podman client | Our thin client for the libpod REST API on the rootless socket. |

## Model

### Aggregates

| Aggregate | Invariants |
| --------- | ---------- |
| Deployment | Belongs to the Application itself or to one of its Previews. Status only moves forward: `queued` → `cloning` → `building` → `starting` → `finished` (an `image` Deployment moves from `cloning` straight on to `building` without cloning), and any active status → `failed` (with an error) or `cancelled`. A Rollback moves from `queued` straight to `starting`; it names its source Deployment, which is `finished`, of the same Application, and whose Image still exists. An Application has at most one queued Deployment of its own and one per Preview, and at most one running Deployment in all; a queued one is only picked up once the Application has no running one, and a new Deploy while one is already queued (for the same Preview) is refused. A Preview Deployment cannot be rolled back to: a Preview always builds its head. Its log is append-only and ordered. |
| Webhook | One per Application, with a secret and Auto-deploy on or off. A call is accepted only with a valid signature for that secret (HMAC-SHA256 of the body for GitHub, Gitea and Forgejo; the token for GitLab). Only a push to the Application's branch, with Auto-deploy on, queues a Deployment. Only a Pull request event, with Previews on, whose head is a branch of the same repository and whose base is the Application's branch, opens, deploys or closes a Preview. |
| Preview | One per Application and Preview number. `open` → `closed`, and back to `open` when the Pull request is reopened. Only an open Preview is deployed. Closing it removes its Containers, Volumes, Images and Preview route on its Server; a closed Preview has nothing left running. Never for an `image` Application. |
| Known host | One per host (and port). The first clone from a host records its keys; every later clone must see the same ones, or the Deployment fails. Only the Owner can forget a host. |

### Commands

- `Deploy(application)`: queues a manual Deployment, or conflicts if one is already queued.
- `Cancel(deployment)`: a queued one ends `cancelled` at once; a running one
  has its step stopped, its new Container removed, and ends `cancelled`. Once
  the Route has moved it is too late, and the Deployment finishes.
- `Rollback(deployment)`: queues a Deployment with trigger `rollback` that
  runs the given Deployment's Image with today's runtime variables, port,
  Domains, Health check, Persistent storage and Resource limits.
- `ReceivePush(application, headers, body)`: verifies a Webhook call and
  queues a Deployment with trigger `webhook`.
- `ReceivePullRequest(application, headers, body)`: verifies the call like a
  push; opened or reopened opens the Preview and queues its Deployment,
  pushed-to queues its Deployment, closed or merged closes it. Ignored (with
  the reason) when Previews are off, the head is a fork, or the base is not
  the Application's branch.
- `DeployPreview(application, number)`: queues a Deployment of an open
  Preview (trigger `webhook`, or `manual` from the dashboard).
- `ClosePreview(application, number)`: cancels its active Deployment, marks
  it closed, removes what it ran on its Server, and edits the Preview comment
  to say so.
- `RotateWebhookSecret(application)`, `SetAutoDeploy(application, on)`,
  `SetPreviews(application, on, git host token)`.
- `ForgetKnownHost(host)`.
- `PruneImages(server)`: Image retention for every Application whose
  Deployments ran on that Server, with that Server's Podman; registered with
  servers through `servers.OnCleanup`.
- The Worker's steps: `Clone`, `Build`, `Start`, `WaitHealthy`,
  `SwitchRoute` (`SwitchPreviewRoute` for a Preview), `CleanUp` (only
  Containers of the same Preview, or of the Application itself),
  `Fail(reason)`, and for a Preview Deployment that finished or failed
  `CommentPreview` (never fails the Deployment). What comes before `Start`
  depends on the Build pack:
  - `dockerfile`: `Clone`, `Build` the Dockerfile at its path.
  - `nixpacks`: `Clone`, `Plan` (Nixpacks writes `.nixpacks/Dockerfile`
    into the clone), `Build` it with the build variables as build args.
  - `static`: `Clone`, write a generated Containerfile (Caddy serving the
    Publish directory on port 80) into the clone, `Build` it.
  - `image`: `Pull` the Image reference with the Registry credentials,
    record the Source image, tag it as the Deployment's Image.

### Domain events

- `DeploymentFinished { deployment, application, slug, succeeded, reason,
  branch, commit, trigger, rollback, preview }`: a Deployment ended succeeded or
  failed. Not for a cancelled Deployment, nor for one failed because a
  restart interrupted it. Registered with `OnDeploymentFinished(f)`; each
  subscriber runs in its own goroutine so it cannot hold up the Worker.

## Integration

- **Publishes:** the Deployment and its log over HTTP (JSON and SSE), and
  `OnDeploymentFinished` (notifications).
- **Consumes:** `projects.ApplicationForDeploy` (the snapshot is taken once, at
  the start of a Deployment, so editing the Application mid-build does not
  change what is being built; it carries the Deploy key for SSH Sources);
  `routing.SwitchRoute` (with the Target server); `ApplicationDeleted` (the
  Webhook goes too, and Containers and Volumes are removed on every Server
  the Application's Deployments ran on); `servers.Connect`, the Server
  connection every step of a Deployment runs through.
  A push Webhook for an `image` Application is ignored: it has no branch.
  `routing.SwitchPreviewRoute` and `routing.DropPreviewRoute` for Previews.
- **Receives:** Webhook calls from git hosts, unauthenticated but signed:
  pushes and Pull request events.
- **Talks to:** the git hosts' REST APIs (Forgejo and Gitea, GitHub,
  GitLab) to write the Preview comment, with the Git host token; the API
  base comes from the Pull request event.

## Why it's shaped this way

- **The Worker runs inside the API and claims jobs from the `deployments`
  table** with `SELECT ... FOR UPDATE SKIP LOCKED`. No queue broker to run,
  the table is already the source of truth for status, and a second worker
  (later: one per Server) needs no redesign. On start, the API marks
  Deployments left active by a crash as `failed` ("interrupted by restart").
- **The route moves before the old Container goes, and only to a healthy
  one.** A redeploy starts the new Container, waits for its Health check,
  switches the Route, then stops the old one gracefully, so a failed build,
  start or check never takes the running Application down and visitors
  never reach a Container that cannot answer yet.
- **The Health check runs inside the new Container**, through Podman's exec
  API (`curl`, else `wget`, against `127.0.0.1:<port><path>`), as Coolify
  does. The API cannot reach Container addresses in development (it runs on
  the host, and rootless Container addresses live in Podman's network
  namespace), and probing through the proxy would need a Route to a
  Container that is not healthy yet. Podman's own health checks are not
  used: rootless they run from systemd timers, which the API container on a
  server does not have. An image with neither tool fails the check with a
  message saying so, which is why the check is off by default.
- **Podman through its REST API, not the CLI or the official bindings.** The
  bindings pull in most of Podman's dependency tree; the CLI means parsing
  text. The REST client is small and will work unchanged over an
  SSH-tunnelled socket for remote Servers.
- **An Application's Containers are found by label**
  (`bakery.application=<id>`), not by the names Bakery remembers, so a
  Container left over from a crash is still cleaned up by the next Deployment.
  When the Application is deleted (`ApplicationDeleted`), its Containers and
  Deployments go with it.
- **Without a Health check, a Container must stay running for two seconds**
  before the Route moves to it, so an app that crashes on boot fails its
  Deployment instead of taking the traffic.
- **Every log line is stored** (batched inserts), so a reload or a second
  viewer sees the whole log, and failed Deployments keep theirs.
- **Queue behind a running Deployment instead of refusing.** A push during a
  build must not be lost, so one Deployment may wait behind the running one.
  More than one waiting adds nothing: the queued one clones the branch head
  when it starts, which already includes every later push. A webhook call
  that finds one queued is answered "already queued" rather than an error.
- **A webhook Deployment builds the branch head, not the pushed SHA.** After
  a burst of pushes the head is what should run, and the clone stays a
  shallow clone of the branch.
- **One Webhook endpoint for every git host**, recognised by its headers, so
  the Owner copies the same URL whatever hosts the repository. The Webhook
  belongs here, not in projects: it is a way to start a Deployment.
- **Known hosts are trusted on first use and kept in the database.** The API
  container has no persistent home, so a `known_hosts` file would be
  forgotten on every upgrade, and not checking host keys at all would let
  anyone in the middle serve their own code. A changed key fails the
  Deployment with a reason until the Owner forgets the host.
- **Webhook payloads must be JSON.** The signature covers the raw body, and
  a form-encoded body is parsed by the HTTP layer before the Webhook sees it,
  so its original bytes are gone. Every supported git host can send JSON
  (GitHub: content type `application/json`); a form-encoded call is refused
  with a message saying so.
- **Cancel is in-process.** The Worker runs inside the API, so the Service
  keeps a cancel function per running Deployment. A second API process would
  need a `cancel_requested` flag the Worker polls; it is added with remote
  Servers if the Worker moves.
  Podman's build API has no way to stop a build: when Bakery hangs up, the
  step being built runs to its end in the background and its result is
  thrown away (no Image is tagged). The Deployment is cancelled at once and
  the Application can deploy again straight away.
- **A Rollback is a new Deployment**, not a change to the old one, so the
  history stays append-only and its log says what ran. It reuses the earlier
  Image tag, so Images of finished Deployments must be kept. Image retention
  keeps the running Deployment's Image and those of the last five finished
  Deployments per Application; older ones are removed during Cleanup of the
  Server they are on (servers calls the `PruneImages` it registered), and a Rollback to one of those is
  refused with "the image is gone". Five covers the rollbacks people make
  in practice without letting every build stay on disk forever.
- **A pulled image is re-tagged as the Deployment's Image**
  (`localhost/bakery/<slug>:<id>`). Rollback, cleanup and Container naming
  then work the same for every Build pack, and a moving tag like `:latest`
  never changes what an old Deployment rolls back to. The digest is recorded
  as the Source image so the history says exactly what ran.
- **An `image` Deployment has no status of its own for pulling**: it skips
  `cloning` and pulls during `building` ("getting the Image"). Its log says
  what happens, and the dashboard needs no new status.
- **Registry credentials go to Podman per pull** in the `X-Registry-Auth`
  header, never into a Podman `auth.json` and never into the log.
- **Plain-http registries only by allow-list.** `BAKERY_INSECURE_REGISTRIES`
  (comma-separated registry hosts, empty by default) turns TLS verification
  off for those hosts only; development lists the Forgejo stand-in.
- **Nixpacks runs as a subprocess of the API**, like `git`: it only reads the
  clone and writes a Dockerfile, it never talks to a container engine, so
  the rule to reach Podman only through its REST API still holds. The binary
  is pinned in the API image; `BAKERY_NIXPACKS` points at it elsewhere.
- **Volumes are created by the Deployment, removed with the Application.**
  `Start` creates each Persistent storage's volume (if missing) with Bakery's
  labels before creating the Container, so Bakery only ever removes volumes
  it made. On `ApplicationDeleted` the volumes go after the Containers.
  During a zero-downtime switch the old and new Container mount the same
  volume for a moment: an app that cannot share its data directory (SQLite
  with an exclusive lock) sees two writers briefly.
- **Resource limits go to Podman as cgroup limits** (`resource_limits`:
  memory in bytes, CPU as a quota over a 100 ms period). Rootless, that
  needs the `cpu` and `memory` controllers delegated to the Bakery user,
  which the install script makes sure of.
- **Every step runs on the Target server, except the clone.** The clone
  stays on Bakery's side, which holds the Deploy keys and Known hosts, and
  is streamed to the Server's Podman as the build context; the libpod build
  endpoint takes it over any connection, SSH included. So the Image is
  built where it runs and no registry is needed. Building on one Server and
  running on another comes with a registry, later.
- **A Deployment records its Server.** Rollback checks the Image on that
  Server, Image retention prunes per Server, and deleting an Application
  finds every Server its Containers and Volumes may be on without asking
  projects about an Application that no longer exists.
- **A Server that cannot be reached fails the Deployment before anything
  changes**, with the reason, so the running version (if any) stays.
- **Previews only for Pull requests from the same repository.** A Preview
  runs with the Application's runtime variables; building a fork's code with
  them would hand those Secrets to anyone who opens a Pull request. Keeping
  to the same repository also means the head is a branch of the
  Application's own Source, cloned with the same Deploy key.
- **A Preview has its own Volumes but the Application's variables,
  settings and Target server.** A Pull request must never write into
  production's data; everything else is what makes it a faithful copy.
  Preview-specific variables can come later.
- **The Preview belongs to deployments, not projects.** Like the Webhook it
  is a way to run an existing Application, not a new kind of Application:
  it has no settings of its own, and projects' rules (unique Domains, one
  Target server) keep holding for the Application.
- **Deployments stay serial per Application, Previews included.** One
  running Deployment per Application keeps the Worker's claim unchanged;
  one queued per (Application, Preview) keeps a burst of pushes to many
  Pull requests from dropping any of them.
- **Image retention per Preview is one Image.** Previews cannot be rolled
  back, so an open Preview keeps only the Image it runs, and a closed one
  none; production keeps its five for Rollback.
- **One Preview comment, edited.** A Pull request that is pushed to often
  would otherwise fill with Bakery comments. The API base URL is read from
  the Pull request event, so self-hosted Forgejo, Gitea, GitLab and GitHub
  Enterprise work without configuration. A failed comment is logged in the
  Deployment log and never fails the Deployment: the Preview itself works.
- **The Preview domain is `pr-<n>.<primary Domain>`.** It works under
  `*.localhost` with no setup and, on a server, needs one wildcard DNS record
  per Application; certificates are still issued per host.
