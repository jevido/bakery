# dev

Local development stack.

- `compose.yml` (add when a unit needs a backing service such as a database)
  runs with `podman compose -f infra/dev/compose.yml`. Bind every port to
  `127.0.0.1` and pick it from the unit's block in the port table in
  `CLAUDE.md`. Local credentials in it are not secrets.
- `down.sh` stops every local dev server by the ports it listens on;
  `task down` runs it with `DEV_PORTS` from the root `Taskfile.yml`.
