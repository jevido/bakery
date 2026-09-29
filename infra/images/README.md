# images

Build files, one directory per deployed resource:

```
infra/images/<resource>/Containerfile
infra/images/<resource>/Containerfile.dockerignore
infra/images/shared/            # scripts used by more than one image
```

Images build from the repository root as context, so a Containerfile can copy
shared files. Reproduce a build locally with
`podman build -f infra/images/<resource>/Containerfile .`.

The same image runs in `next` and `prod`. Never bake environment-specific
values (URLs, credentials, feature flags) into an image; read them from the
environment at runtime.

## Images

| Image | Containerfile | Build |
| ----- | ------------- | ----- |
| `bakery-api` | [`api/Containerfile`](api/Containerfile) | `task images:api` |
| `bakery-web` | [`web/Containerfile`](web/Containerfile) | `task images:web` |

`task images` builds all of them as `localhost/bakery-<name>:dev`.

### bakery-api

The Go API on port 4910; migrates the database before it serves. It drives
Podman, so it needs the host user's rootless Podman socket mounted at
`/run/podman/podman.sock` (with `--security-opt label=disable` on SELinux
hosts). Root in the container is that host user under rootless Podman.

Baked in (the same everywhere): `APP_ENV=production`, `APP_HOST=0.0.0.0`,
`APP_PORT`, `DB_CONNECTION=postgres`, `MIGRATE_ON_START=true`,
`LOG_PRINT=true` (logs go to `podman logs`), `BAKERY_PODMAN_SOCKET`.

From the environment at run time: `APP_KEY` (32 characters), `JWT_SECRET`,
`DB_HOST`, `DB_PORT`, `DB_DATABASE`, `DB_USERNAME`, `DB_PASSWORD`, and the
`BAKERY_*` settings in `services/api/.env.example`.

### bakery-web

The built dashboard served by Caddy on port 80 from a baked JSON config
([`web/caddy.json`](web/caddy.json); admin API off). `index.html` is
`no-cache`, hashed `/assets/*` are cached for a year. The dashboard uses hash
routing, so there is no SPA fallback, and it has no runtime configuration:
it calls `/api` on its own origin, which the proxy's Dashboard Route sends to
`bakery-api`.
