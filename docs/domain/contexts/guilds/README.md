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
| Guild | A group of Members that owns Projects, Servers (all but the Local server), S3 storages, Notification channels, Known hosts and API tokens. Name and optional description. Coolify's Team, Paperclip's Company. |
| Membership | A Member's place in a Guild, holding any number of Roles and always the Base role. At most one per Member and Guild. |
| Role | A named set of Permissions in one Guild, with a color and a Position. Seeded in every Guild: `Admin` (Administrator), `Member` (View resources, See secrets, Deploy, Manage applications), `Viewer` (View resources). Discord's role. |
| Base role | The Role every Member holds, shown as `@everyone`, at Position 0. Cannot be assigned, removed, renamed or deleted; only its Permissions change. Seeded with none. Discord's `@everyone`. |
| Position | A Role's place in its Guild's order, higher above lower. A Member's highest Role is the highest-placed Role they hold. |
| Permission | One of the fixed list in the glossary (`administrator`, `view_resources`, `see_secrets`, `deploy`, `manage_applications`, `manage_servers`, `manage_notifications`, `manage_guild`, `manage_members`, `manage_roles`, `hire_agents`, `approve`, `manage_budgets`). A Member's Permissions in a Guild are the union of their Roles'; `administrator` grants every one. |
| Guild Master | The one Member of a Guild above every Role, with every Permission, never removable or re-roled; changes only by an accepted Transfer offer. Discord's server owner. |
| Transfer offer | The Guild Master's offer of the Guild Master to one other Member; accepted, declined, withdrawn or expired after 7 days. |
| Permission override | On one Project, per Role or per Member: allow, deny or inherit for `view_resources`, `see_secrets`, `deploy` or `manage_applications`. |
| Instance admin | The Member Setup creates. Acts with every Permission in every Guild, just below its Guild Master, and sees every Guild, with or without a Membership. |
| Current guild | The Guild a request acts in: the `bakery_guild` cookie for a Session, the Guild an API token was made in for a token. |
| Invitation | An email and the Roles it brings, for one Guild, with a link that is good once and for 7 days. |
| Guild switcher | Where a Member picks the Current guild among the Guilds they are in (all of them for the Instance admin). |

## Model

### Aggregates

| Aggregate | Invariants |
| --------- | ---------- |
| Guild | Name is 1–255 characters after trimming; the description is optional, at most 255 characters. Deleted only when it owns nothing: no Projects, Servers, S3 storages or Notification channels (each owning context answers through `OnGuildDeleting`); its Memberships, Invitations, API tokens and Known hosts go with it. |
| Guild (Guild Master) | Exactly one Guild Master at all times, a Member with a Membership in the Guild. The Guild Master is never removed from the Guild, never re-roled by anyone, cannot leave it, and cannot have their account deleted while they hold it; they transfer it first or delete the Guild. At most one open Transfer offer, to another Member of the Guild who is a person (never an agent); it changes nothing until accepted, can be withdrawn by the Guild Master and declined by its Member, and expires 7 days after it was made. Accepting swaps the Guild Master in one step; both keep their other Roles. |
| Role | Belongs to one Guild. Name is 1–100 characters after trimming; color is `#rrggbb`; Permissions are from the fixed list. Positions are unique per Guild. The Base role is at Position 0, always exists, and cannot be renamed, deleted, assigned or removed; every other Role sits above it. A Role held by Members can be deleted; they simply stop holding it. |
| Membership | One per Member and Guild. Holds a set of Roles of its own Guild (never the Base role explicitly; it holds that implicitly). The Instance admin's Memberships are never removed. |
| Permission overrides | Keyed by one Project of the Guild. Each entry is a Role or a Member, a Permission from `view_resources`, `see_secrets`, `deploy` and `manage_applications`, and allow or deny (inherit is no entry). For a Member on that Project: start from their Guild Permissions; then, over all the Roles they hold, a deny removes the Permission and otherwise an allow adds it; then the Member's own entry, if any, decides. `administrator`, the Instance admin and the Guild Master skip overrides. |
| Invitation | Belongs to one Guild. Email is valid and not already a Member of this Guild; its Roles are Roles of that Guild below the inviter's highest Role (none means the Base role only); at most one open Invitation per Guild and email; expires 7 days after it was made; accepted at most once; a revoked or expired one cannot be accepted. Only the hash of its token is stored. |

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
- An agent holds Roles like any Member and is never placed above the person
  who hired it.

### Commands

Who may run each is in brackets, by Permission in the Guild concerned. The
Guild Master and the Instance admin hold every Permission; every command
that touches a Role or a Member also follows the hierarchy above.

- `CreateGuild(name, description)` [any Member, with a Session]: the creator
  gets a Membership with the `Admin` Role and is the Guild Master; the Guild
  gets the seeded Roles and becomes their Current guild; it owns nothing and
  deploys to the Local server.
- `RenameGuild(name)`, `ChangeDescription(description)` [`manage_guild`].
- `DeleteGuild()` [`administrator`]: refused while the Guild owns anything, naming
  what (`projects`, `servers`, `s3 storages`, `notification channels`). The
  first Guild is not special, and a Member may delete their last Guild.
- `SwitchGuild(guild)` [any Member of that Guild, the Instance admin for any;
  with a Session]: sets the Current guild of the Session.
