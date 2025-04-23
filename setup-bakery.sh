#!/usr/bin/env bash
set -euo pipefail

# Usage:
#   DOMAIN=jevido.wtf EMAIL=you@domain.com bash ./setup-bakery.sh

# === — CONFIG — ====
DOMAIN=${DOMAIN:?“Please set DOMAIN=your.domain”}
EMAIL=${EMAIL:?“Please set EMAIL=your@email”}

APP_USER=bakery
BAKERY_ROOT=/srv/bakery

echo "🍞  [setup-bakery] Starting setup for $DOMAIN …"
sudo mkdir -p /etc/bakery
echo "DOMAIN=$DOMAIN" | sudo tee /etc/bakery/config > /dev/null
echo "EMAIL=$EMAIL" | sudo tee -a /etc/bakery/config > /dev/null

# 1️) Create bakery user
if ! id -u $APP_USER >/dev/null 2>&1; then
  sudo useradd -m -s /bin/bash $APP_USER
  echo "✔ Created user: $APP_USER"
else
  echo "ℹ  User $APP_USER already exists"
fi

# 2️) Make directory structure
sudo mkdir -p $BAKERY_ROOT/{apps,db,bin,certs,logs}
sudo chown -R $APP_USER:$APP_USER $BAKERY_ROOT

# 3️) Install system packages
sudo apt update
sudo apt install -y curl gnupg2 unzip \
     postgresql \
     ufw certbot \
     nginx python3-certbot-nginx

# 4️) Bun
if ! sudo -u $APP_USER bash -lc "command -v bun" >/dev/null; then
  sudo -u $APP_USER bash -lc "curl -fsSL https://bun.sh/install | bash"
  echo "✔ Installed Bun"
fi

# Ensure PATH is added once
BUN_PATH_LINE='export PATH="$HOME/.bun/bin:$PATH"'
if ! sudo grep -Fxq "$BUN_PATH_LINE" /home/$APP_USER/.profile; then
  echo "$BUN_PATH_LINE" | sudo tee -a /home/$APP_USER/.profile > /dev/null
fi

# 5️) Firewall
sudo ufw allow OpenSSH
sudo ufw allow https
sudo ufw allow 'Nginx Full'
sudo ufw --force enable

# 6️) PostgreSQL
sudo systemctl enable --now postgresql
echo "✔ PostgreSQL up"

# 7️) Certs folder
sudo mkdir -p /etc/letsencrypt/{live,archive}
sudo chown -R root:root /etc/letsencrypt
sudo mkdir -p /srv/bakery/certbot/.well-known/acme-challenge
sudo chown -R www-data:www-data /srv/bakery/certbot

# 8) Global Nginx fallback for Certbot HTTP-01 challenge
sudo tee /etc/nginx/snippets/letsencrypt.conf > /dev/null <<EOF
location /.well-known/acme-challenge/ {
    root /srv/bakery/certbot;
    allow all;
}
EOF
sudo ln -sf /etc/nginx/sites-available/00-default-certbot /etc/nginx/sites-enabled/00-default-certbot
sudo nginx -t && sudo systemctl reload nginx

# 9) Deploy helper scripts
echo "✔ Ensuring bakery CLI & script permissions…"
sudo chmod +x $BAKERY_ROOT/bin/*.sh
sudo chown -R $APP_USER:$APP_USER $BAKERY_ROOT/bin
sudo ln -sf $BAKERY_ROOT/bin/bakery /usr/local/bin/bakery

sudo chown -R $APP_USER:$APP_USER $BAKERY_ROOT/bin
sudo chmod +x $BAKERY_ROOT/bin/*

echo "✅  Done! You can now:"

cat <<EOF
  • ssh root@<your-server> "bakery"        # see CLI help
  • bakery install                       # same thing from bakery user
  • bakery deploy app.$DOMAIN            # deploy apps/pull certs/etc
EOF
