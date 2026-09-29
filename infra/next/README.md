# next

Pre-production: a second server set up exactly like [`prod`](../prod/README.md),
with the same `install.sh`, running the same images before they go to prod.
Only its dashboard domain and image tags differ.

## Install and upgrade

```sh
sudo bash install.sh --domain next.bakery.example.com --email me@example.com \
  --api-image <registry>/bakery-api:<tag> --web-image <registry>/bakery-web:<tag>
```

Promote a tag to prod only after it has run here.

## Resources, configuration, operating it, backups

As in [`infra/prod/README.md`](../prod/README.md). Nothing differs but the
domain passed to `--domain`.
