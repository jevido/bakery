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
| guilds | supporting | `services/api` (`contexts/guilds`) | Guilds, Memberships and their Roles, Permissions, the Guild Master and its Transfer offers, Permission overrides per Project, Invitations, the Current guild of a request |
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
| identity | guilds | customer/supplier | `identity.Authenticate(ctx)` (the Principal: Member, Instance admin flag, the API token's Guild and Permissions, `Allows(permission)`) under every guilds middleware; `identity.ActIn` (the Member's Permission keys in the Current guild) and identity's API token route group, which guilds registers behind its middlewares; `identity.Members`, `identity.MemberByID`, `identity.MemberByEmail`, `identity.ResetTwoFactor` and `identity.RevokeAPITokens` for the Members of a Guild; `identity.OnSetUp` for the first Guild; `identity.SessionMember`, `identity.CreateMember(...)` and `identity.SignIn` when an Invitation is accepted. Identity never imports guilds |
| guilds | projects, deployments, routing, databases, services, servers, notifications | open host service (they are conformist) | The `guilds.Auth` middleware (a signed-in Member with a Membership in the Current guild, or the Instance admin; reading needs `view_resources`), `guilds.Deploy` around deploy actions (the API token's half), `guilds.Can(permission)` naming the Permission every change and every Secret-returning GET needs, `guilds.Owns(name, belongs)` on every route keyed by an id (404 when the owning context's `belongs` says it is outside the Current guild), `guilds.Allows(ctx, permission)` where one response mixes Secrets with public fields, `guilds.InProject(name, projectOf)` on every route under a Project (the Member's Permissions there, after its Permission overrides; 404 when they may not view it), `guilds.Permissions(ctx)` for the Permissions a response reports, `guilds.VisibleProjects(ctx, ids)` filtering a list of Projects, `guilds.ProjectPermissionRoutes` (projects registers a Project's Permission override routes behind its own lookup), `guilds.ForgetProject(ctx, id)` when a Project is deleted, `guilds.IsGuildMaster(ctx, member)` for whatever deletes an account or leaves a Guild, and `guilds.Current(ctx)`, the Guild id every context stores on what it creates and filters by; each owning context registers `guilds.OnGuildDeleting` so a Guild that still owns something is not deleted. The others learn nothing else about Members |
| projects | deployments, routing, databases, services | customer/supplier | `projects.ProjectInGuild`, `projects.EnvironmentInGuild` and `projects.ApplicationInGuild(ctx, id, guildID)`, through `guilds.Owns`, so every route keyed by a Project, Environment, Application or something in one (a Deployment, Database, Scheduled backup, Backup execution, Service) answers 404 outside the Current guild without reading projects' tables. Background paths (deploy workers, Webhooks, probes) ask without a Guild |
| projects | deployments | customer/supplier | `projects.ApplicationForDeploy(id)` returns an `ApplicationSnapshot` (its Guild, Target server, Git repository, Dockerfile path, port, Domains, Persistent storage, Resource limits, decrypted Environment variables and Deploy key) |
| routing | deployments | customer/supplier | `routing.SwitchRoute(serverID, applicationID, domains, container, port)`, called synchronously in a Deployment's route step, so the old Container is removed only after traffic has moved; `routing.SwitchPreviewRoute(serverID, applicationID, preview, domains, container, port)` the same for a Preview, and `routing.DropPreviewRoute(applicationID, preview)` when a Preview closes |
| projects | routing, deployments | published language | `ApplicationDeleted` event: routing drops the Application's Route, Preview routes and Route settings; deployments removes its Containers, Deployments, Previews and Webhook, and its volumes and Images unless the event keeps them |
| projects | databases | customer/supplier | `projects.Environment(id)` (an `EnvironmentSnapshot` with its Project and Guild) places a new Database and names its Project; `projects.OnProjectDeleting` lets databases refuse deleting a Project that still has Databases |
| projects | routing | published language | `ApplicationDomainsChanged { applicationID, domains }` event: routing moves the Application's Route to the new Domains at once |
| projects | services | customer/supplier | `projects.Environment(id)` places a Service; `projects.DomainInUse(domain)` before a Service stores a Domain; services registers `projects.OnDomainCheck` (an Application cannot take a Service's Domain) and `projects.OnProjectDeleting` (a Project with Services is not deleted) |
| routing | services | customer/supplier | `routing.SetServiceRoutes(serviceID, routes)` after a Service is Up, `routing.DropServiceRoutes(serviceID)` before it is deleted |
| servers | deployments | customer/supplier | `servers.Connect(serverID)` gives the Server connection every step of a Deployment runs through; deployments registers its Image retention with `servers.OnCleanup`, called with the Server's id during every Cleanup |
| servers | routing | customer/supplier | `servers.Connect(serverID)` to run and configure the Proxy of a Remote server |
| servers | projects | customer/supplier | `servers.UsableBy(id, guild)` when an Application is created with a Target server (the Local server or one of the Application's Guild); projects registers `servers.OnServerDeleting` so a Server Applications target is not deleted |
| projects, databases, services | servers | customer/supplier | Each registers `servers.OnContainerOwner(kind, inGuild)` for its Containers (`application`, `database`, `service`), so the Local server's Container metrics show a Member only their Current guild's |
| deployments | notifications | published language | `deployments.OnDeploymentFinished { deployment, guild, application, slug, succeeded, reason, branch, commit, trigger, rollback }`: a Deployment ended succeeded or failed (not cancelled, not failed by a restart) |
| databases | notifications | published language | `databases.OnBackupExecutionFinished { backup execution, guild, database, name, type, succeeded, reason, trigger, size, off-site }`: a Backup execution ended (not one failed by a restart) |
| servers | notifications | published language | `servers.OnServerHealthChanged { server, guild, name, change, reason, disk used/total }`: a Server probe found a Server unreachable, reachable again, or its disk usage high; guild is 0 for the Local server, which concerns every Guild |
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
  guilds -->|Auth, Can, Owns, InProject, Current| projects
  guilds --> deployments
  guilds --> routing
  projects -->|ApplicationForDeploy, ApplicationInGuild| deployments
  routing -->|SwitchRoute| deployments
  projects -->|ApplicationDeleted, ApplicationInGuild| routing
  projects -->|ApplicationDeleted| deployments
  projects -->|ApplicationDomainsChanged| routing
  guilds --> databases
  projects -->|Environment, EnvironmentInGuild, ProjectInGuild, OnProjectDeleting| databases
  guilds --> services
  projects -->|Environment, EnvironmentInGuild, ProjectInGuild, DomainInUse, OnDomainCheck, OnProjectDeleting| services
  routing -->|SetServiceRoutes| services
  guilds --> servers
  servers -->|Connect, OnCleanup| deployments
  servers -->|Connect| routing
  servers -->|UsableBy, OnServerDeleting| projects
  guilds -->|OnInvitationCreated| notifications
  deployments -->|OnDeploymentFinished| notifications
  databases -->|OnBackupExecutionFinished| notifications
  servers -->|OnServerHealthChanged| notifications
```
