# Bakery

Bakery is a self-hosted deployment control plane inspired by Coolify. It combines a Bun-powered backend with a SvelteKit frontend to manage multi-application deployments, blue-green releases, SSL automation, analytics, and Postgres lifecycle management from a single interface.

## Features

- **Single Pane of Glass** – Login-protected dashboard for deployments, domains, databases, analytics, logs, and task activity.
- **GitHub Integration** – OAuth flow to link accounts, enumerate repositories/branches, and trigger builds straight from the UI.
- **Blue-Green Deployments** – Automatic slot management with zero-downtime cutovers, rollback history, and live log streaming.
- **Runtime Flexibility** – Detects Dockerfile projects or falls back to Bun processes with generated systemd units and Nginx virtual hosts.
- **PostgreSQL Automation** – One-click provisioning, credential injection into deployment environment variables, and size tracking.
- **Infrastructure Glue** – Manages Nginx reverse proxy, Certbot SSL certificates, systemd units, Docker containers, and crash recovery tasks.
- **Self-Updating** – GUI-triggered updater executes `infrastructure/scripts/update.js` to pull changes, run migrations, rebuild the app, and restart services.

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
│   ├── scripts/        # install.sh (control-plane), update.js (self-update)
│   └── systemd/        # bakery.service + deployment unit template
├── scripts/
│   ├── install-bakery.sh      # Interactive control-plane installer
│   ├── update-bakery.sh       # Manual updater wrapper
│   └── install-node-agent.sh  # External node installer
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

1. **SSH into your Ubuntu server** (root or sudo privileges required). Run the one-line installer **on the server itself** and answer the prompts:

   ```bash
   sudo bash -c 'bash <(curl -fsSL https://raw.githubusercontent.com/jevido/bakery/main/scripts/install-bakery.sh)'
   ```

   The script walks you through the required values (base URL, Certbot email, GitHub OAuth app, admin credentials), installs dependencies, clones Bakery, runs the infrastructure installer, and enables the nightly self-update timer. It also reminds you which DNS A records to create (e.g. `bakery.example.com` plus a wildcard for app subdomains).

2. **Record the admin credentials** printed at the end of the installer. Log in at your chosen `--base-url` and change the password.

3. **Add the suggested DNS records** in Namecheap (or your DNS provider) so both the control plane and any `*.app` subdomains resolve to your server.

4. **Link GitHub** from _Deployments → New → Link GitHub_. Once linked, repositories become selectable in the deployment wizard.

5. **Deploy your applications**: choose the repo/branch, domains, environment variables, optional database provisioning, and the target server node (the control plane itself or an external node you add later).

6. **Stay current**: Bakery auto-runs `infrastructure/scripts/update.js` nightly. You can still trigger manual updates from _Settings → Update Bakery_ or run the wrapper script manually:

   ```bash
   curl -fsSL https://raw.githubusercontent.com/jevido/bakery/main/scripts/update-bakery.sh | sudo bash
   ```

### GitHub App webhook for self-updates

The control plane already exposes a webhook listener at `/api/webhooks/github` that can receive signed `push` events from GitHub. When the repository and branch match the values configured in `BAKERY_SELF_REPO` and `BAKERY_SELF_BRANCH`, the webhook calls `startSelfUpdate`, which runs `infrastructure/scripts/update.js` (the same updater you run manually).

To enable this:

1. Set the following environment variables in `/opt/bakery/.env` (or wherever your service reads its environment):
   - `BAKERY_SELF_REPO=<owner>/<repo>` (defaults to `jevido/bakery`).
   - `BAKERY_SELF_BRANCH=<branch>` (defaults to `main`).
   - `GITHUB_APP_WEBHOOK_SECRET=<secret>` – a random string (`openssl rand -hex 32` is a good choice). This secret must match the value you configure on the GitHub App/webhook and is used to verify the `x-hub-signature-256` header.
