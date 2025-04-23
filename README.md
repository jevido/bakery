# Bakery CI/CD Server Setup

This README explains how to bootstrap your Hetzner server with the **Bakery** CI/CD platform and how to use the `bakery` CLI to deploy and manage your Bun-powered apps.

---

## 📋 Prerequisites

- A **Hetzner VPS** running **Ubuntu 22.04**
- **Root** or **sudo** privileges on the server
- A **domain** (e.g. `jevido.wtf`) with its A-record pointing to your server’s IP
- An **email address** for Let’s Encrypt notifications

---

## 🚀 Bootstrap the CI/CD Platform

1. **Clone the CI/CD repo** into `/srv/bakery/bakery`:

   ```bash
   sudo git clone git@github.com:<you>/bakery.git /srv/bakery/bakery
   cd /srv/bakery/bakery
   ```

2. **Run the setup script**, passing your domain and email:

   ```bash
   sudo DOMAIN=jevido.wtf \
        EMAIL=you@your-email.com \
        bash setup-bakery.sh
   ```

   This will:

   - Create a `bakery` system user
   - Install **Bun**, **PostgreSQL 15**, **UFW**, **Certbot**
   - Open ports for SSH & HTTPS in UFW
   - Create the directory structure under `/srv/bakery`
   - Install the `bakery` CLI and helper scripts

3. **Add the start script** to your package.json
   ```json
		"start": "bun --bun run build/index.js"
   ```

---

## 📂 Directory Structure

After setup, you will have:

```
/srv/bakery/
├── apps/            # Application releases & current symlinks
├── bin/             # bakery CLI and helper scripts
├── certs/           # Let’s Encrypt certificates
├── db/              # (Optional) Database backups or scripts
├── logs/            # (Optional) App or system logs
└── bakery/           # The bakery repo (setup scripts live here)
```

The `bakery` command is symlinked to `/usr/local/bin/bakery`.

---

## 🛠️ Using the `bakery` CLI

Run **as any user** (it uses `sudo` internally) to manage your platform:

```bash
# Show help
bakery

# Deploy or update an app:
# bakery deploy <subdomain> [release]
bakery deploy app.jevido.wtf

# Remove an app and its certificates:
bakery remove app.jevido.wtf

# Upgrade Bakery platform (pull latest & re-run setup):
bakery upgrade
```

- **deploy**: builds, obtains TLS cert, and starts your Bun app as a systemd service
- **remove**: stops and disables the service, deletes code and certs
- **upgrade**: fetches new bakery scripts and re-runs `setup-bakery.sh`

---

## 🎯 Deploying from GitHub Actions

In each app’s GitHub repo, create `.github/workflows/deploy.yml`:

```yaml
name: Deploy to Bakery
on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Copy code to server
        uses: appleboy/scp-action@v0.1.4
        with:
          host: ${{ secrets.HETZNER_HOST }}
          username: bakery
          key: ${{ secrets.HETZNER_SSH_KEY }}
          source: "./"
          target: "/srv/bakery/apps/${{ inputs.subdomain }}/releases/${{ github.sha }}"
          strip_components: 1

      - name: Trigger Bakery deploy
        uses: appleboy/ssh-action@v1.0.0
        with:
          host: ${{ secrets.HETZNER_HOST }}
          username: bakery
          key: ${{ secrets.HETZNER_SSH_KEY }}
          script: /srv/bakery/bin/deploy-app.sh ${{ inputs.subdomain }} ${{ github.sha }}
```

> **Note:** Set `inputs.subdomain` to your full app hostname (e.g. `app.jevido.wtf`).

### Setting GitHub Secrets

In your app repo’s Settings ▶️ Secrets:

- **HETZNER_HOST**: your server’s IP or hostname
- **HETZNER_SSH_KEY**: the private SSH key for `bakery` user

---

## 🔄 Upgrading Bakery

When you add features to the bakery repo:

1. SSH into server:
   ```bash
   ssh root@your.server.ip
   ```
2. Pull latest and upgrade:
   ```bash
   cd /srv/bakery/bakery
   git pull
   bakery upgrade
   ```

---

## 📜 License

GLWTPL © Jeffrey Duivenvoorden
