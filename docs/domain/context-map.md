# Context map

The bounded contexts in this project, which unit hosts each one, and how they
depend on each other.

## Contexts

| Context | Subdomain | Hosted in | Owns |
| ------- | --------- | --------- | ---- |
| deployments | core | `services/api` (`contexts/deployments`) | Deployments, their logs, Webhooks, Previews and Known hosts, talking to Podman (builds, containers) |
| projects | supporting | `services/api` (`contexts/projects`) | Projects, Environments, Applications, Environment variables |
| routing | supporting | `services/api` (`contexts/routing`) | Routes, Service routes, Route settings and the Proxies (a Caddy container per Server and its config) |
| databases | supporting | `services/api` (`contexts/databases`) | Databases, their Containers and volumes, Scheduled backups, Backup executions and S3 storages |
| services | supporting | `services/api` (`contexts/services`) | Services, their Components, networks, volumes and Service variables, and the Service templates |
| servers | supporting | `services/api` (`contexts/servers`) | Servers, their Private keys and Host keys, Server connections, Validation, metrics and Cleanup |
| guilds | supporting | `services/api` (`contexts/guilds`) | Guilds, Memberships and their Roles, Invitations, the Current guild of a request |
| identity | generic | `services/api` (`contexts/identity`) | Members, the Instance admin, Setup, Sessions, API tokens, Profiles and Two-factor authentication |
| notifications | generic | `services/api` (`contexts/notifications`) | Notification channels and their Deliveries |

- **Core:** where the project competes. Gets the most care and the richest model.
- **Supporting:** needed and specific to this project, but not the differentiator.
- **Generic:** solved problems (auth, billing, email). Prefer buying or reusing
  over building.

## Relationships

One row per dependency. Upstream is the side whose model the other has to
adapt to.

