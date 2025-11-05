# 🍞 Bakery — Full Project Roadmap

Bakery is a self-hosted deployment manager for multiple apps on a single instance.  
Built with **SvelteKit (no TypeScript)**, **Bun**, **PostgreSQL**, and **NGINX + Certbot**, it automates deployments, SSL, analytics, CI/CD, and rollback — all through a GUI.  
All user apps run on their own instance; Bakery provides orchestration, monitoring, and management.

---

## Phase 0 · Foundations

**Goal:** establish repo structure, runtime scaffolding, and base infrastructure.

- Lock repo structure:
  - `app/` → SvelteKit GUI
  - `backend/` → Bun backend services
  - `infrastructure/` → install scripts, systemd, NGINX templates
- Implement environment configuration:
  - Config loader for `.env` and production secrets
  - Runtime detection (Bun, Docker)
- Stand up Bun service shell:
  - PostgreSQL connection pool via `bun.sql`
  - Migration framework
  - Base auth tables (`users`, `sessions`, `accounts`)
- Bootstrap SvelteKit frontend:
  - Global layout + routing skeleton
  - Shared UI kit import
  - Auth guard layout for protected routes

---

## Phase 1 · Core Platform Services

**Goal:** enable user accounts, GitHub integration, and base deployment management.

- Implement user authentication:
  - Bcrypt password auth
  - Session management (JWT or signed cookies)
  - Initial admin bootstrap
- GitHub OAuth integration:
  - Secure token storage
  - Repo + branch listing
  - Webhook provisioning for CI/CD triggers
- Core domain models:
  - `apps`, `builds`, `deployments`, `environments`, `slots`
  - CRUD APIs for managing deployments
- Orchestrator service:
  - Queue build/deploy tasks
  - Emit live status via SSE/WebSockets
- System integration adapters:
  - NGINX + Certbot template engine
  - Systemd & Docker adapters for process management
  - Audit logs for all lifecycle events
- Database integration:
  - Each deployment can request a PostgreSQL database
  - Bakery auto-generates DB, injects `DATABASE_URL`, manages migrations

---

## Phase 2 · Deployment Engine

**Goal:** implement build pipelines, blue-green flow, env management, and crash recovery.

- Build pipeline:
  - Detect Bun vs Docker builds automatically
  - Artifact storage (local `/deployments/` folder)
  - Port assignment and service isolation
- Blue-green deployment:
  - Health checks on new version
  - Zero-downtime switch-over
  - Rollback history (retain last 5 versions)
- Environment management:
  - Secure env var vault
  - Template rendering into runtime files
  - Masked logs for secrets
- Crash detection and recovery:
  - Watchdog service monitors process health
  - Auto-restart crashed deployments (systemd unit regen)
- CI/CD automation:
  - On push to selected branch → rebuild and redeploy
  - Delayed deployments (scheduled start time)
  - Manual rollback via GUI

---

## Phase 3 · Observability & Analytics

**Goal:** add full analytics, disk metrics, and predictive alerts.

- Analytics ingestion:
  - Parse NGINX logs for traffic, requests, and errors
  - Collect CPU, RAM, and Disk stats per deployment
- Disk usage tracking:
  - Track total + per-deployment usage
  - Track Postgres DB size
- Predictive disk alerts:
  - Forecast low-space conditions and alert users
- Aggregation + storage:
  - Daily summaries stored in `analytics` tables
- Data exposure:
  - Live metrics and charts over SSE/WebSocket streams
- GUI analytics dashboard:
  - CPU/RAM/Disk usage
  - Traffic + error rates
  - Predictive alerts and space status

---

## Phase 4 · GUI Experience

**Goal:** deliver complete GUI-driven management with dashboards and wizards.

- **Sidebar:**
  - Deployment list with health/status badges
- **Dashboards:**
  - Global metrics overview (CPU, RAM, disk, apps count)
  - Activity feed + recent builds
- **Deployment detail view:**
  - Tabs for Overview / Domains / Env Vars / Builds / Logs / Analytics
  - Rollback + Redeploy buttons
- **Wizards & Modals:**
  - New Deployment (GitHub repo + branch + subdomain + env vars)
  - Domain attach/verify + DNS record instructions
  - Database provisioning wizard
  - Environment variable editor (masking secrets)
- **Logs & Tests:**
  - Live build/runtime logs viewer
  - Pre-deploy tests: run project test suite before build; block on fail
- **Account & Integration:**
  - GitHub link management
  - Account settings, roles (Admin, Developer, Viewer)
  - Multi-user support

---

## Phase 5 · Infrastructure Tooling

**Goal:** finalize installation, self-updates, and system-level integration.

- **Install Script (`install.sh`):**
  - Installs dependencies: git, bun, nginx, certbot, postgresql
  - Clones Bakery repo, runs setup + migrations
  - Configures systemd + NGINX base templates
  - Seeds initial admin user
- **Systemd Units:**
  - Manage Bakery backend and all deployed apps
- **Nginx Templates:**
  - Reverse proxy config for each deployment
  - Certbot certificate automation
- **Log Management:**
  - Logrotate setup for NGINX + Bakery logs
- **Self-Update Mechanism:**
  - Bakery updates itself via CI/CD (pulls latest, runs migrations, restarts)
  - Manual “Check for updates” option in GUI
- **Dev Environment:**
  - Docker Compose for local dev (Bun app + Postgres + Nginx stub)

---

## Phase 6 · Extended Capabilities

**Goal:** expand Bakery beyond a single instance and refine ecosystem.

- Remote Agents:
  - Allow managing multiple remote Bakery nodes from one GUI
- Webhooks + External API:
  - Integrate with Slack/Discord/GitHub Actions
  - Trigger deployments or receive status updates
- Team Management:
  - Role-based permissions and audit logs
- Multi-instance orchestration:
  - Manage multiple hosts under one Bakery controller
- Template system:
  - Preset deployment configs for SvelteKit, Next.js, Nuxt, etc.
- Optional 2FA for accounts
- Extended analytics visualization

---

## Dependencies & Sequencing

- **Phase 0** → Required foundation for all backend and GUI work.
- **Phase 1** → Delivers core identity, GitHub link, and deploy schema.
- **Phase 2** → Enables live deployments, rollback, and CI/CD.
- **Phase 3** → Adds analytics, metrics, and system visibility.
- **Phase 4** → GUI-first usability; no CLI needed afterward.
- **Phase 5** → Stabilizes infra, install, and self-update tooling.
- **Phase 6** → Scales Bakery into a multi-node platform.

---

## ✅ Completion Criteria

The Bakery project is **complete** when:

- The `install.sh` script can bootstrap Bakery fully on a new instance.
- The GUI can manage deployments end-to-end:
  - Create, update, delete deployments
  - Schedule or rollback deployments
  - Manage domains, SSL, databases, and env vars
  - View live logs and analytics
- Bakery automatically self-updates via CI/CD.
- All processes auto-restart on crash.
- Postgres is the **only** database used.
- No manual config files (`apps.json`) — everything via GUI.

---

## Todo: 

- I integrated github, but have not removed the "link" github buttons for users that have their account linked
