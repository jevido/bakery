# routing

- Subdomain: supporting
- Hosted in: `services/api` (module `contexts/routing`)

## Purpose

Makes Domains reach Containers. Owns the Proxy (the Caddy container
`bakery-proxy`: it exists, runs and sits on the `bakery` network) and the
Routes, and renders the whole Caddy configuration from them. It is **not**
responsible for which Container is current; deployments tells it.

## Language

| Term | Meaning |
| ---- | ------- |
| Proxy | The Caddy container `bakery-proxy`. |
| Route | Domain → container name and port. One per Application. |
| Dashboard Route | Bakery's own dashboard domain: `/api/*` to the API container, everything else to the dashboard container. From configuration, not a stored Route. |
| Apply | Render the full Caddy JSON config from all Routes and load it with `POST /load`. |
| ACME | Certificates from an ACME CA (Let's Encrypt by default; any directory URL, e.g. Pebble in tests), used on a Server when Internal TLS is off. |
| Internal TLS | Certificates from Caddy's own CA, for `*.localhost` in development. |

## Model

### Aggregates

| Aggregate | Invariants |
| --------- | ---------- |
| Route | One per Application; its Domain is unique. It always points at a Container that was running when the Route was switched. |

### Commands

- `EnsureProxy()`: at API start, create or start `bakery-proxy` and Apply.
- `SwitchRoute(applicationID, domain, container, port)`: upsert the Route, then Apply.
- `DropRoute(applicationID)`: on `ApplicationDeleted`, then Apply.

### Domain events

None.

## Integration

- **Publishes:** `SwitchRoute` for deployments.
- **Consumes:** `ApplicationDeleted` from projects.

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
