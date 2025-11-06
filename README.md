# Bakery

Bakery is a self-hosted deployment control plane inspired by Coolify. It combines a Bun-powered backend with a SvelteKit frontend to manage multi-application deployments, blue-green releases, SSL automation, analytics, and Postgres lifecycle management from a single interface.

## Features

- **Single Pane of Glass** – Login-protected dashboard for deployments, domains, databases, analytics, logs, and task activity.
- **GitHub Integration** – OAuth flow to link accounts, enumerate repositories/branches, and trigger builds straight from the UI.
- **Blue-Green Deployments** – Automatic slot management with zero-downtime cutovers, rollback history, and live log streaming.
- **Runtime Flexibility** – Detects Dockerfile projects or falls back to Bun processes with generated systemd units and Nginx virtual hosts.
- **PostgreSQL Automation** – One-click provisioning, credential injection into deployment environment variables, and size tracking.
- **Infrastructure Glue** – Manages Nginx reverse proxy, Certbot SSL certificates, systemd units, Docker containers, and crash recovery tasks.
- **Self-Updating** – GUI-triggered updater executes `infrastructure/scripts/update.sh` to pull changes, run migrations, rebuild the app, and restart services.

## Project Structure

```
bakery/
├── app/                # SvelteKit app (UI + backend routes, Bun adapter)
├── backend/
│   ├── migrations/     # SQL migrations executed by the SvelteKit server
│   └── scripts/        # Utility scripts (e.g. create-admin.js)
├── infrastructure/
│   ├── docker/         # Docker deployment templates for user apps
│   ├── nginx/          # Nginx configuration templates generated per deployment
│   ├── scripts/        # install.sh (bootstrap), update.sh (self-update)
│   └── systemd/        # bakery.service + deployment unit template
├── backend/migrations/ # Postgres schema migrations
└── README.md
```

## Requirements

- Ubuntu 22.04+ (or compatible systemd-based distribution)
- Root or sudo access (for installation, configuring systemd, nginx, certbot)
- PostgreSQL 14+
- Nginx & Certbot
- Docker (for Dockerfile-based deployments)

## Quick Start (Production)

1. **SSH into your Ubuntu server** (root or sudo privileges required). Run the one-line installer **on the server itself**:

   ```bash
   curl -fsSL https://raw.githubusercontent.com/jevido/bakery/refs/heads/main/scripts/install-control-plane.sh | sudo bash -s -- \
     --base-url https://bakery.jevido.nl \
     --certbot-email ops@example.com \
     --github-client-id YOUR_GITHUB_APP_CLIENT_ID \
     --github-client-secret YOUR_GITHUB_APP_CLIENT_SECRET \
     --github-webhook-secret YOUR_GITHUB_APP_WEBHOOK_SECRET
   ```

   The script installs the required system packages, clones Bakery, runs the installer, and enables a nightly self-update timer. It also prints the DNS A records you should create (e.g. `bakery.jevido.nl` and a wildcard for app subdomains) so you can copy them straight into Namecheap.

2. **Record the admin credentials** printed at the end of the installer. Log in at your chosen `--base-url` and change the password.

3. **Add the suggested DNS records** in Namecheap (or your DNS provider) so both the control plane and any `*.app` subdomains resolve to your server.

4. **Link GitHub** from *Deployments → New → Link GitHub*. Once linked, repositories become selectable in the deployment wizard.

5. **Deploy your applications**: choose the repo/branch, domains, environment variables, optional database provisioning, and the target server node (the control plane itself or an external node you add later).

6. **Stay current**: Bakery auto-runs `infrastructure/scripts/update.sh` nightly. You can still trigger manual updates from *Settings → Update Bakery* when needed.

### Single-node “Platform” install (Hetzner, etc.)

Once your Hetzner (or other) Ubuntu server is provisioned, SSH into it and run:

