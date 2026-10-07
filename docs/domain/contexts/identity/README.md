# identity

- Subdomain: generic
- Hosted in: `services/api` (module `contexts/identity`)

## Purpose

Knows who may use this installation of The Bakery: the Members, the one
Instance admin created by Setup, how a request proves who sent it (a Session
or an API token), and each Member's own Profile, Two-factor authentication
included. It is **not** responsible for what a Member may do where: Guilds,
Memberships with their Roles and Invitations belong to
[guilds](../guilds/README.md). OAuth login comes later and will grow this
context.

## Language

| Term | Meaning |
| ---- | ------- |
| Member | A person who may sign in: name, email and password hash. What they may do is per Guild, from the Roles on their Membership (see guilds). |
| Instance admin | The Member Setup creates (formerly the Owner). Exactly one. guilds gives them every Permission in every Guild, just below its Guild Master. |
| Setup | Creating the Instance admin and the first Guild. Only possible while no Instance admin exists. |
| Session | A signed JWT in the HttpOnly cookie `bakery_session`, naming a Member. |
| API token | A named `bky_…` secret of one Member, made in one Guild and working only there, sent as `Authorization: Bearer`, with its Token permissions and an optional expiry. |
| Token permission | What a request made with an API token may do, capped by the Member's Permissions in the token's Guild: `root` (everything those Permissions allow), `write` (changes other than deploy actions), `deploy` (deploy, restart, stop, start, cancel, rollback), `read` (reading without Secrets), `read:sensitive` (reading Secrets too). In code the type is still `identity.Permission`; it is this Token permission, never guilds' Permission of a Role. The dashboard and the wire keep the word `permissions`. |
| Principal | Who a request is from: a Member, whether they are the Instance admin, and, when an API token sent it, the token's Guild and Token permissions. guilds adds the Permissions the request acts with (always the Member's current ones in the Current guild). |
| Secret | A value a viewer may not read (see the glossary). |
| Profile | A Member's own name, password, Sessions and Two-factor authentication, changed only by that Member. |
| Two-factor authentication | A TOTP secret on a Member: `off`, `pending` (made, not yet confirmed with a code) or `on`. When on, signing in needs an Authenticator code or a Recovery code after the password. |
| Authenticator code | 6 digits from the Member's app (RFC 6238, SHA-1, 30 s steps), accepted for the current step ± 1 and only once. |
| Recovery code | One of 10 single-use codes handed out when Two-factor authentication is switched on or the codes are renewed. |
| Login challenge | The 5 minutes between a correct password and the second step, carried in the `bakery_login` cookie; at most 5 wrong codes. |
| Sessions valid from | The moment before which a Member's Sessions no longer count; set by a password change, "sign out everywhere else" and a two-factor reset. |

## Model

### Aggregates

| Aggregate | Invariants |
| --------- | ---------- |
| Member | Email is a valid address and unique; name is not empty; password has at least 12 characters and is only stored hashed. Exactly one Member is the Instance admin, and the Instance admin is never removed. Two-factor authentication only counts for sign-in when `on`; its secret is stored only encrypted and Recovery codes only hashed; an Authenticator code is accepted only for a time step later than the last one accepted. A new password has at least 12 characters and needs the current one. |
| API token | Name (the "Description") is 3–255 characters when made and unique per Member; belongs to one Member and one Guild and is removed with either; only the SHA-256 of its value is stored, and the value is shown once. Its Token permissions are known ones, never empty (none means `read`), `root` stands alone, `read:sensitive` brings `read`. The Member's Permissions in that Guild cap what it may be given: `root` only with `administrator`, `write` only with `manage_applications`, `deploy` only with `deploy`, `read:sensitive` only with `see_secrets`, `read` always (refused with 422 `your permissions cannot grant <permission>`). An expiry is in the future when set; from then on the token no longer authenticates. |

### Commands

Who may run each is in brackets.

