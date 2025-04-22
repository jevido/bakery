#!/usr/bin/env bash
set -euo pipefail

# Usage:
#   DOMAIN=jevido.wtf EMAIL=you@domain.com ./setup-bakery.sh

# === — CONFIG — ====
DOMAIN=${DOMAIN:?“Please set DOMAIN=your.domain”}
EMAIL=${EMAIL:?“Please set EMAIL=your@email”}

APP_USER=bakery
BAKERY_ROOT=/srv/bakery

echo "🍞  [setup-bakery] Starting setup for $DOMAIN …"
sudo mkdir -p /etc/bakery
echo "DOMAIN=$DOMAIN" | sudo tee /etc/bakery/config > /dev/null
echo "EMAIL=$EMAIL" | sudo tee -a /etc/bakery/config > /dev/null

# 1️⃣ Create bakery user
if ! id -u $APP_USER >/dev/null 2>&1; then
  sudo useradd -m -s /bin/bash $APP_USER
  echo "✔ Created user: $APP_USER"
else
  echo "ℹ  User $APP_USER already exists"
fi

# 2️⃣ Make directory structure
sudo mkdir -p $BAKERY_ROOT/{apps,db,bin,certs,logs}
sudo chown -R $APP_USER:$APP_USER $BAKERY_ROOT

# 3️⃣ Install system packages
sudo apt update
sudo apt install -y curl gnupg2 unzip \
     postgresql \
     ufw certbot

# 4️⃣ Bun
if ! command -v bun >/dev/null; then
  curl -fsSL https://bun.sh/install | bash
  echo "export PATH=\"\$HOME/.bun/bin:\$PATH\"" \
    | sudo tee -a /home/$APP_USER/.bashrc
  echo "✔ Installed Bun"
else
  echo "ℹ  Bun already installed"
fi

# 5️⃣ Firewall
sudo ufw allow OpenSSH
sudo ufw allow https
sudo ufw --force enable

# 6️⃣ PostgreSQL
sudo systemctl enable --now postgresql
echo "✔ PostgreSQL up"

# 7️⃣ Certs folder
sudo mkdir -p /etc/letsencrypt/{live,archive}
sudo chown -R root:root /etc/letsencrypt

# 8️⃣ Deploy helper scripts
echo "✔ Copying bakery CLI & scripts…"
sudo install -m755 bin/bakery /usr/local/bin/bakery
sudo install -m755 bin/deploy-app.sh bin/remove-app.sh bin/upgrade-bakery.sh \
     $BAKERY_ROOT/bin/
sudo chown -R $APP_USER:$APP_USER $BAKERY_ROOT/bin
sudo chmod +x $BAKERY_ROOT/bin/*.sh

ls -l "$BAKERY_ROOT/bin"
echo "✅  Done! You can now:"

cat <<EOF
  • ssh root@<your-server> "bakery"        # see CLI help
  • bakery install                       # same thing from bakery user
  • bakery deploy app.$DOMAIN            # deploy apps/pull certs/etc
EOF
