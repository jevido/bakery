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
├── app/                # SvelteKit UI (JS only, adapter-bun build)
├── backend/            # Bun backend (controllers, routes, models, lib)
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

1. **Copy the installer** to your target host and run it as root:

   ```bash
   sudo ./infrastructure/scripts/install.sh --repo https://github.com/your-org/bakery.git --certbot-email you@example.com
   ```

   The installer will:

   - Install Bun, Git, PostgreSQL, Nginx, Certbot, Docker, and build prerequisites.
   - Clone Bakery (or use the current checkout), install dependencies, and build the SvelteKit app.
   - Create `/var/lib/bakery` data/logs/build directories and seed Postgres with the Bakery metadata database.
   - Generate a `.env` file with random session/encryption secrets and the server's public IP.
   - Run database migrations and seed an initial admin account.
   - Render and enable the systemd service (`bakery.service`) pointing to the selected install directory.
   - Seed an Nginx base template ready for Certbot-managed certificates.

2. **Record the printed admin credentials** – the installer outputs the generated email and password. Use them to log in and immediately change the password inside the UI.

3. **Configure DNS** for your instance domain, then use the Domain Management section per deployment to trigger SSL issuance through Certbot.

4. **Link GitHub** from *Deployments → New → Link GitHub*. Once linked, your repositories become selectable for deployments.

5. **Deploy Applications**
   - Choose branch, domains, environment variables, and optional blue/green + Postgres provisioning.
   - Bakery builds and boots the app, wiring Nginx, SSL, and systemd or Docker as appropriate.

6. **Self-Update** from *Settings → Update Bakery* (calls `infrastructure/scripts/update.sh`, git pulls, rebuilds the UI, applies migrations, restarts service).

## Development Workflow

1. Install dependencies:

   ```bash
   bun install
   cd app && bun install && cd ..
   cp .env.example .env
   # update .env DATABASE_URL etc. for your local Postgres
   bun backend/lib/migrate.js
   ```

2. Run services in separate terminals:

   ```bash
   bun backend/server.js       # Backend API + SvelteKit handler
   cd app && bun run dev       # Optional if you want hot reloading during UI work
   ```

   The backend serves the built SvelteKit app; during UI development, use the Vite dev server pointed at the API (`PUBLIC_API_URL=http://localhost:4100`).

3. Seed an admin account (first-time only):

   ```bash
   bun backend/scripts/create-admin.js admin@example.com password123
   ```

4. Open `http://localhost:5173/auth/login` in your browser to access the UI.

## Environment Variables

| Variable | Purpose |
| --- | --- |
| `DATABASE_URL` | Postgres connection string for Bakery's metadata database |
| `SESSION_SECRET` | Secret for HTTP session cookies |
| `ENCRYPTION_KEY` | 32-byte key encrypting GitHub tokens and env vars |
| `BAKERY_HOST` / `BAKERY_PORT` | Backend bind interface & port |
| `BAKERY_BASE_URL` | Public URL of the UI (used for OAuth callbacks) |
| `BAKERY_API_URL` | Base URL used by the frontend when generating API calls |
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
  - `/auth/login` – Sign-in form hitting the backend sessions API.
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
