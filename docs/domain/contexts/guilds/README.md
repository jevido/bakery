# guilds

- Subdomain: supporting
- Hosted in: `services/api` (module `contexts/guilds`)

## Purpose

Knows *where* a request acts and *with which Permissions*: the Guilds of
this installation, their Roles and Guild Master, each Member's Membership
(and its Roles) in them, the Permission overrides on Projects, the
Invitations that bring people into a Guild, and the Current guild of every
request. Every Project, Server (but the Local server), S3 storage,
Notification channel, Known host and API token belongs to exactly one
Guild, and the other contexts ask this one which Guild that is and what the
Member may do in it.

It is **not** responsible for *who* someone is: Members, their passwords,
Sessions, API tokens, Profile and Two-factor authentication stay in
identity. It does not own the resources either; each context keeps its own
rows and stores the Guild's id on them.

## Language

| Term | Meaning |
| ---- | ------- |
| Guild | A group of Members that owns Projects, Servers (all but the Local server), S3 storages, Notification channels, Known hosts, API tokens, Goals and Issues. Name, optional description and Issue prefix. Coolify's Team, Paperclip's Company. |
| Membership | A Member's place in a Guild, holding any number of Roles and always the Base role. At most one per Member and Guild. |
| Agent membership | An Agent's place in its Guild: a Membership whose member is an Agent, with its Hirer. It holds Roles like any Membership but never makes the Agent a Member. |
| Role | A named set of Permissions in one Guild, with a color and a Position. Seeded in every Guild: `Admin` (Administrator), `Member` (View resources, See secrets, Deploy, Manage applications, Manage work), `Viewer` (View resources). Discord's role. |
| Base role | The Role every Member holds, shown as `@everyone`, at Position 0. Cannot be assigned, removed, renamed or deleted; only its Permissions change. Seeded with none. Discord's `@everyone`. |
| Position | A Role's place in its Guild's order, higher above lower. A Member's highest Role is the highest-placed Role they hold. |
| Permission | One of the fixed list in the glossary (`administrator`, `view_resources`, `see_secrets`, `deploy`, `manage_applications`, `manage_servers`, `manage_notifications`, `manage_guild`, `manage_members`, `manage_roles`, `hire_agents`, `approve`, `manage_budgets`, `manage_work`, `manage_skills`). A Member's Permissions in a Guild are the union of their Roles'; `administrator` grants every one. |
| Guild Master | The one Member of a Guild above every Role, with every Permission, never removable or re-roled; changes only by an accepted Transfer offer. Discord's server owner. |
| Transfer offer | The Guild Master's offer of the Guild Master to one other Member; accepted, declined, withdrawn or expired after 7 days. |
| Permission override | On one Project, per Role or per Member: allow, deny or inherit for `view_resources`, `see_secrets`, `deploy` or `manage_applications`. |
| Instance admin | The Member Setup creates. Acts with every Permission in every Guild, just below its Guild Master, and sees every Guild, with or without a Membership. |
| Current guild | The Guild a request acts in: the `bakery_guild` cookie for a Session, the Guild an API token was made in for a token, the Guild the `Bakery-Guild` header names for a Desktop key (403 `not a member of this guild` when the Member may not act there; their first Guild without the header). |
| Invitation | An email and the Roles it brings, for one Guild, with a link that is good once and for 7 days. |
| Guild switcher | Where a Member picks the Current guild among the Guilds they are in (all of them for the Instance admin). |

## Model

### Aggregates

