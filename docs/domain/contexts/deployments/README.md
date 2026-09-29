# deployments

- Subdomain: core
- Hosted in: `services/api` (module `contexts/deployments`)

## Purpose

Turns an Application into a running Container: clone the Source, build the
Image with Podman, start the Container, move the Route to it, and remove the
Container it replaces, writing every step to the Deployment log. Also follows
a running Application's Container logs. It is **not** responsible for what an
Application is (projects) or for the Caddy configuration (routing).

## Language

| Term | Meaning |
| ---- | ------- |
| Deployment | One attempt, with a status, its trigger, the branch and commit (SHA, subject, author) it built, its Image and Container. |
| Active | A Deployment in `queued`, `cloning`, `building` or `starting`. |
| Running | A Deployment in `cloning`, `building` or `starting`. |
| Deploy trigger | `manual` or `webhook`. |
| Webhook | The URL and secret a git host calls on push. |
| Known host | A git host's SSH host key, trusted on first use. |
| Worker | The loop inside the API that claims queued Deployments and runs them. |
| Podman client | Our thin client for the libpod REST API on the rootless socket. |

## Model

### Aggregates

| Aggregate | Invariants |
| --------- | ---------- |
| Deployment | Status only moves forward: `queued` → `cloning` → `building` → `starting` → `finished`, and any active status → `failed` (with an error). An Application has at most one queued and at most one running Deployment; a queued one is only picked up once the Application has no running one, and a new Deploy while one is already queued is refused. Its log is append-only and ordered. |
| Webhook | One per Application, with a secret and Auto-deploy on or off. A call is accepted only with a valid signature for that secret (HMAC-SHA256 of the body for GitHub, Gitea and Forgejo; the token for GitLab). Only a push to the Application's branch, with Auto-deploy on, queues a Deployment. |
| Known host | One per host (and port). The first clone from a host records its keys; every later clone must see the same ones, or the Deployment fails. Only the Owner can forget a host. |

### Commands

- `Deploy(application)`: queues a manual Deployment, or conflicts if one is already queued.
- `ReceivePush(application, headers, body)`: verifies a Webhook call and
  queues a Deployment with trigger `webhook`.
- `RotateWebhookSecret(application)`, `SetAutoDeploy(application, on)`.
- `ForgetKnownHost(host)`.
- The Worker's steps: `Clone`, `Build`, `Start`, `SwitchRoute`, `CleanUp`,
  `Fail(reason)`.

### Domain events

None published yet. Notifications will need `DeploymentFinished` and
`DeploymentFailed`; they are added when something consumes them.

## Integration

- **Publishes:** the Deployment and its log over HTTP (JSON and SSE).
- **Consumes:** `projects.ApplicationForDeploy` (the snapshot is taken once, at
  the start of a Deployment, so editing the Application mid-build does not
  change what is being built; it carries the Deploy key for SSH Sources);
  `routing.SwitchRoute`; `ApplicationDeleted` (the Webhook goes too).
- **Receives:** Webhook calls from git hosts, unauthenticated but signed.

## Why it's shaped this way

- **The Worker runs inside the API and claims jobs from the `deployments`
  table** with `SELECT ... FOR UPDATE SKIP LOCKED`. No queue broker to run,
  the table is already the source of truth for status, and a second worker
  (later: one per Server) needs no redesign. On start, the API marks
  Deployments left active by a crash as `failed` ("interrupted by restart").
- **The route moves before the old Container goes.** A redeploy starts the new
  Container, switches the Route, then removes the old one, so a failed build
  or start never takes the running Application down. This is not yet
  zero-downtime (no health check before switching).
- **Podman through its REST API, not the CLI or the official bindings.** The
  bindings pull in most of Podman's dependency tree; the CLI means parsing
  text. The REST client is small and will work unchanged over an
  SSH-tunnelled socket for remote Servers.
- **An Application's Containers are found by label**
  (`bakery.application=<id>`), not by the names Bakery remembers, so a
  Container left over from a crash is still cleaned up by the next Deployment.
  When the Application is deleted (`ApplicationDeleted`), its Containers and
  Deployments go with it.
- **A Container must stay running for two seconds** before the Route moves to
  it, so an app that crashes on boot fails its Deployment instead of taking
  the traffic.
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
