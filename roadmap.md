# Project Roadmap

## Phase 0 · Foundations

Lock repo structure: app/ for SvelteKit GUI, backend/ for Bun services, infrastructure/ for install assets.
Define environment scaffolding: config loading, secrets handling, runtime env detection.
Stand up Bun service shell with PostgreSQL connection pool, migration framework, and base auth tables.
Bootstrap SvelteKit app with global layout, routing skeleton, shared UI kit import, and auth guard.

## Phase 1 · Core Platform Services

Implement user auth flows (bcrypt passwords, session management, initial admin bootstrap).
Add GitHub OAuth integration: store encrypted tokens, repo/branch listing, webhook provisioning.
Build deployment domain models (apps, builds, environments, slots) and CRUD APIs.
Create job runner/orchestrator to enqueue build/deploy tasks and emit live status events.
Introduce systemd/Docker command adapters, Nginx/Certbot template engine, and audit logging.

## Phase 2 · Deployment Engine

Implement build pipelines (Docker + Bun-native) with artifact storage, port management, slot assignment.
Wire blue-green workflow: health checks, switch-over, rollback history (max 5 versions).
Add environment variable vault, template rendering into runtime env files, masked logs.
Integrate crash monitor with auto-restart (systemd unit generation + watcher service).

## Phase 3 · Observability & Analytics

Collect Nginx/system metrics, parse logs, aggregate into daily summaries, store in analytics tables.
Build disk usage tracker per deployment and Postgres DB size polling.
Surface live logs, historical charts, health statuses over SSE/WebSockets to frontend.

## Phase 4 · GUI Experience

Dashboards: global metrics, activity feed, quick actions.
Sidebar deployment explorer with status badges.
Deployment detail view: tabs for overview, domains, env vars, builds, logs, analytics.
Wizards/modals for new deployment, domain attach/verify, env var editing, database provisioning.
Database manager screens (list DBs, credentials reveal, migration trigger/status).

## Phase 5 · Infrastructure Tooling

Author install.sh (dependency install, repo clone, DB init, Nginx baseline, systemd setup, seed admin).
Provide systemd unit files, Nginx & Certbot templates, and logrotate configs.
Ship Docker Compose dev environment (Bun app, Postgres, Nginx stub).
Implement self-update workflow (git pull + migrations + rolling restart) triggered from GUI.
Dependencies & Prioritization

Phase 0 is prerequisite; it establishes shared frameworks and repo layout.
Phase 1 delivers core identity + deployment schema required before any UI interaction.
Phase 2 depends on Phase 1 data models and job runner; enables real deployments.
Phase 3 leverages running deployments to gather metrics; feeds Phase 4 UI.
Phase 4 consumes APIs from prior phases; unlocks full GUI-only management.
Phase 5 finalizes install & ops tooling once platform behaviors are stable.
Immediate Next Actions

Draft technical spec docs (DB schema ERD, service boundaries, API contracts).
Set up Bun + SvelteKit project boilerplates and configure shared lint/format/test tooling.
Prepare migration framework (e.g., drizzle-kit equivalent for bun.sql or custom runner).
Define secrets management approach (dotenv for dev, env vars/secure storage for prod).
Create initial backlog tickets per phase to track granular tasks.
Let me know when you’re ready to dive into Phase 0 setup or if you want any part of the plan adjusted.