#!/usr/bin/env bash
set -euo pipefail

# deploy-app.sh <subdomain> [release]
SUB=$1
REL=${2:-$(date +%s)}
ROOT=/srv/bakery
APP_DIR=$ROOT/apps/$SUB
RELEASE_DIR=$APP_DIR/releases/$REL
CURRENT=$APP_DIR/current
SERVICE=bakery-${SUB//./-}
EMAIL=$(grep '^EMAIL=' /etc/bakery/config | cut -d= -f2)

# 1) Create dirs & move code
sudo mkdir -p $RELEASE_DIR
sudo mv /tmp/deploy_payload/* $RELEASE_DIR
sudo chown -R bakery:bakery $RELEASE_DIR

# 2) Build & deps as bakery user
sudo -u bakery bash -lc "
  cd $RELEASE_DIR
  bun install
  bun run build
"

# 3) Symlink
sudo ln -sfn $RELEASE_DIR $CURRENT

# 4) Certbot (standalone)
if [ ! -d /etc/letsencrypt/live/$SUB ]; then
  sudo certbot certonly \
    --standalone -d $SUB \
    --non-interactive \
    --agree-tos \
    --email $EMAIL
fi

# 5) systemd unit
cat <<EOF | sudo tee /etc/systemd/system/$SERVICE.service
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

sudo systemctl daemon-reload
sudo systemctl enable --now $SERVICE

echo "🚀 Deployed $SUB → running under service $SERVICE"
