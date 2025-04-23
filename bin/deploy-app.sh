#!/usr/bin/env bash
set -euo pipefail

# Usage: deploy-app.sh <subdomain>
SUB=$1
ROOT=/srv/bakery
APP_DIR=$ROOT/apps/$SUB
CURRENT=$APP_DIR/current
SERVICE=bakery-${SUB//./-}
EMAIL=$(grep '^EMAIL=' /etc/bakery/config | cut -d= -f2)
PORT_FILE=$APP_DIR/port
PORT=3000
NGINX_CONF="/etc/nginx/sites-available/$SUB"
NGINX_LINK="/etc/nginx/sites-enabled/$SUB"

echo "🚀 Deploying $SUB on port $PORT…"

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
  cd $CURRENT
  bun install
  bun --bun run build
"
sudo ln -sf "$NGINX_CONF" "$NGINX_LINK"
sudo nginx -t && sudo systemctl reload nginx

# 5) Request TLS cert (standalone mode, stop nginx first)
if [ ! -d "/etc/letsencrypt/live/$SUB" ]; then
  echo "🌐 Requesting TLS cert for $SUB…"
  sudo systemctl stop nginx

  sudo certbot certonly --standalone -d "$SUB" \
    --non-interactive --agree-tos --email "$EMAIL" || {
      echo "❌ Certbot failed. Aborting."
      exit 1
  }

  sudo systemctl start nginx
fi

# 6) Update Nginx with reverse proxy config
sudo tee "$NGINX_CONF" > /dev/null <<EOF
server {
    listen 443 ssl;
    server_name $SUB;

    ssl_certificate     /etc/letsencrypt/live/$SUB/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/$SUB/privkey.pem;

    location / {
        proxy_pass http://localhost:$PORT;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host \$host;
        proxy_cache_bypass \$http_upgrade;
    }
}

server {
    listen 80;
    server_name $SUB;
    return 301 https://\$host\$request_uri;
}
EOF

sudo nginx -t && sudo systemctl reload nginx

# 8) Reload & restart
sudo systemctl daemon-reload
sudo systemctl enable --now "$SERVICE"

echo "✅ Deployed $SUB → service: $SERVICE at https://$SUB"