- `Setup(name, email, password)` [anyone, once]: creates the Instance admin and, through guilds, the first Guild; refused with a conflict once an Instance admin exists.
- `Login(email, password)` [anyone]: returns a Session; a wrong email or password gives the same error.
- `Logout()` [any Member]: clears the Session cookie.
- `CurrentMember(principal)` [any Member]: the signed-in Member; guilds adds their Permissions in the Current guild.
- `CreateMember(name, email, password)` [guilds, when an Invitation to a new email is accepted].
- `CreateAPIToken(name, permissions, expires in days)` [any Member, with a Session]: made in the Current guild; returns the value once. The expiry is 7, 30, 60, 90 or 365 days, or none (Never). `GET /api/api-tokens/permissions` answers which Token permissions the Member may grant.
- `RevokeAPIToken(id)` [the token's Member].
- `ChangeName(name)`, `ChangePassword(current, new)`, `SignOutOtherSessions()` [the Member themselves, with a Session]: a new password and signing out elsewhere end every other Session of the Member; the current one gets a fresh Session.
- `StartTwoFactor()` [the Member, with a Session]: a new secret and its `otpauth://` URI; two-factor becomes pending. Refused while on.
- `ConfirmTwoFactor(code)` [the Member, with a Session]: switches it on and returns 10 Recovery codes, once.
- `RegenerateRecoveryCodes(code)` [the Member, with a Session]: 10 new Recovery codes; the old ones stop working.
- `DisableTwoFactor(password, code or Recovery code)` [the Member, with a Session].
- `LoginTwoFactor(challenge, code or Recovery code)` [anyone holding a Login challenge]: returns a Session; the fifth wrong code ends the challenge.
- `ResetTwoFactor(member)` [an admin of a Guild the Member is in]: switches it off for someone locked out and ends their Sessions; never the Instance admin's, never your own. From the server, `artisan identity:reset-two-factor <email>` does it for anyone, the Instance admin included.

### Domain events

- `SetUp { instance admin }`: Setup just stored the Instance admin. Its one
  subscriber (guilds, with `OnSetUp(f)`) is called synchronously and makes
  the first Guild; its error fails the request, though the Instance admin
  stays.

## Integration

- **Publishes:**
  - `identity.Authenticate(ctx)`: the Principal of a request, by API token
    or Session cookie, or 401 (an expired token is 401 like an unknown one).
    `Principal.Allows(permission)` holds the Token permission rules: a Session
    always, a token when it carries the Token permission or `root`. guilds builds
    its `Auth`, `Deploy`, `Admin` and `Secrets` middlewares on it, asking
    `read` for GET and HEAD and `write` for anything else (`deploy` for
    Coolify's deploy actions: deploying, restarting and stopping
    Applications and Previews, cancelling and rolling back Deployments,
    starting, stopping and restarting Databases and Services, redeploying
    Services), and answering 403 `Missing required permissions:
    <permission>`; every other context's routes sit behind those.
  - `identity.ActIn(ctx, guild, permissions)`: guilds tells identity's
    routes that work inside a Guild (API tokens) the Current guild and the
    wire keys of the Member's Permissions there.
  - `identity.APITokenRoutes(r)`: those routes, which guilds registers
    inside its own middlewares.
  - `identity.Members(ctx, ids)`, `identity.MemberByID(ctx, id)` and
    `identity.MemberByEmail(ctx, email)`: name, email, two-factor on or off
    and the Instance admin flag, for guilds' Members page, `GET /api/me` and
    Invitations.
  - `identity.ResetTwoFactor(ctx, member)` and
    `identity.RevokeAPITokens(ctx, member, guild)`: for guilds, which
    decides who may.
  - `identity.CreateMember(...)` and `identity.SignIn(ctx, member)` (the
    Session cookie on the response): for guilds, when an Invitation to a
    new email is accepted.
  - `identity.SessionMember(ctx)`: who the Session cookie is for, without
    answering 401 itself, for guilds when an existing Member accepts an
    Invitation.
  - `identity.OnSetUp(f)`: see Domain events.
  Other contexts learn nothing else about Members.
- **Consumes:** nothing. Identity never imports guilds; where the first
  Member needs a Guild, guilds subscribes to `SetUp`, and where identity's
  routes need the Current guild, guilds hands it over with `ActIn`.
- **Account deletion, when it comes,** must refuse a Member who is a Guild
  Master of any Guild (they transfer it or delete the Guild first). Since
  identity never imports guilds, it will publish a check hook that guilds
  subscribes to with `guilds.IsGuildMaster`, as it subscribes to `SetUp`.
  The database refuses it too: `guilds.master_id` references the Member
  without a cascade.

## Why it's shaped this way

- **The Session lives in a cookie, not an Authorization header.** The
  dashboard follows live logs with `EventSource`, which cannot send headers.
  The cookie is HttpOnly and SameSite=Strict, and the dashboard reaches the
  API on the same origin (the Vite proxy in dev), so no CORS is needed.
  Scripts use an API token in the `Authorization` header instead, which
  works on every route, streams included.
- **Setup checks and inserts under a table lock.** Two browsers racing through
  first-run setup must not both create an Instance admin. `LOCK TABLE users IN
  EXCLUSIVE MODE` in the Setup transaction serialises them; the second gets a
  conflict. The table is still called `users`, and a unique partial index
  backs "exactly one Instance admin".
- **Teams are Guilds, in their own context.** Who someone is stays here;
  where they act and with which Permissions is
  [guilds](../guilds/README.md), which explains the split.
- **Authenticate reads the Member from the database on every request.** The
  JWT and the API token only name the Member; their Permissions come from
  the Roles on their Membership in guilds, so demoting or removing someone takes effect at
  once instead of when the JWT expires.
- **API tokens are random (`bky_` + 32 bytes base62) and stored as
  SHA-256.** A slow hash is for low-entropy passwords; a 256-bit random
  token only needs a fast one, which keeps the per-request lookup cheap. The
  prefix makes a leaked token recognisable to secret scanners. A request
  made with an API token cannot create or revoke tokens, so a leaked token
  cannot mint more.
- **Token permissions are Coolify's abilities, capped by the Member's
  current Permissions.** Coolify refuses to create a token that exceeds the
  creator's role and then trusts the token; The Bakery also refuses at
  creation, and on every request the token acts with its Member's *current*
  Permissions besides its Token permissions, so taking a Role away narrows
  their tokens at once. That is how the read-only tokens made before Token permissions keep doing
  exactly what they did: they became `read`, every other token `root`, and
  `root` on a member's or viewer's token is still only what their
  Permissions allow.
  A consequence: a `read` token of an admin reads admin-only lists (Members,
  Known hosts, Notification channels, all without Secrets), where the old
  read-only token acted as a viewer. Coolify's read token does the same.
- **A member may grant `write`, `deploy` and `read:sensitive`.** Coolify
  limits its members to read tokens. The Bakery's member Role is the one
  that changes things, and scripts (CI deploys) are what tokens are for,
  so a member may give a token what their Role does, except `root`. An old
  request without `permissions` gets `root` for an admin and everything but
  `root` for a member, so an old script's token does what it did.
- **`deploy` is separate from `write`, as in Coolify.** A CI token that
  only deploys cannot change or delete anything, and cannot even read; a
  `write` token cannot deploy. Coolify's `write:sensitive`, used on one
  route of its API and never offered in its UI, is left out until that API
  arrives.
- **Token values stay `bky_…`, not Sanctum's `id|secret`.** Coolify's CLI
  and scripts send the value only as a Bearer, so its format does not
  matter to them, and the prefix keeps leaked tokens recognisable. The
  Description stays unique per Member, which Coolify does not require, so a
  Member can tell their tokens apart.
- **Keys & Tokens has only API Tokens.** Coolify's Keys & Tokens layout
  also lists Private Keys, Cloud Tokens and Cloud-Init Scripts. The Bakery
  keeps SSH keys per Server and per Application, so there is no shared
  Private Key resource to list yet; it comes with the goal's "what Coolify
  has and The Bakery does not" step. Cloud Tokens and Cloud-Init Scripts
  provision servers at a cloud provider, which is not part of The Bakery.
  The menu shows only what exists rather than dead links.
- **No "API disabled" state.** Coolify can switch its API off in instance
  Settings and then shows "API access is turned off" instead of the page.
  The Bakery's dashboard is itself an API client, so the API is always on;
  an instance switch for the token API can come with instance Settings.
- **The API Tokens page differs from Coolify's in small ways.** The
  Description is cleared after a create, because a second token with the
  same Description is refused (above). The page size is remembered under
  `bakery.page-size.api-tokens`. A viewer sees the same form with only
  Read to choose, since Coolify has no viewer. The page lists only the
  signed-in Member's own tokens, so each row has Revoke, as Coolify's rows
  do for their owner.
- **Own TOTP code, no library.** RFC 6238 is a few dozen lines on
  `crypto/hmac` and `encoding/base32`, fits the domain's no-I/O rule (the
  time and the randomness are passed in) and is checked against the RFC's
  test vectors. TOTP only: it works with every authenticator app and needs
  no outside service; passkeys can come next to it later.