| Aggregate | Invariants |
| --------- | ---------- |
| Guild | Name is 1–255 characters after trimming; the description is optional, at most 255 characters. Deleted only when it owns nothing: no Projects, Servers, S3 storages, Notification channels, Goals, Issues or Agents that are not terminated (each owning context answers through `OnGuildDeleting`); its Memberships (Agent memberships too), Invitations, API tokens and Known hosts go with it. |
| Guild (Guild Master) | Exactly one Guild Master at all times, a Member with a Membership in the Guild. The Guild Master is never removed from the Guild, never re-roled by anyone, cannot leave it, and cannot have their account deleted while they hold it; they transfer it first or delete the Guild. At most one open Transfer offer, to another Member of the Guild who is a person (never an agent); it changes nothing until accepted, can be withdrawn by the Guild Master and declined by its Member, and expires 7 days after it was made. Accepting swaps the Guild Master in one step; both keep their other Roles. |
| Role | Belongs to one Guild. Name is 1–100 characters after trimming; color is `#rrggbb`; Permissions are from the fixed list. Positions are unique per Guild. The Base role is at Position 0, always exists, and cannot be renamed, deleted, assigned or removed; every other Role sits above it. A Role held by Members can be deleted; they simply stop holding it. |
| Membership | One per Member and Guild. Holds a set of Roles of its own Guild (never the Base role explicitly; it holds that implicitly). The Instance admin's Memberships are never removed. |
| Agent membership | One per Agent, in the Agent's Guild. Has an Agent and a Hirer (a Member with a Membership in the same Guild), never a person as its member. Each of its Roles ranks below the Hirer's highest Role; when the Hirer's highest Role drops (re-roled, or a Role moved or deleted), the Roles at or above the new one are removed in the same change. It is never the Guild Master, never offered a Transfer, never invited and never listed with the Members, and `IsMember` is false for it. It is ended when its Agent is terminated. |
| Permission overrides | Keyed by one Project of the Guild. Each entry is a Role or a Member, a Permission from `view_resources`, `see_secrets`, `deploy` and `manage_applications`, and allow or deny (inherit is no entry). For a Member on that Project: start from their Guild Permissions; then the Base role's entry; then, over all the other Roles they hold, a deny removes the Permission and otherwise an allow adds it; then the Member's own entry, if any, decides. Deleting the Project, the Role or the Member's Membership deletes its entries. `administrator`, the Instance admin and the Guild Master skip overrides. |
| Invitation | Belongs to one Guild. Email is valid and not already a Member of this Guild; its Roles are Roles of that Guild below the inviter's highest Role (none means the Base role only); at most one open Invitation per Guild and email; expires 7 days after it was made; accepted at most once; a revoked, declined or expired one cannot be accepted. Only the hash of its token is stored. |

### The hierarchy

- The Guild Master is above every Role and every Member of the Guild. The
  Instance admin comes next in every Guild, above every Role. Everyone else
  is placed by their highest Role.
- A Member can create, edit, delete, reorder, assign or remove only Roles
  whose Position is below their own highest Role (and with `manage_roles`).
- A Member can manage, kick or re-role only Members whose highest Role is
  below their own highest Role (and with the Permission the action needs).
- Nobody changes their own Roles or removes their own Membership; a Member
  leaves a Guild instead (but the Guild Master cannot).
- A Member can give a Role only Permissions they have themselves.
- An Agent holds Roles through its Agent membership like any Member, and is
  never placed above its Hirer: it is only given Roles below the Hirer's
  highest Role, by someone who could assign them (`CanAssign`), and loses
  any that end up at or above it: in the same transaction as the change to
  Roles or Memberships that put them there, and right after an accepted
  Transfer offer that leaves the old Guild Master ranking by their Roles.

### Commands

Who may run each is in brackets, by Permission in the Guild concerned. The
Guild Master and the Instance admin hold every Permission; every command
that touches a Role or a Member also follows the hierarchy above.

- `CreateGuild(name, description)` [any Member, with a Session]: the creator
  gets a Membership with the `Admin` Role and is the Guild Master; the Guild
  gets the seeded Roles and becomes their Current guild; it owns nothing and
  deploys to the Local server.
- `RenameGuild(name)`, `ChangeDescription(description)`, `ChangeIssuePrefix(prefix)` [`manage_guild`].
- `DeleteGuild()` [`administrator`]: refused while the Guild owns anything, naming
  what (`projects`, `servers`, `s3 storages`, `notification channels`). The
  first Guild is not special, and a Member may delete their last Guild.
- `SwitchGuild(guild)` [any Member of that Guild, the Instance admin for any;
  with a Session]: sets the Current guild of the Session.
- `Invite(email, roles)` [`manage_members`; the Roles below one's own]:
  returns the Invitation and its link, once.
- `RevokeInvitation(id)` [`manage_members`].
- `DeclineInvitation(token)` [anyone with the link]: the invited person
  turns an open Invitation down; it stops working as a revoked one does.
- `AcceptInvitation(token, name, password)` [anyone with the link]: when the
  email is new, creates the Member (through identity) with a Membership of
  the invited Roles and signs them in; when a Member already has the email,
  that Member, signed in, gets the Membership (no name or password; without
  their Session it is refused with "sign in as <email> to accept"). Either
  way the invited Guild becomes the Current guild.
- `CreateRole(name, color, permissions)` [`manage_roles`]: placed just
  above the Base role.
- `EditRole(role, name, color, permissions)` [`manage_roles`]: only a Role
  below one's own highest; the Base role's Permissions only.
- `DeleteRole(role)` [`manage_roles`]: only a Role below one's own highest,
  never the Base role.
- `ReorderRoles(order)` [`manage_roles`]: moves only Roles below one's own
  highest, and only among Positions below it.
- `AssignRole(membership, role)`, `RemoveRole(membership, role)`
  [`manage_roles`]: the Role below one's own highest, the Member's highest
  Role below one's own, never one's own Membership, never the Guild Master's.
- `RemoveMembership(membership)` [`manage_members`]: never your own, never
  the Guild Master's or the Instance admin's, only a Member below one's own
  highest Role; the Member keeps their other Memberships, and their API
  tokens made in this Guild stop working at once.
- `ResetMemberTwoFactor(membership)` [`manage_members`]: only a Member below
  one's own highest Role.
- `OfferGuildMaster(membership)` [the Guild Master]: to another Member of
  the Guild who is a person; replaces nothing while an offer is open.
- `WithdrawOffer()` [the Guild Master].
- `AcceptOffer()`, `DeclineOffer()` [the Member it was offered to]:
  accepting makes them the Guild Master in one step.
- `SetOverride(project, role or member, allow, deny)` [`manage_roles` in
  that Project]: for a Role below one's own highest (the Base role
  included) or a Member whose highest Role is below it, never oneself, the
  Guild Master or the Instance admin; only switching Permissions one holds
  in that Project. Both empty is inherit for every Permission and deletes
  the entry.
