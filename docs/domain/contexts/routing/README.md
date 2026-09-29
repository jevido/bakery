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
| Apply | Render the full Caddy JSON config from all Routes and load it with `POST /load`. |
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
- **Caddy reaches Containers by name on the `bakery` network**, so
  Application Containers publish no host ports.
- **The admin API has no authentication**, so it is published on
  `127.0.0.1` only.
- **In development the Proxy listens on 4940/4943 with Internal TLS** and HTTP
  to HTTPS redirects off (they would point at 443). On a real Server the same
  code uses 80/443 and ACME; only configuration changes.
