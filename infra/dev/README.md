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
