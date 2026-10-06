# guilds

- Subdomain: supporting
- Hosted in: `services/api` (module `contexts/guilds`)

## Purpose

Knows *where* a request acts and *with which Role*: the Guilds of this
installation, each Member's Membership (and its Role) in them, the
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
| Membership | A Member's place in a Guild, with a Role. At most one per Member and Guild. |
| Role | `viewer`, `member` or `admin`, per Membership (see the glossary for what each may do). |
| Instance admin | The Member Setup creates. Acts as `admin` in every Guild and sees every Guild, with or without a Membership. |
| Current guild | The Guild a request acts in: the `bakery_guild` cookie for a Session, the Guild an API token was made in for a token. |
| Invitation | An email and a Role for one Guild, with a link that is good once and for 7 days. |
| Guild switcher | Where a Member picks the Current guild among the Guilds they are in (all of them for the Instance admin). |

## Model

### Aggregates

| Aggregate | Invariants |
| --------- | ---------- |
| Guild | Name is 1–255 characters after trimming; the description is optional, at most 255 characters. Deleted only when it owns nothing: no Projects, Servers, S3 storages, Notification channels or Known hosts (each owning context answers through `OnGuildDeleting`); its Memberships, open Invitations and API tokens go with it. |
| Membership | One per Member and Guild. Role is `viewer`, `member` or `admin`. A Guild keeps at least one `admin` Membership. Nobody changes their own Role or removes their own Membership. The Instance admin's Memberships are never demoted or removed. |
| Invitation | Belongs to one Guild. Email is valid and not already a Member of this Guild; Role is admin, member or viewer; at most one open Invitation per Guild and email; expires 7 days after it was made; accepted at most once; a revoked or expired one cannot be accepted. Only the hash of its token is stored. |

### Commands

Who may run each is in brackets; *admin* means an admin of the Guild
concerned, which the Instance admin always is.

- `CreateGuild(name, description)` [any Member]: the creator gets an `admin`
  Membership; the new Guild owns nothing and deploys to the Local server.
- `RenameGuild(name)`, `ChangeDescription(description)` [admin].
- `DeleteGuild()` [admin]: refused while the Guild owns anything.
- `SwitchGuild(guild)` [any Member of that Guild, the Instance admin for any]:
  sets the Current guild of the Session.
- `Invite(email, role)` [admin]: returns the Invitation and its link, once.
- `RevokeInvitation(id)` [admin].
- `AcceptInvitation(token, name, password)` [anyone with the link]: when the
  email is new, creates the Member (through identity) with a Membership of
  the invited Role and signs them in; when a Member already has the email,
  that Member, signed in, gets the Membership.
- `ChangeRole(membership, role)` [admin]: never your own.
- `RemoveMembership(membership)` [admin]: never your own, never the last
  admin's; the Member keeps their other Memberships, and their API tokens made
  in this Guild stop working at once.

### Domain events

- `MemberSetUp` (identity's, consumed): Setup made the Instance admin;
  guilds makes the first Guild, "Default", with their `admin` Membership,
  unless a Guild exists already.

- `InvitationCreated { guild, email, role, invited by, link, expires }`: an
  Invitation was made. Its one subscriber (notifications, with
  `OnInvitationCreated(f)`) is called synchronously and answers whether it
  emailed the link, which the invite response reports as `emailed`.
- `GuildDeleting { guild }`: asked synchronously before a Guild is deleted;
  every context that owns rows for Guilds registers `OnGuildDeleting(f)` and
  refuses while the Guild still owns something there.

## Integration

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
  - `guilds.Current(ctx) uint64`: the Current guild's id, which every other
    context stores on what it creates and filters every list and read by.
  - `guilds.OnGuildDeleting(f)` and `guilds.OnInvitationCreated(f)`: see
    Domain events.
- **Consumes:** identity's `identity.Authenticate(ctx)` (a Principal: the
  Member, whether they are the Instance admin, and for an API token its
  Guild and Permissions), `identity.OnMemberSetUp(f)` and
  `identity.CreateMember(...)` when an Invitation
  to a new email is accepted. Identity never imports guilds.

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
  `MemberSetUp` once the Instance admin is stored and makes "Default" under a
  table lock, only when no Guild exists. An installation from before Guilds
  gets "Default" from a migration instead, with every Member's Role (the
  Owner's as admin).
