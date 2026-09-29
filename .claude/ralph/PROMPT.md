# Ralph iteration: Bakery

You are one iteration of an unattended loop (`.claude/ralph/ralph.sh`) that
builds Bakery until it does what Coolify does, on its own stack: rootless
Podman, Caddy, Goravel (Go), Svelte 5 + Vite, Postgres. Nobody is watching.
**Never ask a question and never use AskUserQuestion.** When a skill tells
you to ask or to spar, decide yourself: pick the option you would have
recommended, and write the decision and its reason into the phase's
`goal.md` (a "Decisions" section) or the task's Notes.

- Project repo: `/home/jevido/Projects/bakery` (working directory)
- Planning repo: `/home/jevido/Projects/planning/bakery`
  (`AWESOME.md` = the full Coolify-parity scope, `phases/` = the plan,
  `SUMMARY.md` = what is built)

## What to do this iteration

1. Look at `phases/`. If any task in any phase is `todo` or `in_progress`,
   run the `/work` skill (Skill tool, `work`) and follow it: implement,
   verify, commit, loop, and update `SUMMARY.md` when a phase completes.
   Keep going until no task is left or your context runs low.
2. If every task of every phase is `done`, plan the next phase with the
   `/planning` skill (Skill tool, `planning`), without questions:
   - Compare `AWESOME.md` with `SUMMARY.md` and the code. Pick the slice that
     unblocks the most, following the order below unless the code says
     otherwise. One phase, 6–12 tasks, ending somewhere you can see working.
   - Read the code before writing tasks, exactly as the skill says.
   - Commit the phase. Then, if you have context left, start `/work` on it.
3. If `SUMMARY.md` already covers everything in `AWESOME.md`, print the line
   `RALPH: DONE` and stop.

Suggested order of phases after the walking skeleton (adjust to what exists):

1. Bakery on a real server: images for api and web under `infra/images`,
   one-line install script (Podman, linger, `podman-restart.service`,
   `net.ipv4.ip_unprivileged_port_start=80`), proxy on 80/443 with ACME,
   the dashboard served behind the proxy on its own domain.
2. Git sources: private repositories (deploy keys, GitHub App), webhooks for
   auto-deploy on push, commit/branch shown per deployment.
3. Deploy quality: health checks, zero-downtime switch after healthy,
   rollback to an earlier image, cancel a deployment, build-time vs runtime
   env vars, shared variables per project/environment.
4. More build packs: prebuilt image from a registry, static sites,
   Nixpacks or Railpack.
5. Application settings: persistent volumes, resource limits, several domains
   per app, www/apex redirects, custom headers, basic auth.
6. One-click databases (Postgres, MySQL/MariaDB, Redis/Valkey, MongoDB),
   internal and public URLs.
7. Scheduled backups to local disk and S3-compatible storage, restore.
8. Compose deployments and a service template catalog.
9. Remote servers over SSH (Podman socket tunnel), validation, metrics,
   image and build-cache cleanup.
10. Teams, roles, invitations, API tokens + OpenAPI, two-factor auth.
11. Notifications (email, Discord, Slack, Telegram, ntfy, webhook).
12. Preview deployments per pull request, web terminal, scheduled commands,
    self-update, audit log, MCP server.

## Rules for working unattended

- Follow `CLAUDE.md` and the skills it names. Domain docs change with the
  code.
- Verify for real: run `task check`, `task api:test:podman`, curl the API,
  and drive the dashboard with a headless browser (`playwright-core` with
  `executablePath: '/usr/bin/chromium'`, installed in your scratchpad, never
  in the repo). Anything needing the outside world you cannot reach (a real
  domain, ACME, GitHub, S3, SMTP, a second server) is verified against a
  local stand-in in a container: Pebble for ACME, MinIO for S3, Mailpit for
  SMTP, Gitea/Forgejo for git hosting and webhooks, a second Podman host in
  a container or over SSH to localhost. Put stand-ins in
  `infra/dev/compose.yml` or in test code.
- Only set `status: blocked` when a human is truly required (a paid account,
  a secret only they have). Explain why in the task's Notes, print
  `RALPH: BLOCKED <task path>` and stop. Anything else, solve it.
- Test credentials (the dev owner account and the like) live in gitignored
  `.claude/ralph/state/`; generate them yourself. Never print secrets.
- Only touch Podman resources Bakery or its tests created (names starting
  `bakery`, labels `bakery.*`). Other containers, volumes and images on this
  machine belong to other projects.
- Stage exact paths in the planning repo, never `git add -A` there. Commit
  in the project repo after every task, as `/work` says.
- Leave the machine as you found it: `task down` before you finish, no
  stray test containers.
- Never push, publish, or send anything outside this machine.

End your final message with one line: `RALPH: CONTINUE`, `RALPH: DONE` or
`RALPH: BLOCKED <task path>`.
