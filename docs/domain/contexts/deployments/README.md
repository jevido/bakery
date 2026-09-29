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
| Deployment | One attempt, with a status, the commit it built, its Image and Container. |
| Active | A Deployment in `queued`, `cloning`, `building` or `starting`. |
| Worker | The loop inside the API that claims queued Deployments and runs them. |
| Podman client | Our thin client for the libpod REST API on the rootless socket. |

## Model

### Aggregates

| Aggregate | Invariants |
| --------- | ---------- |
| Deployment | Status only moves forward: `queued` → `cloning` → `building` → `starting` → `finished`, and any active status → `failed` (with an error). An Application has at most one active Deployment; a new Deploy while one is active is refused. Its log is append-only and ordered. |

### Commands

- `Deploy(application)`: queues a Deployment, or conflicts if one is active.
- The Worker's steps: `Clone`, `Build`, `Start`, `SwitchRoute`, `CleanUp`,
  `Fail(reason)`.

### Domain events

None published yet. Notifications will need `DeploymentFinished` and
`DeploymentFailed`; they are added when something consumes them.

## Integration

- **Publishes:** the Deployment and its log over HTTP (JSON and SSE).
- **Consumes:** `projects.ApplicationForDeploy` (the snapshot is taken once, at
  the start of a Deployment, so editing the Application mid-build does not
  change what is being built); `routing.SwitchRoute`.

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
