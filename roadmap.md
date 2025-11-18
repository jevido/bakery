# 🍞 Bakery — Refocused Roadmap (SSH-Orchestrated Control Plane)

Bakery is a self-hosted deployment manager: SvelteKit UI, Bun backend, PostgreSQL, and Nginx/Certbot for ingress.  
**New direction:** the control plane now drives every deployment by opening SSH sessions directly to `bakery-agent` users on remote nodes. No remote HTTP agents, no polling queue — all orchestration, logging, and updates originate from the control plane.

---

## Phase 0 · Foundation & Configuration

**Goals**
- Keep project layout (`app/`, `backend/`, `infrastructure/`, `scripts/`) and environment loading stable.
- Maintain Bun + PostgreSQL stack, migration tooling, and auth scaffolding.
- Continue using SvelteKit (no TS) + shared UI kit.

---

## Phase 1 · Accounts, GitHub, Nodes (read/verify only)

**Deliverables**
- Password auth + session cookies; admin bootstrap.
- GitHub OAuth, repo + branch selection, webhook registration.
- Node CRUD page that mints SSH keys, stores host/port/user, and verifies reachability via `ssh bakery-agent@host echo bakery-ready`.
- Installer script (`install-node-agent.sh`) that:
  - Creates `bakery-agent` user
  - Installs Docker, Docker CLI, Postgres client/server, git, curl, build-essential
  - Adds Bakery’s public key to `authorized_keys`
  - Grants sudoers entries needed for deployments (systemctl, docker, postgres utilities)

**No background agent yet** — nodes are just SSH targets with required tooling.

---

## Phase 2 · SSH-Driven Deployment Engine

**Key refactor**
- Remove “agent/task queue” architecture.
- Implement a control-plane executor that:
  1. Opens SSH session(s) to the node when a deployment starts.
  2. Runs git clone, Bun install, tests, docker build/run, systemd management **over SSH**.
  3. Streams stdout/stderr back to the control plane (store in `deployment_logs` and pipe to UI).
  4. Writes metadata (active slot, dockerized flag, ports) after commands succeed.

**Components**
- SSH job runner with cancellation/timeouts, environment injection, log tap.
- Deployment orchestrator (formerly agent workflows) rewritten to use the SSH runner.
- Systemd/Nginx generation now triggered from the control plane (no remote API calls).
- Status tracking + retries handled entirely by control-plane orchestrator.

---

## Phase 3 · UI + API Adjustments

**Goals**
- `/api/deployments/:id` should reflect SSH-driven executions: include log offsets, running states, active slots.
- Live Task Log panel must tail the new log stream (no HTTP-agent posts); implement SSE or poll-for-new-rows logic keyed by log ID/timestamp.
- Nodes UI updates:
  - Remove agent token/pairing concepts.
  - Provide two copy buttons per node: “Bootstrap (curl install-node-agent.sh …)” and “Restart agent service” (if we still ship a helper service; optional now).
  - Show last SSH handshake, last successful deployment, disk usage snapshot.
- Deployment detail view includes “Deploy now”, “Redeploy last commit”, “Restart systemd service” buttons that trigger SSH workflows.

---

## Phase 4 · Logging, Analytics, and Monitoring (Over SSH)

**Tasks**
- Tail runtime logs via SSH (e.g., `journalctl -u bakery-deployment-… -f`) or read from files written during deployment.
- Continue collecting disk/traffic analytics via control-plane sidecar processes over SSH (or local stats if running on same host).
- Alerting when SSH commands fail, docker build errors, or systemd status is unhealthy.

---

## Phase 5 · Infrastructure Tooling Cleanup

**Objectives**
- Simplify `/api/agent/*` endpoints (remove or repurpose for SSH metadata).
- Clean DB schema: drop unused columns (`api_token`, `pairing_code`, etc.).
- Update docs + README to reflect the new SSH-only deployment model.
- Ensure installer scripts (control-plane + node) cover:
  - Docker CLI availability
  - Postgres CLI
  - Log directories + permissions
  - Systemd units for Bakery itself

---

## Phase 6 · Advanced Enhancements (Post-SSH Migration)

- Multi-node orchestration via SSH to multiple hosts.
- Parallel deployments with connection pooling.
- Pluggable “execution adapters” (e.g., run over WireGuard, Bastion host, etc.).
- Notification hooks (Slack, Discord) triggered after SSH runs complete.

---

## Transition Notes

1. **Remove legacy agent artifacts**
   - Delete `/api/agent/*`, agent install scripts, and task queue code.
   - Clean up environment variables (`BAKERY_AGENT_API`, etc.).

2. **Introduce SSH execution layer**
   - Central utility for running commands, streaming logs, copying files, managing env.
   - Support for long-running commands with heartbeat/ping to UI.

3. **Refactor deployment workflow**
   - Map former `agent/workflows.js` steps (clone → build → deploy → configure ingress) to the new SSH utility.
   - Replace `recordDeploymentVersionRemote` / `updateDeploymentStatus` calls with direct DB writes from the control plane.

4. **UI changes**
   - Node detail cards show SSH metadata and copyable commands.
   - Deployment pages update via SSE/poll to reflect new logs and statuses.

5. **Testing/Validation**
   - Local mode (dev) runs commands against localhost; production mode targets remote nodes over SSH.
   - Integration tests for end-to-end SSH deploy, rollback, restart.

This roadmap replaces the previous agent-based plan and sets the milestones needed to operate purely via control-plane initiated SSH sessions.
