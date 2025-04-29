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
NGINX_CONF="/etc/nginx/sites-available/$SUB"
NGINX_LINK="/etc/nginx/sites-enabled/$SUB"
APP_SLUG=$(echo "$SUB" | tr . _ | tr -cd '[:alnum:]_') # Safe for postgres

# Determine port
if [ -f "$PORT_FILE" ]; then
  PORT=$(cat "$PORT_FILE")
else
  BASE_PORT=3000
  MAX_PORT=4000
  USED_PORTS=$(ss -tuln | awk '{print $5}' | grep -oE '[0-9]+$' | sort -n | uniq)
  for ((p = $BASE_PORT; p <= $MAX_PORT; p++)); do
    if ! echo "$USED_PORTS" | grep -q "^$p$"; then
      PORT=$p
      echo "$PORT" | sudo tee "$PORT_FILE" >/dev/null
      break
    fi
  done

  if [ -z "$PORT" ]; then
    echo "❌ Could not find a free port between $BASE_PORT and $MAX_PORT."
    exit 1
  fi
fi

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

# 3) Do database shit like migrations and creating initial database
if [ ! -f "$APP_DIR/.env" ]; then
  echo "ℹ️ .env not found. Creating database and .env for $SUB."

  DB_PASSWORD=$(openssl rand -base64 32 | tr -dc 'a-zA-Z0-9' | head -c 32)
  DB_NAME="bakery_${APP_SLUG}"
  DB_USER="bakery_${APP_SLUG}"

  sudo -u postgres psql <<EOF
CREATE DATABASE "$DB_NAME";
CREATE USER "$DB_USER" WITH ENCRYPTED PASSWORD '$DB_PASSWORD';
GRANT ALL PRIVILEGES ON DATABASE "$DB_NAME" TO "$DB_USER";
EOF

  echo "✅ Database and user created: $DB_NAME / $DB_USER"

  # ✅ Write postgres url to .env
  cat <<EOT >"$APP_DIR/.env"
DATABASE_URL=postgresql://$DB_USER:$DB_PASSWORD@localhost:5432/$DB_NAME
EOT

  echo "✅ .env created at $APP_DIR/.env"
else
  echo "ℹ️ .env already exists, skipping database setup."
fi

# Always copy .env back to current release
sudo -u bakery cp "$APP_DIR/.env" "$CURRENT/.env"

# 4) Install deps & build as bakery user
sudo -u bakery bash -lc "
  cd $CURRENT
  bun install
  bun --bun run build
"

if sudo -u bakery bash -lc "bun run" | grep -q 'db:migrate'; then
  echo "🔎 db:migrate script found, running migrations..."
  sudo -u bakery bash -lc "bun run db:migrate"
else
  echo "ℹ️ No db:migrate script found, skipping migrations."
fi

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
sudo tee "$NGINX_CONF" >/dev/null <<EOF
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

sudo ln -sf "$NGINX_CONF" "$NGINX_LINK"
sudo nginx -t && sudo systemctl restart nginx

# 7) Create systemd service
sudo tee /etc/systemd/system/$SERVICE.service >/dev/null <<EOF
[Unit]
Description=Bun app for $SUB
After=network.target

[Service]
Type=simple
User=bakery
WorkingDirectory=$CURRENT
ExecStart=/home/bakery/.bun/bin/bun start
Environment=PORT=$PORT
Restart=always

[Install]
WantedBy=multi-user.target
EOF

# 8) Reload & restart
sudo systemctl daemon-reload
sudo systemctl enable "$SERVICE"
sudo systemctl restart "$SERVICE"

echo "✅ Deployed $SUB → service: $SERVICE at https://$SUB"