```bash
curl -fsSL https://raw.githubusercontent.com/the-bakery-app/bakery/main/scripts/install-control-plane.sh | sudo bash -s -- \
  --base-url https://bakery.jevido.nl \
  --certbot-email ops@example.com \
  --github-client-id YOUR_GITHUB_APP_CLIENT_ID \
  --github-client-secret YOUR_GITHUB_APP_CLIENT_SECRET \
  --github-webhook-secret YOUR_GITHUB_APP_WEBHOOK_SECRET
```

The script installs git/curl if needed, clones the public Bakery repository, runs the installer, wires the `.env` with your public URL and GitHub credentials, and enables the nightly auto-update timer. During installation it prints the DNS A records to configure (control plane host plus optional wildcard) so you can copy/paste them into Namecheap right away.

## Adding External Nodes

Bakery can delegate builds, Docker runtime, and Nginx management to additional hosts while you operate everything from the primary GUI.

1. Open **Servers** in the sidebar and click *Add a server*. Choose a friendly name so you can recognise the node later.
2. Copy the generated installer command and run it as root on the remote machine. The script installs Bun, Docker, Nginx, and the Bakery agent service.
3. When the installer finishes it prints a one-time pairing code. Paste the code back into the Servers page to activate the node.
4. New deployments expose a **Server node** selector. Pick the remote node to run clones, builds, Nginx, and Certbot there, or choose the control plane to keep workloads local.

Agents maintain a secure polling connection to the control plane over HTTPS. Allow outbound access from each node to the Bakery instance and inbound SSH for the installer command.

## Development Workflow

1. Install dependencies:

 ```bash
  bun install
  (cd app && bun install)
  cp .env.example .env
  cp app/.env.example app/.env
  ```

2. Launch the development database (foreground):

 ```bash
  bun run dev:db
  ```

  This streams Docker Compose logs until you hit <kbd>Ctrl</kbd>+<kbd>C</kbd>. Leave this terminal running while you work. The database uses the same credentials as `.env` (`postgres:postgres`), so if you previously ran the old container you may need to remove `tmp/postgres` to re-initialise it with the new user.

3. In a second terminal, apply database migrations as needed:

   ```bash
   bun run migrate
   ```

4. Finally, start the SvelteKit UI + API:

   ```bash
   bun run dev:web
   ```

5. Seed an admin account (first-time only, with the stack running):

   ```bash
   bun backend/scripts/create-admin.js admin@example.com password123
   ```

6. Open `http://localhost:5173/login` in your browser to access the UI.

## Environment Variables

| Variable | Purpose |
| --- | --- |
| `DATABASE_URL` | Postgres connection string for Bakery's metadata database |
| `SESSION_SECRET` | Secret for HTTP session cookies |
| `ENCRYPTION_KEY` | 32-byte key encrypting GitHub tokens and env vars |
| `BAKERY_HOST` / `BAKERY_PORT` | Backend bind interface & port |
| `BAKERY_BASE_URL` | Public URL of the UI (used for OAuth callbacks) |
| `BAKERY_PUBLIC_IP` | Public IP for DNS guidance and verification |
| `BAKERY_DATA_DIR` / `BAKERY_LOGS_DIR` / `BAKERY_BUILDS_DIR` | File system locations for build artifacts, data snapshots, and log storage |
| `GITHUB_CLIENT_ID` / `GITHUB_CLIENT_SECRET` | GitHub OAuth credentials |
| `CERTBOT_EMAIL` | Email passed to Certbot when issuing certificates |

## Key Backend Components

- `backend/server.js` – Bun HTTP server that routes `/api/*` requests to the JSON API and everything else to the pre-built SvelteKit handler.
- `backend/lib/` – Shared utilities for configuration, Postgres connectivity (`bun:postgres`), GitHub OAuth, deployment orchestration, systemd, Docker, Nginx templating, and analytics collection.
- `backend/controllers/` – Auth, deployments, domains, databases, and system endpoints consumed by the UI.
- `backend/models/` – Database access helpers wrapping SQL queries per aggregate.
- `backend/lib/tasks.js` – Background worker polling the `tasks` table to execute deployments, rollbacks, analytics snapshots, and crash restarts.
- `backend/migrations/` – SQL migrations applied through `backend/lib/migrate.js`.

