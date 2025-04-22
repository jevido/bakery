#!/usr/bin/env bash
set -euo pipefail

# Usage: deploy-app.sh <subdomain> [release]
SUB=$1
REL=${2:-$(date +%s)}
ROOT=/srv/bakery
APP_DIR=$ROOT/apps/$SUB
RELEASE_DIR=$APP_DIR/releases/$REL
CURRENT=$APP_DIR/current
SERVICE=bakery-${SUB//./-}
EMAIL=$(grep '^EMAIL=' /etc/bakery/config | cut -d= -f2)

echo "🚀 Deploying $SUB (release $REL)…"

# 1) Ensure the release directory exists (code must already be there)
sudo mkdir -p $RELEASE_DIR

if [ -z "$(ls -A $RELEASE_DIR)" ]; then
  echo "❌ No files found in $RELEASE_DIR!"
  echo "   Make sure your GitHub Action rsyncs into this path."
  exit 1
fi

# 2) Grant ownership
sudo chown -R bakery:bakery $RELEASE_DIR

# 3) Install deps & build as bakery user
sudo -u bakery bash -lc "
  cd $RELEASE_DIR
  bun install
  bun run build
"

# 4) Point “current” at the new release
sudo ln -sfn $RELEASE_DIR $CURRENT

# 5) Obtain/renew TLS cert if needed
if [ ! -d /etc/letsencrypt/live/$SUB ]; then
  sudo certbot certonly \
    --standalone -d $SUB \
    --non-interactive --agree-tos \
    --email $EMAIL
fi

# 6) Write (or overwrite) systemd service
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

# 7) Reload & restart
sudo systemctl daemon-reload
sudo systemctl enable --now $SERVICE

echo "✅ Deployed $SUB → service: $SERVICE"