2. Restart the `bakery.service` so the new variables take effect (`sudo systemctl restart bakery`).
3. Create a GitHub App or webhook for the same repository:
   - Set the **Payload URL** to `https://<your-base-url>/api/webhooks/github` (same as `BAKERY_BASE_URL` + `/api/webhooks/github`).
   - Choose **Content type: application/json**.
   - Paste the same secret you stored in `GITHUB_APP_WEBHOOK_SECRET`.
   - Subscribe to **Push** events (optionally other events if you need them later).
   - Install the app/webhook on the repository so GitHub can deliver events.

GitHub will now notify Bakery of every push. When the push targets the configured branch, the webhook will log the request, verify the signature, and begin the updater unless another run is in progress. You can watch `/var/log/bakery/bakery-service.log` for the `self-update` entries to confirm it worked.

Administrators can also visit the **System → Self-update** page to see whether an updater job is currently running, when the last run finished, and whether the GitHub App webhook is configured properly.

### Single-node “Platform” install (Hetzner, etc.)

Once your Hetzner (or other) Ubuntu server is provisioned, SSH into it and run the same installer:

```bash
sudo bash -c 'bash <(curl -fsSL https://raw.githubusercontent.com/jevido/bakery/main/scripts/install-bakery.sh)'
```

Provide your base URL, Certbot email, GitHub OAuth details, and desired admin credentials when prompted. The script handles dependency installation, cloning Bakery, wiring `.env`, and enabling the nightly self-update timer.

## Adding External Nodes

Bakery can delegate builds, Docker runtime, and Nginx management to additional hosts while you operate everything from the primary GUI.

1. Open **Servers** in the sidebar and click _Add a server_. Choose a friendly name so you can recognise the node later.
2. Click **Link node** to copy the prefilled installer command, SSH into the remote machine, and run it as root. The script installs Docker/nginx, creates the `bakery-agent` user, provisions the directories, and drops the control plane’s SSH key in `~bakery-agent/.ssh/authorized_keys`:

   ```bash
   curl -fsSL https://raw.githubusercontent.com/jevido/bakery/main/scripts/install-node-agent.sh | \
     sudo SSH_USER=bakery-agent SSH_KEY_BASE64="<long-string>" bash
   ```

   When the installer finishes, return to the Servers page, enter the node’s reachable host/IP, and click **Verify & activate**. Bakery establishes an SSH session with the generated key and marks the node active.
3. Once verified, the node appears as **Active**; deployments can now target it directly from the control plane.
4. New deployments expose a **Server node** selector. Pick the remote node to run clones, builds, Nginx, and Certbot there, or choose the control plane to keep workloads local.

### Push-based agent roadmap

Bakery’s node agent currently polls the control plane for work, which requires the agent to be online whenever the control plane needs to trigger an update. You asked for the inverse: the control plane should open an SSH channel to the nodes and push commands (deploy/update) on demand. These are the phases we’ll execute:

1. **Phase 1 — SSH command channel**
   - Collect SSH metadata for each node (host, port, user, key fingerprint) and validate it when creating/updating the node.
   - Teach the control plane to open `ssh bakery-agent@node` connections and run scripts that boot the agent, deploy builds, or perform self-updates.
   - Provide a lightweight bootstrap (install-node-agent) script that sets up the `bakery-agent` user, copies the control plane’s public key, and acknowledges the node registration.
2. **Phase 2 — Control-plane driven CI/CD**
   - Route the GitHub App webhook through the new SSH runner so every push to the configured repo/branch executes `infrastructure/scripts/update.js` on the node.
   - Report webhook delivery status in System → Self-update and tie it to the SSH execution logs for easier debugging.
3. **Phase 3 — Manual control**
   - Add “Update now” / “Deploy now” buttons next to each node (and on the System self-update page) that trigger the SSH runner manually.
   - Stream the result logs back to Bakery and raise alerts when SSH execution fails, so you can see the full lifecycle without dropping into the server directly.

Completing these phases means your laptop-hosted control plane can push updates to the Hetzner node regardless of whether the agent is polling, and GitHub pushes still trigger the same workflow.

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
 bun run db
