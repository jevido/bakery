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
- `compose.yml` also holds **Garage**, the S3 stand-in, under the `s3`
  profile (`garage/garage.toml`, dev-only secrets): `task s3:up` starts it on
  `127.0.0.1:4960` and runs `garage/setup.sh`, which gives the node a
  layout and creates the bucket `bakery-backups` and a key for it, written to
  `.claude/ralph/state/garage.env`. `task s3:down` removes it, its volumes
  and that file.
- `compose.yml` also holds the **Remote server stand-in** under the `remote`
  profile (`remote/Containerfile`): sshd and rootless Podman for the user
  `podman` (uid 1000), SSH on `127.0.0.1:4972`, no passwords; unprivileged
  ports start at 80 inside it, so a Remote Proxy can bind 80/443, published
  on `127.0.0.1:4974` (HTTP) and `4975` (HTTPS). `task
  remote:up` builds and starts it, `task remote:down` removes it. Every start
  is a fresh server: new host keys, an empty `authorized_keys` (add a key
  with `podman exec -i bakery-dev-remote-1 sh -c 'cat >>
  /home/podman/.ssh/authorized_keys'`). There is no systemd inside, so the
  entrypoint starts the Podman API socket and a `loginctl` shim reports
  linger, which `touch /etc/bakery-stand-in/no-linger` turns off.
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
  build; and rolls back to the first deployment.
- `buildpack/test.sh` (`task buildpack:test`) deploys one Application per
  Build pack, including a private image from Forgejo's registry.
- `settings/test.sh` (`task settings:test`) checks Application settings:
  two Domains, removing one without a deploy, the Www redirect, a Response
  header, Basic auth, a file in Persistent storage surviving a redeploy and
  a rollback, Resource limits on the Container, and the volume going with
  the Application.
- `databases/test.sh` (`task databases:test`, needs `task dev` but not
  Forgejo) creates a Database of every Engine and queries each on its
  Internal URL from a container on the `bakery` network, keeps a Postgres
  row across stop/start and a Public port change, reaches it on the Public
  URL from the host, checks a Project with Databases cannot be deleted, and
  deletes everything with its volumes. `RESTART_API=1` also removes one
  Database's Container, restarts `task dev` detached and waits for the API
  to start it again.
- `backups/test.sh` (`task backups:test`, needs `task dev` and `task s3:up`)
  backs up a row of a PostgreSQL, MySQL, MariaDB and MongoDB Database to
  local disk and Garage, drops it and restores it; restores from Garage
  after removing the local file; checks Retention 2 leaves two files and two
  objects; waits for an every-minute schedule to back up by itself; checks
  Redis refuses; and deletes the Databases, whose local Backups go while
  the S3 copies stay.
- `services/test.sh` (`task services:test`, needs `task dev`) creates a
  Service from the whoami template and one from a compose file (a whoami
  `web` and a Postgres `db`), checks both answer through the Proxy, that
  `db` resolves by name on the Service network and keeps its row and
  generated password across Redeploy, that Stop takes the Domain away and
  Start brings it back, that `build:` is refused with its line, that
  Applications and Services cannot take each other's Domains and a Project
  with a Service cannot be deleted, and that deleting the Services leaves
  no container, network or volume.
- `servers/test.sh` (`task servers:test`, needs `task dev`) checks the Local
  server is reachable with metrics and cannot be deleted, starts the Remote
  server stand-in, adds it as a Server, authorises its Server key and
  validates it, sees a Bakery container running there in its metrics, cleans
  up both Servers without touching an unlabelled image, replaces the
  stand-in's host keys and checks the Server is refused until the host key
  is forgotten, and removes it. Every script shares `lib/e2e.sh`.
