# dev

Local development stack.

- `compose.yml` holds the dev Postgres 18 (`127.0.0.1:4920`, user, password
  and database `bakery`). `task db:up` starts it and waits until it accepts
  connections; `task dev` does that first. `task db:down` stops it; the data
  stays in the `bakery-dev_bakery-db` volume. It runs with `podman compose -f infra/dev/compose.yml`. Bind every
  port to `127.0.0.1` and pick it from the unit's block in the port table in
  `CLAUDE.md`. Local credentials in it are not secrets.
- `down.sh` stops every local dev server by the ports it listens on;
  `task down` runs it with `DEV_PORTS` from the root `Taskfile.yml`.
- `compose.yml` also holds **Forgejo**, the git host stand-in, under the
  `git` profile: `task git:up` starts it on `127.0.0.1:4950` (HTTP) and
  `127.0.0.1:4952` (SSH) with host networking, so its webhooks reach the dev
  API on `127.0.0.1:4910`; `task git:down` removes it and its volumes.
- `git/test.sh` (`task git:test`, with `task dev` running) creates a private
  repository in Forgejo, deploys it through Bakery with the Application's
  Deploy key, adds Bakery's Webhook to it, pushes and waits for the push to
  deploy by itself, then checks a push signed with a rotated secret deploys
  nothing. It signs in as the Owner from `BAKERY_OWNER_EMAIL` /
  `BAKERY_OWNER_PASSWORD` (or `.claude/ralph/state/owner.env`) and removes
  everything it created, Forgejo included (`KEEP_FORGEJO=1` keeps it).
- `deploy/test.sh` (`task deploy:test`, same requirements) deploys a repository
  from Forgejo with a Health check, a build-only variable and Shared
  variables; redeploys while polling it (no request may fail); checks a
  commit whose health check fails keeps the old version; cancels a slow
  build; and rolls back to the first deployment. Both scripts share
  `lib/e2e.sh`.