## Frontend Highlights

- Built with SvelteKit (no TypeScript) using the Node adapter.
- Layout provides a persistent sidebar with deployment status indicators and a top bar for breadcrumbs/account controls.
- Pages include:
  - `/login` – Sign-in form hitting the backend sessions API.
  - `/` – Dashboard metrics (disk usage, task activity).
  - `/deployments` – Deployment summary list with status badges.
  - `/deployments/new` – Wizard for new deployments (GitHub repo selection, domains, env vars, blue-green, Postgres toggle).
  - `/deployments/[id]` – Detail view with actions (redeploy, restart, rollback), environment editor, domain verification, database provisioning, history table, and live log stream.
- `$lib/api.js` centralises authenticated fetch calls with automatic JSON parsing and error handling.

## Deployment Slots & Rollback

- Deployments default to a single `blue` slot, but enabling blue-green toggles between `blue` and `green` to allow zero-downtime releases.
- Each deployment version stores commit SHA, slot, port, and runtime type. Up to five historical versions are retained per app.
- Rollback requests enqueue a task that switches Nginx/systemd back to the selected slot and restarts the service.

## Docker vs Bun Apps

- Repositories containing a `Dockerfile` are built with `docker build` and launched through `docker run` (managed containers named `bakery-deployment-<id>-<slot>`).
- Non-Docker projects run via Bun with generated systemd units that execute `bun run start` within the checkout; environment variables (including generated database URLs) are injected via the unit file.

## Certificates & Domains

- Users add domains in the deployment UI; Bakery renders Namecheap-style DNS instructions and stores the desired hostnames.
- The **Verify DNS** button performs an `A` record lookup to ensure the domain points at `BAKERY_PUBLIC_IP` before triggering Certbot with `--nginx`.
- Nginx templates are regenerated for every deployment change, including blue/green slot switches, and `systemctl reload nginx` is issued automatically.

## Database Lifecycle

- Postgres provisioning is handled via `psql`, creating a dedicated role and database per deployment, with credentials stored encrypted and injected as `DATABASE_URL`.
- Database inventory and size tracking are exposed through the GUI; dropping a deployment schedules the cleanup task, which removes databases and service units.

## Update Workflow

- Backend endpoint `POST /api/system/update` executes `infrastructure/scripts/update.sh`, which pulls the latest code, reinstalls dependencies, rebuilds the app, applies migrations, and restarts the `bakery` systemd service.
- Update progress is reflected in the activity feed sourced from the `tasks` table.

## Logging & Analytics

- Deployment logs are persisted in the `deployment_logs` table and pushed to the frontend over Server-Sent Events (~4s interval polling).
- Nginx access logs feed basic traffic/bandwidth metrics stored in `analytics_snapshots`, while filesystem probing tracks disk usage.
- Crash detection runs every two minutes, scanning recent logs for error signatures and enqueuing restart tasks when necessary.

## Docker Templates for User Apps

- `infrastructure/docker/Dockerfile.template` and `infrastructure/docker/compose.template.yml` provide opinionated defaults for teams that want to standardise containerised builds (Bun base image, exposed port, restart policy).

## Contributing

1. Fork and clone the repository.
2. Use `bun run lint` inside `app` to keep Svelte formatting consistent.
3. Add or adjust migrations through SQL files in `backend/migrations/` and run `bun backend/lib/migrate.js` to apply them locally.
4. Submit PRs with clear descriptions of backend/app updates plus any new infrastructure templates.

## License

Bakery is provided under the MIT license. See the included `LICENSE` file for the full text.