- `Invite(email, roles)` [`manage_members`; the Roles below one's own]:
  returns the Invitation and its link, once.
- `RevokeInvitation(id)` [`manage_members`].
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
- `SetOverride(project, role or member, permission, allow | deny | inherit)`
  [`manage_roles`]: for a Role or Member below one's own highest.

### Domain events

- `SetUp` (identity's, consumed): Setup made the Instance admin, and guilds
  makes the first Guild, "Default", with the seeded Roles and their
  Membership holding `Admin`, and makes them its Guild Master, unless a
  Guild exists already.

- `InvitationCreated { guild, email, role, invited by, link, expires }`: an
  Invitation was made. Its one subscriber (notifications, with
  `OnInvitationCreated(f)`) is called synchronously and answers whether it
  emailed the link, which the invite response reports as `emailed`.
- `GuildDeleting { guild }`: asked synchronously before a Guild is deleted;
  every context that owns rows for Guilds registers `OnGuildDeleting(f)` and
  refuses while the Guild still owns something there.

## Integration

The middlewares below still check today's `viewer`/`member`/`admin`; task 03
of the Roles phase replaces them with one check by Permission
(`guilds.Can(permission)`), and this list changes with it.

- **Publishes:**
  - `guilds.Auth`: the request comes from a Member (by `identity.Authenticate`)
    with a Membership in the Current guild, or from the Instance admin; a
    viewer there is refused (403) on anything but GET and HEAD. An API
    token's Permissions apply as identity describes them.
  - `guilds.Deploy`: `guilds.Auth` for Coolify's deploy actions, where an
    API token needs `deploy` instead of `write`.
  - `guilds.Admin`: only an admin of the Current guild.
  - `guilds.Secrets`: member or higher in the Current guild, and
    `read:sensitive` for an API token, for GETs that return Secrets.
  - `guilds.CanSeeSecrets(ctx)`: for a response that mixes Secrets with
    fields a viewer may see.
  - `guilds.Owns(name, belongs)`: after `guilds.Auth`, 404 for a route
    whose `{id}` names something outside the Current guild; `belongs(id,
    guild)` is the owning context's check (e.g. `projects.ApplicationInGuild`).
  - `guilds.Current(ctx) uint64`: the Current guild's id, which every other
    context stores on what it creates (or reaches through something that
    does) and filters every list and read by.
  - `guilds.RoleOf(ctx)`: the Role the request acts with there.
  - `guilds.OnGuildDeleting(kind, f)` and `guilds.OnInvitationCreated(f)`:
    see Domain events. Projects, servers, databases (S3 storages) and
    notifications register `OnGuildDeleting`.
- **Serves:** `GET /api/me` (the Member, their Role in the Current guild,
  `instance_admin`, the Current `guild` and every Guild they may switch to
  with their Role there). The Guilds, with a Session only and also for a
  Member in no Guild: `GET /api/guilds` (every Guild they may switch to,
  with their Role), `POST /api/guilds` (`{"name", "description"}`; 201, and
  it becomes the Current guild) and `POST /api/guilds/{id}/switch` (204; 404
  for a Guild they may not act in). The Current guild:
  `GET /api/guilds/current` (`name`, `description` and `blocking`, what
  keeps it from being deleted) for every Role, `PATCH /api/guilds/current`
  (`{"name", "description"}`) and `DELETE /api/guilds/current` (204, or 409
  with `blocking`) for admins. The Members of the Current guild:
  `GET /api/members` for every Role, and for admins
  `PATCH /api/members/{id}` (`{"role"}`), `DELETE /api/members/{id}` and
  `DELETE /api/members/{id}/two-factor`. On the wire the Instance admin's `role` reads `owner`, with
  `instance_admin: true`. The Invitations of the Current guild, for admins:
  `GET /api/invitations`, `POST /api/invitations` (`{"email", "role"}`) and
  `DELETE /api/invitations/{id}`; another Guild's id answers 404. Open to
  anyone with the link: `GET /api/invitations/by-token/{token}` (the
  Invitation, its `guild` and `existing_member`) and
  `POST /api/invitations/by-token/{token}/accept`.
- **Consumes:** identity's `identity.Authenticate(ctx)` (a Principal: the
  Member, whether they are the Instance admin, and for an API token its
  Guild and Permissions), `identity.ActIn`, identity's API token routes
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
  and hands the Current guild and Role over with `identity.ActIn`. The
  Role that caps a new API token's Permissions is the one guilds found for
  that very request.
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
- **The guild switcher is Coolify's until the look changes.** It leads the
  top bar's breadcrumb, where Coolify's team switcher sits. Paperclip's
  guild rail replaces it when the dashboard takes Paperclip's shell.
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
  renamed, recolored, changed or deleted like any other.
- **The Base role starts with no Permissions.** Discord's `@everyone` grants
  a few by default; here every Member already holds one of the seeded Roles,
  and giving the Base role anything would widen today's access.
- **Deny beats allow across Roles in an override.** The goal's rule. Discord
  does it the other way round (a Role's allow beats another Role's deny); a
  deny here is meant to hold whichever other Role the Member also has, which
  is what "this Role must not deploy on this Project" says. The Member's own
  override still beats both, as in Discord.
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
- **The Guild Master changes only by an accepted offer.** A Guild must have
  exactly one at every moment, so it is never deleted or removed, only
  swapped, and only with the receiving person's consent, as Discord's
  ownership transfer asks the new owner. The offer expires after 7 days so a
  forgotten one cannot be accepted months later.
- **`administrator` may delete an empty Guild.** Discord lets only the owner
  delete a server. Today's admins can delete a Guild that owns nothing, and
  the seeded `Admin` Role keeps that.
- **A Member can only give Permissions they have.** Discord's rule; without
  it a `manage_roles` holder could make a Role with `administrator` below
  their own and hand it out.
