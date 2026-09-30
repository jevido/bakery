# identity

- Subdomain: generic
- Hosted in: `services/api` (module `contexts/identity`)

## Purpose

Knows who may use this Bakery and what each of them may do: the Members,
each with a Role, the one Owner created by Setup, the Invitations that bring
in the others, and how a request proves who sent it (a Session or an API
token). It is **not** responsible for several Teams, two-factor
authentication or OAuth login; those come in later phases and will grow
this context.

## Language

| Term | Meaning |
| ---- | ------- |
| Member | A person who may sign in: name, email, password hash and Role. |
| Role | `viewer`, `member`, `admin` or `owner` (see the glossary for what each may do). |
| Owner | The Member with the `owner` Role. Exactly one, created by Setup. |
| Setup | Creating the Owner. Only possible while no Owner exists. |
| Invitation | An email and a Role, with a link that is good once and for 7 days. |
| Session | A signed JWT in the HttpOnly cookie `bakery_session`, naming a Member. |
| API token | A named `bky_…` secret of one Member, sent as `Authorization: Bearer`. |
| Principal | Who a request is from: a Member and the Role the request acts with (the Member's, or viewer for a read-only API token). |
| Secret | A value a viewer may not read (see the glossary). |

## Model

### Aggregates

| Aggregate | Invariants |
| --------- | ---------- |
| Member | Email is a valid address and unique; name is not empty; password has at least 12 characters and is only stored hashed. Exactly one Member is the Owner; the Owner's Role never changes and the Owner is never removed. Nobody changes their own Role or removes themselves. |
| Invitation | Email is valid and not an existing Member's; Role is admin, member or viewer, never owner; at most one open Invitation per email; expires 7 days after it was made; accepted at most once; a revoked or expired one cannot be accepted. Only the hash of its token is stored. |
| API token | Name is 1–64 characters and unique per Member; belongs to one Member and is removed with them; only the SHA-256 of its value is stored, and the value is shown once. |

### Commands

Who may run each is in brackets.

- `SetupOwner(name, email, password)` [anyone, once]: refused with a conflict once an Owner exists.
- `Login(email, password)` [anyone]: returns a Session; a wrong email or password gives the same error.
- `Logout()` [any Member]: clears the Session cookie.
- `CurrentMember(principal)` [any Member]: the signed-in Member and Role.
- `Invite(email, role)` [admin, owner]: returns the Invitation and its link, once.
- `RevokeInvitation(id)` [admin, owner].
- `AcceptInvitation(token, name, password)` [anyone with the link]: creates the Member with the invited email and Role and signs them in.
- `ChangeRole(member, role)` [admin, owner]: never to or from owner, never your own.
- `RemoveMember(member)` [admin, owner]: never the Owner, never yourself; their Sessions and API tokens stop working at once.
- `CreateAPIToken(name, readOnly)` [any Member, with a Session]: returns the value once.
- `RevokeAPIToken(id)` [the token's Member].

### Domain events

- `InvitationCreated { email, role, invited by, link, expires }`: an
  Invitation was made. Its one subscriber (notifications, with
  `OnInvitationCreated(f)`) is called synchronously and answers whether it
  emailed the link, which the invite response reports as `emailed`.

## Integration

- **Publishes:**
  - `identity.Auth`: the request comes from a Member, by Session cookie or
    API token; a viewer is refused (403) on anything but GET and HEAD.
    Every other context's routes sit behind it, so a new route needs at
    least `member` to change anything.
  - `identity.Admin`: only admin and owner; wraps Servers, S3 storages,
    Known hosts, Members and Invitations.
  - `identity.Secrets`: member or higher, for GETs that return Secrets (and
    the list of S3 storages, which members pick for a Backup schedule).
  - `identity.CanSeeSecrets(ctx)`: for a response that mixes Secrets with
    fields a viewer may see; the controller leaves the Secrets out.
  - `OnInvitationCreated(f)`: see Domain events.
  Other contexts learn nothing else about Members.
- **Consumes:** nothing.

## Why it's shaped this way

- **The Session lives in a cookie, not an Authorization header.** The
  dashboard follows live logs with `EventSource`, which cannot send headers.
  The cookie is HttpOnly and SameSite=Strict, and the dashboard reaches the
  API on the same origin (the Vite proxy in dev), so no CORS is needed.
  Scripts use an API token in the `Authorization` header instead, which
  works on every route, streams included.
- **Setup checks and inserts under a table lock.** Two browsers racing through
  first-run setup must not both create an Owner. `LOCK TABLE users IN
  EXCLUSIVE MODE` in the Setup transaction serialises them; the second gets a
  conflict. The table is still called `users`, and a unique partial index on
  the owner Role backs "exactly one Owner".
- **One Team per installation, not several.** Every resource already
  belongs to the installation; a Team > Project hierarchy would touch every
  context's tables and queries for a feature few self-hosters use. The
  Members of this installation are its one team; several Teams can be added
  above it later.
- **Invitations are links, not emails.** Bakery sends no email yet. The
  admin copies the link and sends it however they like.
- **Roles are enforced by coarse middlewares, not per route checks.** Auth
  refuses changes by viewers, `Admin` wraps whole admin areas, `Secrets`
  wraps the GETs that return Secrets. It covers every context with a few
  lines each, and a new route is safe by default.
- **Auth reads the Member from the database on every request.** The JWT
  and the API token only name the Member; the Role comes from the row, so
  demoting or removing someone takes effect at once instead of when the
  JWT expires.
- **API tokens are random (`bky_` + 32 bytes base62) and stored as
  SHA-256.** A slow hash is for low-entropy passwords; a 256-bit random
  token only needs a fast one, which keeps the per-request lookup cheap. The
  prefix makes a leaked token recognisable to secret scanners. A request
  made with an API token cannot create or revoke tokens, so a leaked token
  cannot mint more.
- **The Owner is not transferable yet**, and can be neither demoted nor
  removed, so an installation can never be left without someone who can
  manage it.
