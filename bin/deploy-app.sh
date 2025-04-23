#!/usr/bin/env bash
set -euo pipefail

# Usage: deploy-app.sh <subdomain> [release]
SUB=$1
ROOT=/srv/bakery
APP_DIR=$ROOT/apps/$SUB
CURRENT=$APP_DIR/current
SERVICE=bakery-${SUB//./-}
EMAIL=$(grep '^EMAIL=' /etc/bakery/config | cut -d= -f2)
PORT=$(echo "$SUB" | sha256sum | tr -dc '0-9' | head -c 4) # Creates a pseudo-unique port like 3001–3999
PORT="3$PORT"

echo "🚀 Deploying $SUB…"

# 1) Ensure the release directory exists
sudo mkdir -p "$CURRENT"

if [ ! -d "$CURRENT" ]; then
  echo "❌ No files found in $CURRENT!"
  echo "   Make sure your GitHub Action rsyncs into this path."
  exit 1
fi

# 2) Grant ownership
sudo chown -R bakery:bakery "$CURRENT"

# 3) Install deps & build as bakery user
sudo -u bakery bash -lc "
  cd \"$CURRENT\"
  bun install
  bun --bun run build
"

# 4) Write (or overwrite) systemd service
cat <<EOF | sudo tee /etc/systemd/system/$SERVICE.service > /dev/null
[Unit]
Description=Bakery app $SUB
After=network.target

[Service]
User=bakery
WorkingDirectory=$CURRENT
ExecStart=/home/bakery/.bun/bin/bun .output/server/index.js --port $PORT
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# 5) Write Nginx config
NGINX_CONF="/etc/nginx/sites-available/$SUB"
cat <<EOF | sudo tee "$NGINX_CONF" > /dev/null
server {
    listen 80;
    server_name $SUB;

    location / {
        proxy_pass http://localhost:$PORT;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host \$host;
        proxy_cache_bypass \$http_upgrade;
    }
}
EOF

# 6) Enable site + reload Nginx
sudo ln -sf "$NGINX_CONF" /etc/nginx/sites-enabled/$SUB
sudo nginx -t && sudo systemctl reload nginx

# 7) Obtain TLS certificate using Nginx plugin
sudo certbot --nginx --non-interactive --agree-tos --email "$EMAIL" -d "$SUB"

# 8) Reload & restart app
sudo systemctl daemon-reload
sudo systemctl enable --now "$SERVICE"

echo "✅ Deployed $SUB → service: $SERVICE on port $PORT"
