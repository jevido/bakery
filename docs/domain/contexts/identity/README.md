# identity

- Subdomain: generic
- Hosted in: `services/api` (module `contexts/identity`)

## Purpose

Knows who may use this Bakery: the one Owner, created by Setup, who signs in
and gets a Session. It is **not** responsible for teams, roles, invitations,
API tokens or two-factor authentication; those come in later phases and will
grow this context.

## Language

| Term | Meaning |
| ---- | ------- |
| Owner | The single person who administers the installation. Has a name, an email and a password hash. |
| Setup | Creating the Owner. Only possible while no Owner exists. |
| Session | A signed JWT in the HttpOnly cookie `bakery_session`. |

## Model

### Aggregates

| Aggregate | Invariants |
| --------- | ---------- |
| Owner | Exactly zero or one exists. Email is a valid address; name is not empty; password has at least 12 characters and is only stored hashed. |

### Commands

- `SetupOwner(name, email, password)`: refused with a conflict once an Owner exists.
- `Login(email, password)`: returns a Session; a wrong email or password gives the same error.
- `Logout()`: clears the Session cookie.
- `CurrentOwner(session)`: the signed-in Owner, or unauthenticated.

### Domain events

None yet.

## Integration

- **Publishes:** the `auth` HTTP middleware. Every other context's routes sit
  behind it and learn only that an Owner is signed in.
- **Consumes:** nothing.

## Why it's shaped this way

- **The Session lives in a cookie, not an Authorization header.** The
  dashboard follows live logs with `EventSource`, which cannot send headers.
  The cookie is HttpOnly and SameSite=Strict, and the dashboard reaches the API
  on the same origin (the Vite proxy in dev), so no CORS is needed. Cost:
  non-browser clients have to handle a cookie until API tokens exist.
- **Setup checks and inserts under a table lock.** Two browsers racing through
  first-run setup must not both create an Owner. `LOCK TABLE users IN
  EXCLUSIVE MODE` in the Setup transaction serialises them; the second gets a
  conflict. The table is still called `users` so teams can add rows later
  without a rename.
