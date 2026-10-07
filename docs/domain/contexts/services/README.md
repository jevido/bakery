# services

- Subdomain: supporting
- Hosted in: `services/api` (module `contexts/services`)

## Purpose

Runs Services: multi-container software described by a Compose file, from
a Service template in one click or pasted by a member, in an Environment.
Owns the Service, its Components (one Container each), its network, its
volumes and its Service variables, and the catalog of Service templates.
It is **not** responsible for building images (a Component always runs an
`image:`), for Applications and their Deployments, for Databases, or for
serving Domains (routing does, through Service routes).

## Language

| Term | Meaning |
| ---- | ------- |
| Service | A Compose file run as a set of Containers in one Environment, with a name and a Slug. |
| Description | Free text about a Service, at most 255 characters, empty for none. |
| Compose file | The `compose.yml` text a Service is made of, in the subset The Bakery supports. |
| Component | One entry under the Compose file's `services:`, run as the Container `bakery-svc-<service id>-<name>`. The Bakery's word, so "service" never means two things. |
| Public Component | A Component whose environment names `SERVICE_FQDN_<NAME>_<PORT>` or `SERVICE_URL_<NAME>_<PORT>`: it has a port and 1–10 Domains, and the Proxy serves it. |
| Service variable | A `${NAME}` the Compose file refers to, with a value stored encrypted. Set by a member, or generated when it is a Magic variable. |
| Magic variable | A Service variable The Bakery fills in by itself, following Coolify's template convention: `SERVICE_PASSWORD_<X>`, `SERVICE_PASSWORD_64_<X>`, `SERVICE_USER_<X>`, `SERVICE_BASE64_<X>`, `SERVICE_BASE64_64_<X>` (generated once), `SERVICE_FQDN_<NAME>` and `SERVICE_URL_<NAME>` (a Component's primary Domain, without and with `https://`). |
| Service template | A named, described Compose file embedded in The Bakery, with Coolify's category and logo, the catalog a member picks from on the New Resource page. |
| Service network | The network `bakery-svc-<service id>`; every Component is on it under its Component name. |
| Service volume | A named volume of the Compose file, as `bakery-svc-<service id>-<volume>`. |
| Desired state | `running` or `stopped`: what a member asked for. |
| Component status | What a Component's Container is doing, read from Podman: `starting`, `running`, `stopped`, `exited` or `missing`. |
| Service status | Summed up from its Components: `running` (all run), `stopped`, `deploying` (an action is in progress), `degraded` (some do not run), `failed` (the last action failed, with its reason). |
| Up | Pull the images, make the network and volumes, and (re)create and start every Component in `depends_on` order, then switch its Service routes. |
| Redeploy | Up with every image pulled again; volumes and Service variables stay. |

## Model

### Aggregates

| Aggregate | Invariants |
| --------- | ---------- |
| Service | Belongs to one Environment of one Project. Name 1–100 characters; Description at most 255 characters; one created without a name gets its Service template's name, or `docker-compose-<random>` (8 lowercase letters and digits) for a pasted Compose file, as Coolify names it. Slug lowercase `[a-z0-9-]`, unique among Services. The Compose file parses under the supported subset (see below) and has 1–20 Components; Component names match `^[a-z0-9][a-z0-9_-]{0,62}$`; `depends_on` names existing Components without cycles. A Public Component has a port 1–65535 and 1–10 Domains, the first primary, each a lowercase hostname, listed once, unique across The Bakery (Application Domains included) and never the dashboard domain. Service variable names match `^[A-Za-z_][A-Za-z0-9_]*$`; every variable the Compose file refers to has a value or a default. Magic variables are generated once, never set by hand, and kept when the Compose file changes while it still refers to them. Desired state is `running` or `stopped`. |

