# fake server

A local stand-in for a fresh server, so `infra/install/install.sh` can be
tested for real without a machine or a real ACME CA.

- `bakery-test-server`: Fedora with systemd as PID 1 (privileged,
  `--systemd=always`). `install.sh` installs Podman, creates the `bakery`
  user and runs Bakery inside it, nested rootless Podman included.
- `bakery-test-pebble`: [Pebble](https://github.com/letsencrypt/pebble) as
  ACME CA on `https://pebble:14000/dir`, with every challenge valid, so no
  DNS stand-in is needed. The dashboard domain is `bakery.test`.

```sh
task images          # build localhost/bakery-{api,web}:dev
task server:up       # start both
task server:install  # copy the images and install.sh in, run it
task server:down     # remove everything bakery-test-server made
```

Quirks of the stand-in, not of real servers: `newuidmap`/`newgidmap` are
setuid in the image (file capabilities do not survive in a rootless
container), `/var/lib/bakery` is a volume (nested overlay storage cannot sit
on the container's overlay root), and `install.sh` gets `--subids
1000-65536` because the container has only 65536 IDs to hand out.

To look around: `./infra/dev/server/server.sh exec bash`, then
`runuser -u bakery -- sh -c 'cd; XDG_RUNTIME_DIR=/run/user/$(id -u) podman ps'`.
Certificates are signed by Pebble's issuing root at
`https://pebble:15000/roots/0` (itself served under
`pebble.minica.pem`, copied to `/root/pebble-minica.pem`).
