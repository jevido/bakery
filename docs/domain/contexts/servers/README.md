# servers

- Subdomain: supporting
- Hosted in: `services/api` (module `contexts/servers`)

## Purpose

The machines The Bakery runs Containers on: the Local server (the rootless Podman
socket of the user running The Bakery) and Remote servers reached over SSH. This
context knows how to reach a Server, whether it is fit to run on
(Validation), what it uses (Server metrics and Container metrics) and how to
free its disk (Cleanup), and it keeps probing every Server to notice one
that goes down or fills up (Server probe).

It also hands other contexts a Server connection, so deployments and routing
can run Containers and a Proxy on any Server. It does **not** decide what
runs where: an Application's Target server is projects' and the Deployment
that runs there is deployments'. Databases and Services still run on the
Local server. It does not keep a metrics history either.

## Language

| Term | Meaning |
| ---- | ------- |
| Server | A machine The Bakery runs Containers on, with a name and an optional description. |
| Local server | The Server The Bakery itself runs on, reached through the rootless Podman socket. Always exists, named `localhost` until renamed; only its name and description can be edited, and it cannot be deleted. |
| Remote server | A Server reached over SSH as a given user, whose rootless Podman API socket is tunnelled through that connection. |
| Private key | The ed25519 key pair The Bakery generates for one Remote server. The Owner adds the public half to the user's `~/.ssh/authorized_keys`; the private half is encrypted at rest. |
| Host key | The SSH host key of a Remote server, pinned on the first connection and required to match on every later one until the Owner forgets it. |
| Validation | The checks run against a Server and their outcome: `ssh` (Remote servers only), `podman` (the API answers, version at least 4.4), `socket`, `linger` and `ports` (unprivileged ports from 80; reported, not required). |
| Server status | `unvalidated` (never checked, or host, port or user changed since), `reachable` (every required check passed) or `unreachable`. |
| Server metrics | CPU use, memory used and total, and disk used and total of Podman's storage on a Server, read live. |
| Server details | The operating system, architecture, kernel, CPU cores, memory, Podman version and boot time of a Server, read live from Podman. |
| Container metrics | CPU and memory use of each Bakery Container on a Server, read live. |
| Server probe | Every 5 minutes, each Server whose latest Validation passed is connected to and its disk read. Two failures in a row make a Reachable Server Unreachable; one success makes an Unreachable one Reachable again. *High disk usage* is raised at 90 % used and cleared below 85 %. |
| Cleanup | Freeing disk on a Server: dangling Bakery images and build layers, and Image retention for the Applications on it. Daily and on demand. |
| Server connection | What other contexts get from `Connect(server)`: the Server's Podman client and a way to open a unix socket on it. Pooled: one SSH connection per Remote server, redialled when it drops, dropped when the Server is edited, deleted or its Host key forgotten. |

## Model

### Aggregates

| Aggregate | Invariants |
| --------- | ---------- |
| Server | The name is unique (1–63 characters); the description is at most 255 characters; host, port and user together are unique. The Local server always exists, takes no host, port or user, and cannot be deleted. Changing host, port or user clears the Host key and makes the Server `unvalidated`. A pinned Host key only changes through Forget host key. The Server status follows its latest Validation or Server probe. Only a probe that changes the status or the high disk usage flag announces anything. |

### Commands

- `Add(name, description, host, port, user)`: a Remote server with a new
  Private key, `unvalidated`.
- `Edit(server, name, description, host, port, user)`: on the Local server
  only the name and description.
- `Delete(server)`: Remote servers only.
  Delete is refused while a registered check says the Server is still in
  use (an Application targets it).
- `Validate(server)`: connects, runs the checks, records the Validation and
  pins the Host key on first contact. A Server that cannot be reached is
  recorded `unreachable` with the reason; that is an outcome, not an error.
- `ForgetHostKey(server)`.
- `ProbeAll()`: a Server probe of every Server whose latest Validation
  passed (one that failed it stays as it is until validated again), every 5 minutes
  (`BAKERY_SERVER_PROBE_INTERVAL`).