The supported Compose subset, per Component: `image` (required),
`command`, `entrypoint`, `environment` (map or list), `volumes` (named
volumes only), `depends_on` (list or map; every condition means "started
first"), `working_dir`, `user`, `expose`, `labels`; `restart`,
`healthcheck`, `stop_grace_period`, `hostname` and `x-*` are ignored.
Refused, naming the line and key: `build`, `ports`, bind mounts and
`tmpfs`, `network_mode`, `networks` other than `default`, `privileged`,
`cap_add`, `devices`, `pid`, `ipc`, `extends`, `secrets`, `configs`,
`profiles`, `env_file`, `container_name` and any unknown key. Top level:
`services` (required), `volumes`, `name` and `x-*`.

### Commands

- `CreateService(environment, name, compose)`: parse, generate Magic
  variables, give Public Components their default Domains, store, Up.
- `CreateServiceFromTemplate(environment, template, name?)`: the same with
  the template's Compose file.
- `UpdateService(name?, description?, compose?, domains?, variables?)`: validate and
  store; a changed Compose file keeps Service variables' values and
  Components' Domains by name. Domains of a running Service move at once;
  Compose and variable changes apply on the next Up.
- `StartService`, `StopService`, `RestartService`: set the desired state
  and act on the Containers.
- `RedeployService`: Up, pulling every image again.
- `DeleteService(service, delete volumes?)`: drop its Service routes,
  remove its Containers, its network and (unless delete volumes is off) its
  volumes, then the row.
- `Recover()`: at API start, bring Up (without pulling) every Service that
  should run and has a Component Container missing.

### Domain events

None published yet.

## Integration

- **Publishes:** the Services API (`/api/environments/{id}/services`,
  `/api/projects/{id}/services`, `/api/services/{id}` and its actions and
  per-Component log stream, `/api/service-templates`) for the dashboard.
- **Consumes:** `projects.ProjectOf` (through `guilds.InProject`), so every route keyed by an Environment, Project or Service answers 404 outside the Current guild or a Project the request may not view, and its Permissions count that Project's overrides; `projects.Environment(id)` to place a Service;
  `projects.DomainInUse(domain)` before storing a Domain; registers
  `projects.OnDomainCheck` (Applications cannot take a Service's Domain)
  and `projects.OnProjectDeleting` (a Project with Services is not
  deleted); `routing.SetServiceRoutes` and `routing.DropServiceRoutes` to
  serve Public Components.

## Why it's shaped this way

- **Its own context, not part of deployments or databases.** A Service has
  no Git repository and no build, like a Database, but many Containers, Domains and
  arbitrary images, unlike one. Either host would have to bend its model.
  The name clashes with the repo's `services/` directory; Coolify's word
  wins because it is the one people coming from Coolify know.
- **Component, not "service", for one compose entry**, so a type called
  Service is always the whole thing.
- **The Bakery parses the Compose file and runs it through the libpod API**,
  never `podman compose`: the Podman CLI is off limits and the client must
  work over an SSH-tunnelled socket later. Owning the parser means a
  subset, and a readable refusal (with the line) for the rest instead of
  surprising behaviour. YAML comes from `go.yaml.in/yaml/v3`, a pure
  parser and the one dependency the domain has, as `robfig/cron` is in
  databases.
- **`ports` and bind mounts are refused.** Traffic reaches a Service
  through Domains on the Proxy; host ports and host paths would be the
  first things to collide between Services and the first to break on a
  remote Server. Both can come later on purpose.
- **Magic variables follow Coolify's templates**, so its catalog ports
  over with small changes and people who know Coolify know the names.
  Generated values are stored encrypted and never regenerated: a
  database's password must not change under its data.
- **One network per Service, the Component name as alias**, so compose's
  "reach a service by its name" works unchanged. Only Public Components
  join `bakery`, where the Proxy reaches them; the rest are private to the
  Service. Cost: Applications cannot reach a Service's database yet.
- **Volumes are kept until the Service is deleted**, across Stop,
  Redeploy and Compose changes; a volume the Compose file no longer names
  stays until then too, so a typo never deletes data.
- **Danger Zone offers only "delete volumes".** Coolify's Danger Zone also
  offers keeping the connected networks, keeping configuration files and a
  Docker cleanup. A Service's network is its own (nothing else joins it), so
  it always goes; a Service has no files on the server and builds no images,
  so the other two have nothing behind them. The volume checkbox starts
  ticked, as Coolify's does. Kept volumes are left for an admin to copy data
  out of by hand: The Bakery never reattaches them, because their names carry
  the Service's id and no new Service gets that id.
- **Domains are one namespace with Applications.** projects owns
  Application Domains, services owns Service Domains; each asks the other
  through a published check instead of reading its tables. The unique
  indexes stay per table, so a create racing another create in the other
  context could slip through; that is rare enough to accept.
- **No Deployment history for Services.** A Service is pulled, not built;
  what a member needs is whether it runs and why not. Status is read from
  Podman, only the desired state and the last error are stored, as for
  Databases.
- **Service routes live in routing**, next to Application Routes and
  rendered into the same full Caddy config, so there is still one place
  that knows every Domain the Proxy serves. They get no Route settings yet.
- **Templates are embedded in the API**, not fetched at run time: no calls
  to the outside world, a catalog that matches the code that runs it, and
  images pinned to a major version.
- **Component stays one term.** Coolify splits a Service's entries into
  Service Applications and Service Databases, guessed from the image name,
  and gives the two different pages. The Bakery runs every entry the same way
  and does not model that split; one word for one thing.
- **The sidebar lists only what The Bakery has behind it.** Coolify's
  Service sidebar also has Backups and Import Backup (its per-volume backups
  of a Service Database), Terminal, Scheduled Tasks, Webhooks, Resource
  Operations (clone and move) and Tags. The Bakery backs up Databases only,
  has no terminal, no scheduled tasks, no deploy webhook for a Service, no
  clone or move between Environments and no tags yet; each gets its sidebar
  item when it exists.
- **No Network section.** Coolify lets a Service choose whether its
  containers join the predefined network. Here only Public Components join
  `bakery` and the rest stay on the Service's own network (see above);
  making it a choice would change that rule, not port a page.
- **No configuration checker.** Coolify compares the running containers
  with the saved Compose file and asks for a restart when they differ.
  The Bakery says so once, in the toast after a save: the change applies on
  the next Restart.
- **No per-Component page.** Coolify opens each Service Application or
  Service Database on a page of its own with a human name, a description,
  the image and advanced settings (gzip, strip prefix, log drain, restart
  limits). A Component here is one compose entry and nothing more: its
  card and modal on General show its image, status and Domains and edit
  nothing, its image changes in the Compose file, and its Domains are on
  the Domains page. The card has no per-Component Restart, as there is no
  use case for one. Details shows only what has a value: no Server (a
  Service runs on the local Server) and no Stack Sub-Resources (Components
  have no ids of their own).
- **The Actions menu is Deploy, Restart, Restart (pull latest) and Stop**,
  mapped onto start, restart, redeploy (which pulls every image) and stop.
  Force Deploy, Force Cleanup Containers and Remove container are left out:
  a stopped Service has no containers left to clean up or remove.
- **General has no "Preview generated Compose" or "Validate".** The
  Compose file is parsed and checked when it is saved, and refused with the
  line that fails; nothing is generated from it that anyone would need
  to preview.
- **A running Service shows Running, not "Running (no healthcheck)".** A
  Service is called running only once every Component's container is up
  and past its own healthcheck when it has one, so its pill is green as a
  healthy Database's is.
- **Domains are hostnames, nothing more.** Coolify's Domains page also
  checks DNS, offers Cloudflare and suggested Domains, warns about a
  missing port and sets HTTP → HTTPS, search engine indexing and the www
  redirect. Here a Domain is a hostname served over HTTPS on its
  Component's port, so those have nothing behind them. Generate domain
  gives the Component's default Domain from the API, so the rule stays in
  one place.
- **Environment Variables are the Compose file's, one at a time.** A
  Service's variable names come from its Compose file, so the page has no
  Add variable, no Delete and no Developer view (a `.env` textarea could
  only change values the row dialog already changes). Each row saves on
  its own; generated values are marked Managed and never edited; the
  filter adds one option per Component, as Coolify's does.
- **Persistent Storage is read-only, per Component.** Volumes are the
  Compose file's named volumes, shown with their Podman volume name and
  mount path; adding or removing one is a Compose file change.
- **Runtime Logs read once like a Database's**, with the same lines
  choice, one card per Component.
