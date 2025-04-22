#!/usr/bin/env bash
set -euo pipefail

# Usage: deploy-app.sh <subdomain> [release]
SUB=$1
ROOT=/srv/bakery
APP_DIR=$ROOT/apps/$SUB
CURRENT=$APP_DIR/current
SERVICE=bakery-${SUB//./-}
EMAIL=$(grep '^EMAIL=' /etc/bakery/config | cut -d= -f2)

echo "🚀 Deploying $SUB (release $REL)…"

# 1) Ensure the release directory exists (code must already be there)
sudo mkdir -p $CURRENT

if [ ! -d $CURRENT ]; then
  echo "❌ No files found in $CURRENT!"
  echo "   Make sure your GitHub Action rsyncs into this path."
  exit 1
fi

# 2) Grant ownership
sudo chown -R bakery:bakery $CURRENT

# 3) Install deps & build as bakery user
sudo -u bakery bash -lc "
  cd $CURRENT
  bun install
  bun --bun run build
"


# 4) Obtain/renew TLS cert if needed
if [ ! -d /etc/letsencrypt/live/$SUB ]; then
  sudo certbot certonly \
    --standalone -d $SUB \
    --non-interactive --agree-tos \
    --email $EMAIL
fi

# 5) Write (or overwrite) systemd service
cat <<EOF | sudo tee /etc/systemd/system/$SERVICE.service > /dev/null
[Unit]
Description=Bakery app $SUB
After=network.target

[Service]
User=bakery
WorkingDirectory=$CURRENT
ExecStart=/home/bakery/.bun/bin/bun .output/server/index.js \
  --tls-cert /etc/letsencrypt/live/$SUB/fullchain.pem \
  --tls-key  /etc/letsencrypt/live/$SUB/privkey.pem
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# 6) Reload & restart
sudo systemctl daemon-reload
sudo systemctl enable --now $SERVICE

echo "✅ Deployed $SUB → service: $SERVICE"