| Upstream | Downstream | Pattern | Through |
| -------- | ---------- | ------- | ------- |
| identity | guilds | customer/supplier | `identity.Authenticate(ctx)` (the Principal: Member, Instance admin flag, the API token's Guild and Permissions, `Allows(permission)`) under every guilds middleware; `identity.ActIn` and identity's API token and Invitation route groups, which guilds registers behind its middlewares; `identity.Members`, `identity.ResetTwoFactor` and `identity.RevokeAPITokens` for the Members of a Guild; `identity.OnMemberAdded` for the first Guild and new Memberships; `identity.CreateMember(...)` when an Invitation to a new email is accepted. Identity never imports guilds |
| guilds | projects, deployments, routing, databases, services, servers, notifications | open host service (they are conformist) | The `guilds.Auth` middleware (a signed-in Member with a Membership in the Current guild, or the Instance admin; viewers refused on every change), `guilds.Deploy` around deploy actions, `guilds.Admin` around admin-only areas, `guilds.Secrets` around GETs that return Secrets, `guilds.CanSeeSecrets(ctx)` where one response mixes Secrets with public fields, and `guilds.Current(ctx)`, the Guild id every context stores on what it creates and filters by; each owning context registers `guilds.OnGuildDeleting` so a Guild that still owns something is not deleted. The others learn nothing else about Members |
| projects | deployments | customer/supplier | `projects.ApplicationForDeploy(id)` returns an `ApplicationSnapshot` (Target server, Git repository, Dockerfile path, port, Domains, Persistent storage, Resource limits, decrypted Environment variables and Deploy key) |
| projects | routing | customer/supplier | `projects.ApplicationExists(id)`, so the Route settings API answers 404 for an unknown Application without reading projects' tables |
| routing | deployments | customer/supplier | `routing.SwitchRoute(serverID, applicationID, domains, container, port)`, called synchronously in a Deployment's route step, so the old Container is removed only after traffic has moved; `routing.SwitchPreviewRoute(serverID, applicationID, preview, domains, container, port)` the same for a Preview, and `routing.DropPreviewRoute(applicationID, preview)` when a Preview closes |
| projects | routing, deployments | published language | `ApplicationDeleted` event: routing drops the Application's Route, Preview routes and Route settings; deployments removes its Containers, Deployments, Previews and Webhook, and its volumes and Images unless the event keeps them |
| projects | databases | customer/supplier | `projects.Environment(id)` places a new Database and names its Project; `projects.OnProjectDeleting` lets databases refuse deleting a Project that still has Databases |
| projects | routing | published language | `ApplicationDomainsChanged { applicationID, domains }` event: routing moves the Application's Route to the new Domains at once |
| projects | services | customer/supplier | `projects.Environment(id)` places a Service; `projects.DomainInUse(domain)` before a Service stores a Domain; services registers `projects.OnDomainCheck` (an Application cannot take a Service's Domain) and `projects.OnProjectDeleting` (a Project with Services is not deleted) |
| routing | services | customer/supplier | `routing.SetServiceRoutes(serviceID, routes)` after a Service is Up, `routing.DropServiceRoutes(serviceID)` before it is deleted |
| servers | deployments | customer/supplier | `servers.Connect(serverID)` gives the Server connection every step of a Deployment runs through; deployments registers its Image retention with `servers.OnCleanup`, called with the Server's id during every Cleanup |
| servers | routing | customer/supplier | `servers.Connect(serverID)` to run and configure the Proxy of a Remote server |
| servers | projects | customer/supplier | `servers.Exists(id)` when an Application is created with a Target server; projects registers `servers.OnServerDeleting` so a Server Applications target is not deleted |
| deployments | notifications | published language | `deployments.OnDeploymentFinished { deployment, application, slug, succeeded, reason, branch, commit, trigger, rollback }`: a Deployment ended succeeded or failed (not cancelled, not failed by a restart) |
| databases | notifications | published language | `databases.OnBackupExecutionFinished { backup execution, database, name, type, succeeded, reason, trigger, size, off-site }`: a Backup execution ended (not one failed by a restart) |
| servers | notifications | published language | `servers.OnServerHealthChanged { server, name, change, reason, disk used/total }`: a Server probe found a Server unreachable, reachable again, or its disk usage high |
| guilds | notifications | customer/supplier | `guilds.OnInvitationCreated { guild, email, role, invited by, link, expires }`, called synchronously; notifications answers whether it emailed the link |

## External systems

| System | Used by | Through |
| ------ | ------- | ------- |
| Podman (rootless libpod API) | deployments, routing, databases, services, servers | The Bakery's own thin client in `app/podman`, over the local socket or tunnelled over SSH |
| SSH | servers (for deployments and routing through Server connections) | `golang.org/x/crypto/ssh`: Podman's socket and the Remote Proxy's admin socket through `direct-streamlocal` channels, plain commands for checks |
| Caddy admin API | routing | JSON config loaded with `POST /load` |
| S3-compatible storage | databases | Its own thin S3 client (Signature V4), for Backup executions |
| Git hosts' REST APIs (Forgejo and Gitea, GitHub, GitLab) | deployments | Plain HTTPS calls with the Git host token, to write the Preview comment |
| SMTP servers | notifications | `net/smtp`, with the settings of an email Notification channel |
| Discord, Slack, Telegram, ntfy, webhook receivers | notifications | Plain HTTPS POSTs, one small sender per Channel kind |

Patterns: *customer/supplier*, *conformist*, *anticorruption layer*,
*open host service* / *published language*, *shared kernel*, *separate ways*.
A shared kernel is a deliberate exception and needs a line saying why.

## Diagram

```mermaid
flowchart LR
  identity -->|Authenticate, CreateMember| guilds
  guilds -->|Auth, Admin, Secrets, Current| projects
  guilds --> deployments
  guilds --> routing
  projects -->|ApplicationForDeploy| deployments
  routing -->|SwitchRoute| deployments
  projects -->|ApplicationDeleted| routing
  projects -->|ApplicationDeleted| deployments
  projects -->|ApplicationDomainsChanged| routing
  guilds --> databases
  projects -->|Environment, OnProjectDeleting| databases
  guilds --> services
  projects -->|Environment, DomainInUse, OnDomainCheck, OnProjectDeleting| services
  routing -->|SetServiceRoutes| services
  guilds --> servers
  servers -->|Connect, OnCleanup| deployments
  servers -->|Connect| routing
  servers -->|Exists, OnServerDeleting| projects
  guilds -->|OnInvitationCreated| notifications
  deployments -->|OnDeploymentFinished| notifications
  databases -->|OnBackupExecutionFinished| notifications
  servers -->|OnServerHealthChanged| notifications
```