```

(`bun run dev:db` remains available as an alias.)

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

7. Start the lightweight deployment agent in another terminal (no sudo needed):

   ```bash
   BAKERY_LOCAL_MODE=1 bun agent/index.js
   ```

   This runs the agent loop against your dev server, spawns Bun apps directly, and writes per-deployment config/log files into `.data/`.

8. From the UI create deployments as usual. Each slot listens on an automatically assigned port (base `5200` by default); no Nginx/systemd interaction happens when `BAKERY_LOCAL_MODE=1`, so you can hit the app via `http://127.0.0.1:<port>`.

## Environment Variables

| Variable                                                    | Purpose                                                                    |
| ----------------------------------------------------------- | -------------------------------------------------------------------------- |
| `DATABASE_URL`                                              | Postgres connection string for Bakery's metadata database                  |
| `SESSION_SECRET`                                            | Secret for HTTP session cookies                                            |
| `ENCRYPTION_KEY`                                            | 32-byte key encrypting GitHub tokens and env vars                          |
| `BAKERY_HOST` / `BAKERY_PORT`                               | Backend bind interface & port                                              |
| `BAKERY_BASE_URL`                                           | Public URL of the UI (used for OAuth callbacks)                            |
| `BAKERY_PUBLIC_IP` / `BAKERY_PUBLIC_IPV6`                   | Public IPv4/IPv6 addresses for DNS guidance and verification               |
| `BAKERY_DATA_DIR` / `BAKERY_LOGS_DIR` / `BAKERY_BUILDS_DIR` | File system locations for build artifacts, data snapshots, and log storage |
| `GITHUB_CLIENT_ID` / `GITHUB_CLIENT_SECRET`                 | GitHub OAuth credentials                                                   |
| `CERTBOT_EMAIL`                                             | Email passed to Certbot when issuing certificates                          |
| `BAKERY_LOCAL_MODE`                                         | When truthy, skips systemd/nginx actions and runs deployments directly (defaults to on in dev) |

## Local Deployment Runtime

Setting `BAKERY_LOCAL_MODE=1` (the default while `NODE_ENV !== 'production'`) activates a self-contained runtime so you can exercise deployments without touching systemd, nginx, or Certbot on your workstation.

- Deployments are launched by the agent via `bun run start` and tracked in-memory — no sudo required.
- Generated unit files, nginx templates, and logs are written to `.data/systemd`, `.data/nginx/sites-enabled`, and `.data/logs` for inspection.
- Nginx reloads and Certbot requests are skipped automatically; domains can still be created and you can inspect the rendered config, but you should hit the app directly on its assigned port (default range `5200+`).
- Docker-based repositories still use your local Docker CLI if available; otherwise the task will fail just like in production.
- Toggle the behaviour explicitly by setting `BAKERY_LOCAL_MODE=0` (to test real systemd/nginx) or `1`.

## Key Backend Components

- `backend/server.js` – Bun HTTP server that routes `/api/*` requests to the JSON API and everything else to the pre-built SvelteKit handler.
- `backend/lib/` – Shared utilities for configuration, Postgres connectivity (`bun:postgres`), GitHub OAuth, deployment orchestration, systemd, Docker, Nginx templating, and analytics collection.
- `backend/controllers/` – Auth, deployments, domains, databases, and system endpoints consumed by the UI.
- `backend/models/` – Database access helpers wrapping SQL queries per aggregate.
- `backend/lib/tasks.js` – Background worker polling the `tasks` table to execute deployments, rollbacks, analytics snapshots, and crash restarts.
- `backend/migrations/` – SQL migrations applied through `scripts/migrate.js` (run via `bun run migrate`).

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

- Backend endpoint `POST /api/system/update` executes `infrastructure/scripts/update.js`, which pulls the latest code, reinstalls dependencies, rebuilds the app, applies migrations, and restarts the `bakery` systemd service.
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
3. Add or adjust migrations through SQL files in `backend/migrations/` and run `bun run migrate` to apply them locally.
4. Submit PRs with clear descriptions of backend/app updates plus any new infrastructure templates.

## License

Bakery is provided under the MIT license. See the included `LICENSE` file for the full text.
