# install

`install.sh` turns a fresh Linux server (Fedora/RHEL or Debian/Ubuntu, with
systemd) into a running Bakery, and upgrades it when run again.

```sh
sudo bash install.sh --domain bakery.example.com --email me@example.com
```

Point the domain's DNS at the server and open ports 80 and 443 first: the
proxy gets its certificate over ACME (HTTP-01). A Database given a Public
port in the dashboard is published on that port on every interface; Bakery
does not manage a firewall, so open (or keep closed) such ports yourself.

## What it does

1. Installs Podman when it is missing (`dnf` or `apt-get`).
2. Sets `net.ipv4.ip_unprivileged_port_start=80`
   (`/etc/sysctl.d/90-bakery.conf`) so rootless containers can bind 80/443.
3. Creates the system user `bakery` (home `/var/lib/bakery`) with subuid and
   subgid ranges, and enables linger so its containers run without a login.
   If that user's systemd does not get the `cpu` and `memory` cgroup
   controllers (older systemd), writes
   `/etc/systemd/system/user@.service.d/90-bakery-delegate.conf` so an
   Application's resource limits work.
4. Enables `podman.socket` and `podman-restart.service` for that user; the
   latter starts every `--restart=always` container at boot.
5. Generates `APP_KEY`, `JWT_SECRET` and the database password once into
   `/var/lib/bakery/bakery.env` (mode 600).
6. As `bakery`: creates the `bakery` network and runs `bakery-postgres`
   (volume `bakery-postgres`), `bakery-web` and `bakery-api`. The API
   creates `bakery-proxy` on 80/443 itself and serves the dashboard on
   `--domain` through it. Nothing but the proxy, and Databases the Owner
   gives a Public port, publishes a host port.
7. Waits until `https://<domain>/api/health` answers.

## Options

| Flag | Meaning |
| ---- | ------- |
| `--domain` | Dashboard domain; applications default to `<name>.<domain>`. Required. |
| `--email` | ACME account email. Required unless `--acme-ca` is given. |
| `--api-image`, `--web-image` | Images to run (default `ghcr.io/jevido/bakery-{api,web}:latest`). |
| `--acme-ca` | ACME directory URL (default Let's Encrypt). |
| `--acme-ca-root` | PEM file the ACME directory's own certificate is signed by (a private CA). |
| `--load` | `podman load` images from this archive before starting. |
| `--no-pull` | Use images already present. |

## Running it again

Re-running keeps `bakery.env`, the database and the certificates. It pulls
the images (unless `--no-pull`) and recreates `bakery-api`, `bakery-web` and
`bakery-proxy`; applications keep running, and are unreachable only for the
seconds the proxy takes to come back.

Tested end to end in a fake server container with `task server:test` (see
`infra/dev/server/`).