- `Metrics(server)`: Server metrics and Container metrics, live.
- `Details(server)`: Server details, live.
- `CleanUp(server)`: runs Cleanup and records when it ran and how much it
  reclaimed.

### Domain events

- `ServerHealthChanged { server, name, change, reason, disk used, disk total }`,
  change being `unreachable`, `reachable` or `server_disk_usage`: a Server
  probe changed what is known about a Server. Registered with
  `OnServerHealthChanged(f)`.

## Integration

- **Publishes:** `Connect(server)` (a Server connection; 0 is the Local
  server; a changed Host key refuses as Validate does), `LocalID()`,
  `Exists(server)`, `OnServerDeleting(check)` (projects: a Server Applications
  target is not deleted) `OnCleanup(retention)` (deployments registers its
  Image retention, called with the Server's id during every Cleanup) and
  `OnServerHealthChanged(f)` (notifications).
- **Consumes:** the auth middleware from identity. Nothing else: servers
  imports no other context, so every context may depend on it.

## Why it's shaped this way

- **Its own context, not part of deployments.** Deployments, Databases,
  Services and the Proxy will all run on Servers, and a Server has a
  lifecycle of its own (keys, Validation, metrics, Cleanup). Keeping it in
  deployments would make three other contexts depend on deployments'
  internals to find a machine.
- **Registered and observed first, deployed to later.** Servers were added,
  validated and observed before anything ran on them; Applications came
  next, Databases and Services follow the same pattern later.
- **Retention is a hook, not a call into deployments.** servers used to call
  `deployments.PruneImages` directly; once deployments and routing needed
  `servers.Connect` that would be an import cycle, so deployments registers
  its retention with `OnCleanup` instead and servers depends on no context.
- **Server connections are pooled.** A Deployment or a log follow makes many
  Podman calls; a new SSH handshake per call would be slow and fill the
  Server's auth log. One connection per Server, checked with a keepalive and
  redialled when dead, is enough for one installation.
- **SSH through `golang.org/x/crypto/ssh`, Podman's socket through a
  `direct-streamlocal` channel.** The libpod client is unchanged: only its
  dialer differs. There is no `ssh -L` process to supervise and no podman
  CLI. Checks libpod cannot answer (linger, the unprivileged port sysctl,
  free disk) run as plain commands in an SSH session.
- **One Private key per Server**, generated by The Bakery, so a key can be revoked
  on one machine without touching the others, and the Owner never hands
  The Bakery a key they use elsewhere.
- **Host keys pinned per Server on first contact**, as Known hosts are for
  git hosts, but owned here: a Server's identity is this context's concern.
  A changed key makes every connection fail until the Owner forgets it, so a
  machine in the middle cannot receive Containers or secrets.
- **Metrics are read live** from the libpod API when asked, not stored.
  A history (graphs, disk usage notifications over time) can be added without
  changing how they are read.
- **Cleanup never touches anything without `bakery.managed=true`**, and which
  Deployment images may go is decided by deployments, which alone knows what
  a Rollback may still need. Images are removed without force, so one a
  Container still uses stays. Removing an Image also removes the build
  layers only it used, which is how build cache goes. Layers of a build that
  failed before tagging carry no label and are left alone: telling them apart
  from another project's would mean guessing.
- **Cleanup runs daily at 03:00 server time** on every Reachable Server, and
  on demand. What it freed is an estimate: an Image's size counts layers it
  may share with Images that stay.
- **A Server probe needs two failures before Unreachable**, and high disk
  usage clears only below 85 %, so one dropped SSH connection or a disk
  hovering at 90 % does not page anyone again and again. The probe reuses
  the Server metrics read, so it adds no check of its own to maintain.
- **Linger is required**: without it the user's Containers stop when their
  last session ends, which on a server is right after The Bakery disconnects.
- **Cleanup, not Docker Cleanup.** Coolify names it after Docker; The Bakery's
  engine is rootless Podman, so the word drops the engine's name. What it
  frees is the same kind of thing: unused images and build layers.
- **Private key is per Remote server.** Coolify keeps Private keys as a
  list a Server picks from; The Bakery generates one for each Remote server, so
  a leaked key opens one machine. The word is Coolify's either way.