- `ForgetProject(project)` [projects, when it deletes a Project]: its
  Permission overrides go.

### Domain events

- `SetUp` (identity's, consumed): Setup made the Instance admin, and guilds
  makes the first Guild, "Default", with the seeded Roles and their
  Membership holding `Admin`, and makes them its Guild Master, unless a
  Guild exists already.

- `InvitationCreated { guild, email, roles, invited by, link, expires }`: an
  Invitation was made. Its one subscriber (notifications, with
  `OnInvitationCreated(f)`) is called synchronously and answers whether it
  emailed the link, which the invite response reports as `emailed`.
- `GuildDeleting { guild }`: asked synchronously before a Guild is deleted;
  every context that owns rows for Guilds registers `OnGuildDeleting(f)` and
  refuses while the Guild still owns something there.

## Integration

Every check asks for a Permission by its wire key, never for a Role. A
request's Permissions are its Member's in the Current guild (every one for
the Instance admin), and for an API token only what its Token permissions
cover: `see_secrets` with `read:sensitive`, `deploy` with `deploy`, the
other changes with `write`, `administrator` only with `root`.

- **Publishes:**
  - `guilds.Auth`: the request comes from a Member (by `identity.Authenticate`)
    with a Membership in the Current guild, or from the Instance admin;
    GET and HEAD need `view_resources` there (403 `you need the View
    resources permission`), and an API token is refused what its Token
    permissions do not cover by method, as identity describes them. It
    refuses no change by itself: every change route names its Permission
    with `guilds.Can`. For an Agent principal (a Run key), its Guild is
    the Run's (a `Bakery-Guild` header naming another is 403), its
    Permissions are its Agent membership's, Project overrides resolved as
    for a Member, never capped like an API token, and it is let in only
    on routes behind `guilds.AuthAgents`; every route behind `guilds.Auth`
    answers it 403 `agents cannot use this route`.
  - `guilds.AuthAgents`: `guilds.Auth` for a route open to Agent principals
    as well (`Auth{Agents: true}`); the owning context still names the
    route's Permission with `guilds.Can`. An Agent principal that is no
    longer in its Run's Guild is 403 `not a member of this guild`.
  - `guilds.Deploy`: `guilds.Auth` for Coolify's deploy actions, where an
    API token needs the `deploy` Token permission instead of `write`.
  - `guilds.Can(permission)`: after `guilds.Auth`, only requests that may
    use that Permission in the Current guild (403 `you need the <Name>
    permission`, the glossary's name; for an API token whose Member holds
    it but whose Token permissions do not cover it, Coolify's `Missing
    required permissions: <token permission>`). An unknown key panics at
    boot. Changes in projects, routing, databases (not S3 storages),
    services and deployments need `manage_applications`, deploy actions
    `deploy`, GETs that return Secrets `see_secrets`; Servers, S3 storages
    and Known hosts `manage_servers` (Known hosts for reading too, as they
    were admin-only); Notification channels `manage_notifications`, reads
    included; the Current guild's General `manage_guild`, deleting it
    `administrator`, Members and Invitations `manage_members`.
  - `guilds.Allows(ctx, permission)`: the same question inside a handler
    whose answer differs by Permission, e.g. Secrets in a response a
    viewer may also read.
  - `guilds.InProject(name, projectOf)`: after `guilds.Auth`, on every
    route whose `{id}` is a Project or something in one (Environments,
    Applications, Deployments, Databases, Scheduled backups, Backup
    executions, Services, variables, routing): 404 when it is outside the
    Current guild or in a Project where the request lacks `view_resources`;
    otherwise the request's Permissions are resolved in that Project, so
    `guilds.Can` and `guilds.Allows` after it count its Permission
    overrides. `projectOf(id)` is the owning context's lookup answering the
    Project and Guild (`projects.ProjectOf(kind)` for `project`,
    `environment` and `application`; databases, services and deployments
    reach theirs through it). Routes keyed by no Project (`POST
    /api/projects`, `GET /api/projects`) keep the guild-wide check.
  - `guilds.VisibleProjects(ctx, ids)`: the Projects among ids the request
    may view, with one read of the Guild's overrides; a list across
    Projects (`GET /api/projects`) drops the others.
  - `guilds.Permissions(ctx)`: the request's Permissions as wire keys, in
    its Project after `guilds.InProject`.
  - `guilds.ForgetProject(ctx, project)` and
    `guilds.ProjectPermissionRoutes(r, projectOf)`: projects calls the first
    from its delete use case and registers the second, since guilds cannot
    find a Project itself.
  - `guilds.Owns(name, belongs)`: after `guilds.Auth`, 404 for a route
    whose `{id}` names something outside the Current guild that is in no
    Project; `belongs(id, guild)` is the owning context's check (e.g.
    databases' S3 storages).
  - `guilds.Current(ctx) uint64`: the Current guild's id, which every other
    context stores on what it creates (or reaches through something that
    does) and filters every list and read by.
  - `guilds.MemberID(ctx)`, `guilds.AgentID(ctx)` and `guilds.RunID(ctx)`:
    who is asking. For an Agent principal `MemberID` is 0, `AgentID` names
    the Agent and `RunID` the Run its Run key belongs to; for a person
    `AgentID` and `RunID` are 0. A handler that answers what the asker may
    manage answers false for an Agent: an Agent manages nothing.
  - `guilds.IsGuildMaster(ctx, member) bool`: whether the Member is the
    Guild Master of any Guild. Whatever deletes an account or lets a Member
    leave a Guild asks it first and refuses while it is true.
  - `guilds.IssuePrefix(ctx, guild) string`: the Guild's Issue prefix,
    which work renders Issue identifiers from when it reads them.
  - `guilds.IsMember(ctx, guild, member) bool`: whether the Member holds a
    Membership in the Guild; work asks it before making someone an
    Assignee or a Goal's owner.
  - `guilds.OnGuildDeleting(kind, f)` and `guilds.OnInvitationCreated(f)`:
    see Domain events. Projects, servers, databases (S3 storages),
    notifications, work and agents register `OnGuildDeleting`.
  - Agent memberships, for the agents context: `guilds.JoinAgent`
    (create one for an Agent with its Hirer and Roles),
    `guilds.AssignAgentRole` / `guilds.RemoveAgentRole` (give or take one
    of its Roles; the caller has already checked that the actor may manage
    the Agent, so `manage_roles` is not asked), `guilds.LeaveAgent` (end
    it), `guilds.AgentRoles` and `guilds.AgentPermissions` (the union of
    its Roles, as for any Membership), `guilds.AgentCanIn` (whether the
    Agent may use a Permission in one of the Guild's Projects, its Roles'
    Permission overrides there counted as InProject counts a person's),
    `guilds.RoleNames` (Role names for
    a payload) and `guilds.RankAbove` (whether one person ranks above
    another). Giving a Role at or above the Hirer's highest is refused
    (`guilds.ErrAboveHirer`, 422), and so is a Role with a Permission the
    Hirer (at the hire) or the asking person (later) does not hold. The Roles an Agent loses because its
    Hirer dropped are taken silently, with no Activity event. agents
    registers `guilds.OnMemberLeaving`, called once a Member is removed
    from a Guild (there is no other way to leave one yet, and no account
    deletion), which terminates the Agents they hired there.
- **Serves:** `GET /api/me` (the Member, their `permissions` in the Current
  guild as wire keys, `administrator` meaning every one, the former `role`
  derived from them for scripts, `instance_admin`, `guild_master` (whether
  they are the Current guild's), `offers` (the open Transfer offers to them
  in any Guild: `id`, `guild` `{id, name}`, `from`, `to`, `created_at`,
  `expires_at`), the Current `guild` and every Guild they may switch to
  with their `permissions` and `role` there). The Guilds, with a Session only and also for a Member in no
  Guild: `GET /api/guilds` (every Guild they may switch to, with their
  `permissions` and `role`), `POST /api/guilds` (`{"name", "description"}`; 201, and
  it becomes the Current guild) and `POST /api/guilds/{id}/switch` (204; 404
  for a Guild they may not act in). The Current guild:
  `GET /api/guilds/current` (`name`, `description`, `blocking`, what
  keeps it from being deleted, `guild_master` `{id, name, email}` and
  `offer`, the open Transfer offer or null) for every Member, `PATCH /api/guilds/current`
  (`{"name", "description", "issue_prefix"}`, `issue_prefix` left as it is when absent; 422 for an invalid one or one another Guild has) with `manage_guild` and `DELETE
  /api/guilds/current` (204, or 409 with `blocking`) with `administrator`.
  The Members of the Current guild: `GET /api/members` for every Member
  (each with `roles`, `[{id, name, color}]` top first, besides the former
  `role`), and with `manage_members` `DELETE /api/members/{id}` and
  `DELETE /api/members/{id}/two-factor`. `PATCH /api/members/{id}` answers
  410 Gone, naming the routes that replace it. On the wire the Instance admin's `role` reads `owner`, with
  `instance_admin: true`; the Guild Master comes first in the list, marked
  `guild_master: true`. The Roles of the Current guild: `GET /api/roles`
  for every Member (`id`, `name`, `color`, `position`, `permissions`,
  `base`, `members`, top first and the Base role last) and, with
  `manage_roles` and by the hierarchy, `POST /api/roles` (`{"name",
  "color", "permissions"}`; 201, at Position 1), `PATCH /api/roles/{id}`
  (any of those three), `DELETE /api/roles/{id}` (204), `PUT
  /api/roles/order` (`{"role_ids"}`, top first, every Role but the Base
  role; answers the Roles), `PUT` and `DELETE
  /api/members/{id}/roles/{role_id}` (the `member` with their `roles`).
  Another Guild's Role answers 404, a Role at or above one's own highest,
  the Base role's name or color, or a Permission one lacks 403.
  `GET /api/permissions` lists the fixed list (`key`, `name`,
  `description`, `overridable`) for every Member. A Project's Permission
  overrides, with `manage_roles` in that Project: `GET
  /api/projects/{id}/permissions` (`overrides`, each `{role_id,
  member_id, allow, deny}` with one id null), `PUT
  /api/projects/{id}/permissions/roles/{role_id}` and
  `.../members/{member_id}` (`{"allow", "deny"}`, wire keys; both empty
  deletes it; answers the `override`) and `DELETE` of either (204). A
  Permission that cannot be overridden or is both allowed and denied
  answers 422 on `allow`; a Role or Member not below one's own, or a
  Permission one lacks there, 403; another Guild's Role 404. `GET
  /api/projects/{id}` adds `permissions`, the request's there. Transfer offers, with a Session only:
  `POST /api/guilds/current/guild-master-offer` (`{"member_id"}`; 201 with
  the `offer`; 403 for anyone but the Guild Master, 422 for themselves or
  someone outside the Guild, 409 while one is open) and `DELETE
  /api/guilds/current/guild-master-offer` (withdraw; 204, 404 without
  one) for the Current guild; `POST /api/guild-master-offers/{id}/accept`
  and `/decline` (204) for the offered Member, in whichever Guild is
  Current, 404 for anyone else and 409 `this offer has expired` or `this
  offer is no longer open`. The Invitations of the Current guild, with
  `manage_members`:
  `GET /api/invitations`, `POST /api/invitations` (`{"email",
  "role_ids"}`, each a Role the inviter may assign, none for the Base role
  only; `{"role": "member"}` still names the seeded Role of that name) and
  `DELETE /api/invitations/{id}`; an Invitation shows its `roles` and the
  former `role` they read as; another Guild's id answers 404. Open to
  anyone with the link: `GET /api/invitations/by-token/{token}` (the
  Invitation, its `guild` and `existing_member`) and
  `POST /api/invitations/by-token/{token}/accept` and
  `POST /api/invitations/by-token/{token}/decline` (204).
- **Consumes:** identity's `identity.Authenticate(ctx)` (a Principal: the
  Member, whether they are the Instance admin, and for an API token its
  Guild and Permissions; for a Run key the Agent principal), `identity.ActIn`, identity's API token routes
  (registered behind guilds' middlewares), `identity.Members`,
  `identity.MemberByID`, `identity.MemberByEmail`,
  `identity.ResetTwoFactor`, `identity.RevokeAPITokens`,
  `identity.OnSetUp(f)`, and for an accepted Invitation
  `identity.SessionMember(ctx)` (who is signed in, without answering 401),
  `identity.CreateMember(...)` and `identity.SignIn(ctx, member)`. Identity
  never imports guilds.

## Why it's shaped this way

- **A context of its own rather than a larger identity.** The goal makes
  the Guild a bounded context of its own. Identity answers *who* sent a
  request; guilds answers *where* it acts and *with which Role*. The Role,
  Admin and Secrets checks move here on top of `identity.Authenticate`, and
  identity registers hooks instead of importing guilds, so the two cannot
  form a cycle.
- **Membership, not renaming Member to Account.** Coolify's Members page and
  Paperclip's members keep the word Member for the person, and the glossary
  already refuses "account". A Member's place in a Guild gets its own word,
  Membership, as Paperclip's `company_memberships`.
- **Roles are enforced by coarse middlewares, not per route checks.**
  `Auth` refuses changes by viewers, `Admin` wraps whole admin areas,
  `Secrets` wraps the GETs that return Secrets, each reading the Membership
  in the Current guild on every request, so a demotion counts at once. It
  covers every context with a few lines each, and a new route is safe by
  default.
- **The Instance admin acts as admin in every Guild.** It is Paperclip's
  instance admin, and nothing that the Owner could do before Guilds stops
  working. Some Guild always has someone who can manage it, even after its
  last admin leaves the installation.
- **The Current guild is a cookie, not a URL prefix.** The dashboard follows
  live logs with `EventSource`, which sends cookies but no headers, and every
  existing `/api/...` path stays as it is. An API token is bound to the Guild
  it was made in, as Coolify binds its tokens to a team. Paperclip's
  per-company URL prefix can come with its look without changing the API.
  A Desktop key names the Guild in the `Bakery-Guild` header instead: the
  Desktop app shows all of its Member's Guilds at once and keeps no cookie
  jar, and its own HTTP client can send headers.
- **The Local server is shared by every Guild.** Databases and Services run
  only on The Bakery's own machine, so a Guild without it could run neither.
  Only the Instance admin edits it. Coolify gives its localhost to the root
  team only.
- **Domains stay unique across the installation.** One Proxy serves every
  Guild, so a hostname can route to only one place whichever Guild owns it.
  Known hosts, by contrast, are per Guild, so one Guild's admin cannot change
  what another Guild trusts.
- **`team` stays on Coolify's `/api/v1` wire where its shape needs it.**
  Coolify's API names the Guild `team` (`/api/v1/teams`, `team_id`); scripts
  written for it keep working. A deliberate difference, made when that API
  arrives.
- **Any Member may create a Guild, and the first one is called "Default".**
  Coolify lets every user create teams; a new Guild owns nothing until its
  admin adds Servers, so creating one gives nobody more reach. "Default" is
  neutral and renamed on the Guild's General page.
- **The first Guild is made after Setup, not in its transaction.** Identity
  cannot hand its transaction to a context it does not know, so guilds hears
  `SetUp` once the Instance admin is stored and makes "Default" under a
  table lock, only when no Guild exists. An installation from before Guilds
  gets "Default" from a migration instead, with every Member's Role (the
  Owner's as admin).
- **identity's Guild-bound routes are registered by guilds.** API tokens are
  identity's, but only make sense in a Current guild, which identity cannot
  work out without importing guilds. So identity publishes
  them as route groups, guilds registers them behind its own middlewares
  and hands the Current guild and the Member's Permissions there (wire
  keys, `administrator` spelled out as every one) over with
  `identity.ActIn`. The Permissions that cap a new API token's Token
  permissions are the ones guilds found for that very request.
- **A Member in no Guild keeps their account.** Removing a Membership
  leaves the Member and their other Memberships; with none left, signing in
  still works but every Guild-bound request answers 403 `you are in no
  guild` and `GET /api/me` answers with no `guild`. Coolify deletes a user
  only when they leave their last team; The Bakery keeps the person, so
  another Guild can invite them back without a new account.
- **The Instance admin reads `owner` on the wire.** `GET /api/members` and
  `GET /api/me` keep that word for the Instance admin, beside
  `instance_admin: true`, so scripts written before Guilds keep working; the
  dashboard's Members page shows them as "Instance admin", with no Role to
  change.
- **No personal guild per account.** Coolify gives every user a personal
  team that cannot be deleted. A Bakery account may be in no Guild at all:
  it sees only "You are in no guild" with Create guild (Coolify's Select
  Team page), until it makes one or accepts an Invitation. For the same
  reason "Default" is not special, and a Member may delete their last Guild.
- **Known hosts go with their Guild; the rest must go first.** Projects,
  Servers, S3 storages and Notification channels are things a person made
  and may still want, so they block deleting the Guild, as Coolify's
  projects, servers and sources block deleting a team. Known hosts are only
  what the Guild's git sources were trusted with and mean nothing outside it,
  so they are deleted with it. The foreign keys enforce both: a resource
  made between the check and the delete still refuses it.
  Paperclip deletes a company with everything in it in one go; a Guild's
  Applications, Databases and backups run and live on Servers, so each goes
  by its own deliberate delete first.
- **Guilds are switched from two places: Paperclip's company menu and a
  guild rail like Discord's server list.** The Guild menu heads the sidebar:
  the Current guild's pattern icon and name, opening every Guild with the
  current one checked, New guild, Invite and Log out. Left of the sidebar,
  on every page, the guild rail shows one icon per Guild, the current one
  marked by a pill on its left edge, and "+" for a New guild; on a phone it
  sits in the slide-over menu. Paperclip v2026.1005.0 removed its company
  rail and switches companies from the menu alone; the rail is kept as a
  deliberate difference because the goal asks for Discord's server list.
  Both switch the same way and open the other Guild's Dashboard, and both
  offer New guild to everyone, since any Member may create a Guild.
  Paperclip lets a person drag the companies into their own order; nothing
  stores an order of Guilds, so both list them as the API does.
- **The Guild's workspace is Paperclip's shell, with what has no Bakery
  counterpart yet left out.** The sidebar, breadcrumb bar, account menu and
  settings sidebar follow Paperclip's Layout. The theme keeps a third choice,
  System, beside Dark and Light, because people already chose it. There is no
  command palette, search, properties panel or mobile bottom nav: in
  Paperclip they reach Issues and Agents, which come later, and they come
  with them. The Roles and Role pages have no Paperclip counterpart (a
  Company's members there hold one fixed role), so they are laid out as
  Paperclip's Company settings and access pages are.
- **Every Role reads the Guild's General and Members pages.** As in Coolify,
  whose team pages every member of the team sees; only admins see and make
  Invitations and change anything.
- **Listing, creating and switching Guilds need a Session.** An API token
  acts in the one Guild it was made in, so it has nothing to switch, and a
  leaked token cannot make Guilds. Coolify's `/api/v1/teams` comes with that
  API.
- **No Admin View and no MCP server setting, yet.** Coolify's team Admin
  View (every user of the installation, for the instance admin) comes with
  instance Settings, and its "MCP server" setting on the team's General page
  with agents reaching the API.
- **A Guild keeps an admin, checked under a row lock.** Changing or
  removing a Membership locks every Membership of the Guild first, so two
  admins demoting each other at once cannot leave it with none.
- **Invitations are guilds', and an existing Member accepts one while signed
  in.** An Invitation brings someone into one Guild, so it belongs where the
  Memberships are. Coolify's invitee already has an account; The Bakery's
  may not, so a new email still picks a name and password on the link's
  page. An email that already has a Member must not be taken over by
  whoever holds the link, so that Member's own Session is required, and the
  link's page sends them through sign-in and back.
- **Accepting is one transaction for the Membership and the Invitation, not
  for the new Member.** Identity stores a new Member in its own
  transaction, called while guilds holds the Invitation's row lock; the
  Membership and "accepted" are written together after it. If that last
  step fails, the person has an account but no Membership and the link
  still works: accepting again as the now existing Member (signed in)
  finishes it. A shared transaction across the two contexts would leak
  identity's persistence into guilds.
- **Roles and Permissions work like Discord's.** The goal fixes it: a Role
  is a named, colored set of Permissions at a Position, a Member holds any
  number of them plus the Base role, and their Permissions are the union.
  Discord's model is well known, lets a Guild make exactly the access it
  needs ("Deployer": deploy only) and keeps checks to one question, "does
  this request have Permission X here?". Code never asks for a Role by name.
- **Viewer, member and admin are seeded Roles.** Every Guild gets `Admin`,
  `Member` and `Viewer` with the Permissions that keep the meaning of the
  fixed roles they replace, and every Membership is moved onto the matching
  one, so nobody's access changes. They are ordinary Roles from then on:
  renamed, recolored, changed or deleted like any other. `manage_work`,
  added with Goals and Issues, was seeded into `Member` (and `Admin` holds
  it through `administrator`): planning work is what a Member did not have
  to be given, and a Viewer still only reads. `manage_skills`, added with
  Skills, was seeded into no Role but `Admin` (through `administrator`),
  as `hire_agents`: a Skill changes what every Agent that has it does.
- **The Base role starts with no Permissions.** Discord's `@everyone` grants
  a few by default; here every Member already holds one of the seeded Roles,
  and giving the Base role anything would widen today's access.
- **Deny beats allow across Roles in an override.** The goal's rule. Discord
  does it the other way round (a Role's allow beats another Role's deny); a
  deny here is meant to hold whichever other Role the Member also has, which
  is what "this Role must not deploy on this Project" says. The Member's own
  override still beats both, as in Discord.
- **The Base role's override comes first, as in Discord.** It is applied
  before the other Roles', so an allow on a Role beats a deny on `@everyone`.
  That is how a Project is made private: deny `view_resources` to
  `@everyone`, allow it to the Roles that may see it. Counting the Base role
  among the other Roles would let its deny beat every allow and make that
  impossible.
- **A Project one may not view answers 404**, the same as another Guild's
  thing, and is missing from every list, so a hidden Project leaks not even
  its existence. Reading still needs `view_resources` in the Guild first
  (`guilds.Auth`), so an allow on a Project can lift a Role's deny or give
  `see_secrets`, `deploy` and `manage_applications` there, but cannot open a
  Project to someone without `view_resources` at all. Discord lets a
  channel allow give view to a Role without it; here Servers, S3 storages
  and channels would then need their own answer, and no flow asks for it.
- **`project_id` has no foreign key.** Projects are the projects context's
  table; it tells guilds when one is deleted (`ForgetProject`), the way
  guilds asks the others before a Guild is deleted (`OnGuildDeleting`).
  Containers on a Server's page stay guild-wide: Servers are not in a
  Project.
- **Only four Permissions can be overridden per Project.** Those are the
  ones that mean something inside one Project; Servers, Notification
  channels, Members and Roles are the Guild's, not a Project's.
- **The Guild Master is separate from the Instance admin.** The Guild Master
  is the Guild's own (Discord's server owner); the Instance admin runs the
  installation. In "Default" the Instance admin becomes the Guild Master,
  because they made it; in a newer Guild whoever created it is. The Instance
  admin ranks just below the Guild Master in every Guild, so nothing the
  Owner could do before Guilds stops working, but a Guild Master is never at
  their mercy.
- **An Agent holds Roles through an Agent membership.** The goal gives an
  Agent Roles like a Discord bot. A Membership is the only thing that holds
  Roles, so an Agent gets a Membership of its own, marked as an Agent's and
  carrying its Hirer, and every check (`Can`, `InProject`, the hierarchy)
  applies to it unchanged. It is kept out of everything that means a person
  (the Members page, Invitations, Transfer offers, `IsMember`), so nothing
  that worked on Members changes. "Never placed above the person who hired
  it" is kept twice: a Role at or above the Hirer's highest is refused, and
  when the Hirer drops, the Agent's Roles at or above the Hirer's new
  highest are removed in the same change, as a Discord bot loses what its
  inviter could not give it. Paperclip's own per-agent permission grants
  are left out: one permission system, not two.
- **Agents are let in route by route.** Paperclip's agent JWT reaches most
  of its API and each route checks the actor type. Here every route is
  closed to Agent principals unless its context marks it open, so a route
  written for people never serves an Agent by accident; each slice of the
  goal opens what its Agents need.
- **The Guild Master changes only by an accepted offer.** A Guild must have
  exactly one at every moment, so it is never deleted or removed, only
  swapped, and only with the receiving person's consent, as Discord's
  ownership transfer asks the new owner. The offer expires after 7 days so a
  forgotten one cannot be accepted months later.
- **The Guild Master replaces "a Guild keeps an admin".** Phase 25 refused
  any change that left a Guild without a Member holding `administrator`.
  The Guild Master always holds every Permission and can never be removed
  or re-roled, so that check went: an admin may now demote the last other
  admin, because the Guild Master is still there.
- **Expiry is read, not swept.** An open offer past `expires_at` counts as
  expired wherever it is read, and is written `expired` then (or when a new
  offer is made for the Guild). No scheduler runs for it, and nothing can
  act on an offer between its expiry and that write.
- **"Cannot leave" is enforced where removal exists.** No "leave guild"
  and no account deletion exist yet, so the Guild Master's Membership is
  protected by refusing its removal (and its Roles' change) to everyone;
  both future actions must ask `guilds.IsGuildMaster` first.
- **The offers banner shows offers of every Guild.** The offered Member
  sees each open offer under the top bar of every page, naming its Guild,
  not only while that Guild is Current: otherwise someone who never
  switches to it would never learn of the offer before it expires.
- **`administrator` may delete an empty Guild.** Discord lets only the owner
  delete a server. Today's admins can delete a Guild that owns nothing, and
  the seeded `Admin` Role keeps that.
- **A Member can only give Permissions they have.** Discord's rule; without
  it a `manage_roles` holder could make a Role with `administrator` below
  their own and hand it out.
- **Discord's hierarchy, rule for rule.** A Member changes only Roles below
  their own highest and only Members whose highest Role is below theirs, so
  someone given `manage_roles` cannot climb past whoever gave it. A new
  Role lands just above `@everyone`, as in Discord, and is dragged up from
  there. A Role's Permissions can only be switched on or off by someone who
  holds them, `administrator` included: otherwise a `manage_roles` holder
  could make a Role with `administrator` below their own and assign it.
  `@everyone`'s Permissions may be edited by anyone with `manage_roles`
  (it is below everyone), but never its name, color or Position. Unlike
  Discord, nobody changes their own Roles: a Member leaves instead.
- **`PATCH /api/members/{id}` is gone, not kept.** It set one Role, and a
  Member now holds several, so any meaning it kept would silently drop the
  others. It answers 410 naming the routes that replace it. Invitations
  still accept `{"role": "member"}`, which has one obvious meaning: the
  seeded Role of that name.