- **An Authenticator code is good once.** The last accepted time step is
  stored per Member and only a later step is accepted, with a conditional
  update so two racing sign-ins cannot both use one code. A code seen over
  someone's shoulder cannot be replayed inside its 90 s window.
- **The Login challenge is a cookie, not a table.** After a correct
  password the API sets `bakery_login` (HttpOnly, SameSite=Strict, 5
  minutes): the Member, the expiry and the wrong attempts so far, encrypted
  with the app key. Nothing to clean up in the database. Replaying an older
  challenge cookie only gives back attempts within its 5 minutes, and
  one-time codes still hold.
- **Other Sessions end by a timestamp, not a session table.**
  `sessions_valid_from` on the Member; Auth refuses a Session JWT issued
  before it. JWTs carry their issue time in whole seconds, so the stamp is
  kept in whole seconds and a Session issued in that same second still
  counts: that is what lets the fresh cookie handed out by the same request
  survive. The trade-off is a window of under a second.
- **Recovery codes are random (10 base32 characters, 50 bits) and stored
  as SHA-256**, like API tokens, and each is deleted when used.
- **API tokens are not asked for a code.** A token is created from a
  Session that already passed two-factor; asking scripts for codes would
  make tokens useless. The two-factor and Profile routes take a Session
  only, so a leaked token can neither switch two-factor off nor change the
  password.
- **The Instance admin's lost phone is an artisan command.** Nobody
  outranks the Instance admin in the dashboard, and whoever has a shell on
  the server already controls The Bakery.
- **The Instance admin is not transferable yet**, and can be neither
  demoted nor removed, so an installation can never be left without someone
  who can manage it. It is the Member that was the Owner before Guilds.
- **The `viewer` Role is kept although Coolify has none.** Coolify's Roles
  are owner, admin and member; The Bakery adds viewer for read-only access
  (dashboards on a wall, a read-only API token) without handing out
  Secrets. The Coolify API (`/api/v1`) reports a viewer as a member with
  read-only rights. Members keep their name, as on Coolify's Team page;
  Coolify's Teams are Guilds.
- **Profile, not Account.** The page where a Member changes their own
  name, password, Sessions and Two-factor authentication is Coolify's
  Profile page, so The Bakery calls it Profile too.
