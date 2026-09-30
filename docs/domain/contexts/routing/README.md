# routing

- Subdomain: supporting
- Hosted in: `services/api` (module `contexts/routing`)

## Purpose

Makes Domains reach Containers. Owns the Proxy (the Caddy container
`bakery-proxy`: it exists, runs and sits on the `bakery` network) and the
Routes and the Route settings, and renders the whole Caddy configuration from them. It is **not**
responsible for which Container is current; deployments tells it.

## Language

| Term | Meaning |
| ---- | ------- |
| Proxy | The Caddy container `bakery-proxy`. |
| Route | An Application's Domains → container name and port. One per Application. |
| Route settings | How the Proxy treats an Application's traffic: Www redirect, Response headers, Basic auth. One per Application, default all off. |
| Www redirect | `off`, `to_apex` or `to_www`. Each Domain's counterpart (`www.` added or removed) answers 308 to the Domain, keeping path and query. |
| Counterpart | The host a Www redirect adds for one Domain. |
| Response header | Name and value set on every response. |
| Basic auth | One username + password asked for before any request is proxied. |
| Dashboard Route | Bakery's own dashboard domain: `/api/*` to the API container, everything else to the dashboard container. From configuration, not a stored Route. |
| Apply | Render the full Caddy JSON config from all Routes and load it with `POST /load`. |
| ACME | Certificates from an ACME CA (Let's Encrypt by default; any directory URL, e.g. Pebble in tests), used on a Server when Internal TLS is off. |
| Internal TLS | Certificates from Caddy's own CA, for `*.localhost` in development. |

## Model

### Aggregates

| Aggregate | Invariants |
| --------- | ---------- |
| Route | One per Application, with at least one Domain. It always points at a Container that was running when the Route was switched. (That Domains are unique is projects' rule.) |
| Route settings | One per Application, stored even before it has a Route. Www redirect is `off`, `to_apex` or `to_www`. At most 20 Response headers, names are HTTP tokens, listed once (case-insensitively), never hop-by-hop (`Connection`, `Keep-Alive`, `Proxy-*`, `TE`, `Trailer`, `Transfer-Encoding`, `Upgrade`) or `Content-Length`; values on one line, at most 1024 characters. Basic auth, when on, has a username (1–100 characters, no `:`) and a password hash; only the bcrypt hash is kept and it is never returned. |

### Commands

- `EnsureProxy()`: at API start, create or start `bakery-proxy` and Apply.
- `SwitchRoute(applicationID, domains, container, port)`: upsert the Route, then Apply.
- `ChangeDomains(applicationID, domains)`: on `ApplicationDomainsChanged`; moves an existing Route to the Domains, then Applies. No Route yet: nothing to do.
- `ChangeRouteSettings(applicationID, settings)`: store, then Apply.
- `DropRoute(applicationID)`: on `ApplicationDeleted`, drops the Route and the Route settings, then Applies.

### Domain events

None.

## Integration

- **Publishes:** `SwitchRoute` for deployments; the Route settings over HTTP (`GET/PUT /api/applications/{id}/routing`) for the dashboard.
- **Consumes:** `ApplicationDeleted` and `ApplicationDomainsChanged` from projects.

## Why it's shaped this way

- **Caddy is configured only through its admin API, and always with the full
  config.** Rendering everything from the `routes` table and loading it with
  `POST /load` means Caddy holds no state Bakery does not have: a lost or
  recreated Proxy is fixed by one Apply. Cost: every change re-sends the whole
  config, which is fine for hundreds of routes.
- **The Dashboard Route is rendered from configuration, not stored as a
  Route.** It has no Application and must be served before the first
  deploy, so it lives in `BAKERY_DASHBOARD_DOMAIN` and is rendered first on
  every Apply. `/api/*` and the dashboard share one origin, which keeps the
  `SameSite=Strict` session cookie working without CORS. Projects refuses
  the domain for Applications so the two never collide.
- **Caddy reaches Containers by name on the `bakery` network**, so
  Application Containers publish no host ports.
- **The admin API has no authentication**, so it is published on
  `127.0.0.1` only in development, and not published at all on a server,
  where the API runs on the `bakery` network and reaches it as
  `bakery-proxy:2019`.
- **Caddy autosaves its last config** (`caddy run --resume`, volume
  `bakery-proxy-config`), so after a host reboot the Proxy serves the last
  Routes even before the API is back.
- **In development the Proxy listens on 127.0.0.1:4940/4943 with Internal TLS** and HTTP
  to HTTPS redirects off (they would point at 443). On a real Server the same
  code uses 80/443 and ACME; only configuration changes.
- **With ACME, `:80` is Caddy's, not ours.** The rendered config has no
  plain-HTTP server then, so Caddy runs its own on `:80` that answers
  HTTP-01 challenges and redirects everything else to HTTPS. With Internal
  TLS the same Routes are also served over plain HTTP, since redirects would
  point at 443 while development publishes 4943.
- **An extra ACME root is copied into the Proxy through the Podman API**
  (`/data/bakery/acme-root.pem`) on every EnsureProxy, so a private CA such
  as Pebble can stand in for Let's Encrypt without building a Caddy image.
- **Route settings live here, not on the Application in projects.** They
  are about what the Proxy does with traffic, which is routing's language,
  and they need no Deployment, so routing owns a small API of its own and a
  `route_settings` table keyed by application id (no foreign key, like
  `routes`). The Domains stay in projects, which checks they are unique.
- **An explicit Domain wins over a counterpart.** When a Www redirect's
  counterpart is another Application's Domain, the counterpart is not
  rendered, so one Application's redirect setting can never take traffic
  from another.
- **Basic auth keeps only a bcrypt hash**, which is also what Caddy's
  `http_basic` provider wants; the password is never stored or returned, and
  Health checks are unaffected because they run inside the Container.
- **Response headers are set, not added**, so a header the app already sends
  is replaced rather than doubled.
