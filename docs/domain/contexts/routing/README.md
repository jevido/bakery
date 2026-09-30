# routing

- Subdomain: supporting
- Hosted in: `services/api` (module `contexts/routing`)

## Purpose

Makes Domains reach Containers. Owns the Proxies (the Caddy container
`bakery-proxy` on each Server that has Routes: it exists, runs and sits on that Server's `bakery` network) and the
Routes, the Preview routes, the Service routes and the Route settings, and renders the whole Caddy configuration from them. It is **not**
responsible for which Container is current; deployments tells it.

## Language

| Term | Meaning |
| ---- | ------- |
| Proxy | The Caddy container `bakery-proxy` on one Server. The Local server's also serves the Dashboard Route and the Service routes. |
| Remote Proxy | The Proxy on a Remote server. Its admin API is a unix socket in the volume `bakery-proxy-admin`, reached over the Server connection. |
| Route | An Application's Domains → container name and port on its Target server. One per Application. |
| Preview route | A Preview's Preview domain (`pr-<n>.<primary Domain>`) → its container name and port on the Application's Server. One per Application and Preview number. |
| Service route | A Public Component's Domains → its Container and port. One per Public Component of a Service. |
| Route settings | How the Proxy treats an Application's traffic: Www redirect, Response headers, Basic auth. One per Application, default all off. |
| Www redirect | `off`, `to_apex` or `to_www`. Each Domain's counterpart (`www.` added or removed) answers 308 to the Domain, keeping path and query. |
| Counterpart | The host a Www redirect adds for one Domain. |
| Response header | Name and value set on every response. |
| Basic auth | One username + password asked for before any request is proxied. |
| Dashboard Route | Bakery's own dashboard domain: `/api/*` to the API container, everything else to the dashboard container. From configuration, not a stored Route. |
| Apply | For one Server: render the full Caddy JSON config from that Server's Routes (plus, on the Local server, the Service routes and the Dashboard Route) and load it into that Server's Proxy with `POST /load`. |
| ACME | Certificates from an ACME CA (Let's Encrypt by default; any directory URL, e.g. Pebble in tests), used on a Server when Internal TLS is off. |
| Internal TLS | Certificates from Caddy's own CA, for `*.localhost` in development. |

## Model

### Aggregates

| Aggregate | Invariants |
| --------- | ---------- |
| Route | One per Application, with at least one Domain, on one Server. It always points at a Container on that Server that was running when the Route was switched. (That Domains are unique is projects' rule.) |
| Preview route | One per (Application, Preview number), on the Application's Server, pointing at a Container that was running when it was switched. Rendered with the Application's Response headers and Basic auth but never a Www redirect, and after every Route and Service route, so a Domain someone set explicitly always wins over a Preview domain. |
| Service route | One per (Service, Component), with at least one Domain, pointing at a Container that was running when the Service's routes were set. A Service's routes are always replaced as a set. Rendered with default Route settings. |
| Route settings | One per Application, stored even before it has a Route. Www redirect is `off`, `to_apex` or `to_www`. At most 20 Response headers, names are HTTP tokens, listed once (case-insensitively), never hop-by-hop (`Connection`, `Keep-Alive`, `Proxy-*`, `TE`, `Trailer`, `Transfer-Encoding`, `Upgrade`) or `Content-Length`; values on one line, at most 1024 characters. Basic auth, when on, has a username (1–100 characters, no `:`) and a password hash; only the bcrypt hash is kept and it is never returned. |

### Commands

- `EnsureProxy()`: at API start, create or start the Local server's `bakery-proxy` and Apply; then, in the background, the same for every Remote server that has Routes.
- `SwitchRoute(server, applicationID, domains, container, port)`: upsert the Route, ensure that Server's Proxy (creating it on a Server's first Route), then Apply that Server.
- `ChangeDomains(applicationID, domains)`: on `ApplicationDomainsChanged`; moves an existing Route to the Domains, then Applies. No Route yet: nothing to do.
- `ChangeRouteSettings(applicationID, settings)`: store, then Apply.
- `SwitchPreviewRoute(server, applicationID, preview, domains, container, port)`: like `SwitchRoute`, for a Preview.
- `DropPreviewRoute(applicationID, preview)`: when a Preview closes; removes it, then Applies its Server.
- `DropRoute(applicationID)`: on `ApplicationDeleted`, drops the Route, the Preview routes and the Route settings, then Applies.
- `SetServiceRoutes(serviceID, routes)`: replaces the Service's Service routes, then Applies.
- `DropServiceRoutes(serviceID)`: removes them, then Applies.

### Domain events

None.

## Integration

- **Publishes:** `SwitchRoute`, `SwitchPreviewRoute` and `DropPreviewRoute` for deployments; `SetServiceRoutes` and `DropServiceRoutes` for services; the Route settings over HTTP (`GET/PUT /api/applications/{id}/routing`) for the dashboard.
- **Consumes:** `ApplicationDeleted` and `ApplicationDomainsChanged` from projects; `servers.Connect` to run and configure a Remote Proxy.

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
- **Service routes are their own table, rendered as Routes.** A Service
  has several Public Components and no Application id, so `routes` (one
  per Application) cannot hold them; `service_routes` is keyed by
  (service id, component) and turned into the same Route shape before
  rendering, so the Caddy config stays one full render of everything the
  Proxy serves. They have no Route settings yet.
- **A Proxy per Server, not one central Proxy.** Each Server serves its own
  Applications on its own 80/443, so the DNS of an Application's Domain
  points at the Server it runs on, as in Coolify. A central Proxy would need
  host ports on every Server and traffic between hosts, and would make
  Bakery's own server a single point of failure for all of them. Each
  Server's Proxy gets only its own Routes; Apply works per Server, so a
  change on one Server never reloads another.
- **A Remote Proxy's admin API is a unix socket in a volume**
  (`bakery-proxy-admin`, mounted at `/run/bakery-admin`), opened through the
  Server's SSH connection (`direct-streamlocal`, like the Podman socket) at
  the volume's mountpoint. No port on the Server is published for it, so
  nothing clashes and nothing outside can reach the unauthenticated admin
  API.
- **Service routes and the Dashboard Route stay on the Local server**:
  Services still run there, and the dashboard is Bakery's own.
- **Preview routes are a table of their own**, not extra rows of Route. A
  Route is one per Application (the upsert relies on it) and projects owns
  its Domains; a Preview route's Domain is derived, and it disappears when
  its Pull request closes. Rendering it with the Application's Route
  settings keeps an Application behind Basic auth protected in its Previews
  too; a Www redirect makes no sense for a derived host.
