#!/usr/bin/env bash

set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  echo "This installer must be run as root (use sudo)." >&2
  exit 1
fi

SSH_USER="${SSH_USER:-bakery-agent}"
SSH_KEY_BASE64="${SSH_KEY_BASE64:-}"
DATA_ROOT="${DATA_ROOT:-/var/lib/bakery-node}"
LOG_ROOT="${LOG_ROOT:-/var/log/bakery-node}"

if [[ -z "$SSH_KEY_BASE64" ]]; then
  echo "Missing SSH_KEY_BASE64. Run this script via the Bakery control plane UI." >&2
  exit 1
fi

RESET="\033[0m"
BLUE="\033[34m"
GREEN="\033[32m"

section() {
  local title="$1"
  echo -e "${BLUE}\n--------------------------------------------------------------------------------\n${title}\n--------------------------------------------------------------------------------${RESET}"
}

info() {
  echo -e "${GREEN}[INFO]${RESET} $1"
}

section "Installing prerequisites"
apt-get update -y
apt-get install -y curl git docker.io docker-cli build-essential nginx certbot python3-certbot-nginx postgresql postgresql-contrib

section "Creating system user and directories"
if ! id "$SSH_USER" >/dev/null 2>&1; then
  useradd --system --create-home --shell /bin/bash "$SSH_USER"
else
  usermod --shell /bin/bash "$SSH_USER" || true
fi

if ! getent group docker >/dev/null; then
  groupadd docker
fi

usermod -aG docker "$SSH_USER" || true

mkdir -p "$DATA_ROOT"/{data,logs,builds} "$LOG_ROOT"
chown -R "$SSH_USER:$SSH_USER" "$DATA_ROOT" "$LOG_ROOT"

TEMPLATE_DIR="$DATA_ROOT/templates/nginx"
mkdir -p "$TEMPLATE_DIR"

cat >"$TEMPLATE_DIR/app.conf" <<'NGINX_TEMPLATE'
upstream {{UPSTREAM_NAME}} {
    server 127.0.0.1:{{PORT}};
    keepalive 32;
}

{{HTTP_REDIRECT_BLOCKS}}

server {
    {{LISTEN_DIRECTIVE}}
    {{HTTP2_DIRECTIVE}}
    {{HTTPS_DOMAINS}}

{{SSL_DIRECTIVES}}

    access_log {{ACCESS_LOG}};
    error_log {{ERROR_LOG}};

    location / {
        proxy_pass http://{{UPSTREAM_NAME}};
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $http_connection;
        proxy_set_header Origin $http_origin;
        proxy_cache_bypass $http_upgrade;
        proxy_buffering on;
    }
}
NGINX_TEMPLATE

chown -R "$SSH_USER:$SSH_USER" "$DATA_ROOT/templates"

section "Authorizing Bakery control plane"
SSH_DIR=$(eval echo "~$SSH_USER/.ssh")
mkdir -p "$SSH_DIR"
chmod 700 "$SSH_DIR"

SSH_PUBLIC_KEY=$(printf '%s' "$SSH_KEY_BASE64" | base64 -d)
AUTHORIZED_KEYS="$SSH_DIR/authorized_keys"
touch "$AUTHORIZED_KEYS"

if ! grep -qxF "$SSH_PUBLIC_KEY" "$AUTHORIZED_KEYS"; then
  echo "$SSH_PUBLIC_KEY" >> "$AUTHORIZED_KEYS"
fi

chown -R "$SSH_USER:$SSH_USER" "$SSH_DIR"
chmod 600 "$AUTHORIZED_KEYS"

section "Finalizing"
systemctl enable --now docker >/dev/null 2>&1 || true
systemctl enable --now postgresql >/dev/null 2>&1 || true

# SAFER SUDOERS CONFIGURATION
SUDOERS_FILE="/etc/sudoers.d/${SSH_USER}"

cat >"$SUDOERS_FILE" <<EOF
Defaults:${SSH_USER} !requiretty

# Minimal safe root privileges
${SSH_USER} ALL=(root) NOPASSWD: \
  /usr/bin/systemctl, \
  /usr/bin/certbot

# PostgreSQL administrative privileges
${SSH_USER} ALL=(postgres) NOPASSWD: \
  /usr/bin/psql, \
  /usr/bin/createuser, \
  /usr/bin/createdb, \
  /usr/bin/dropdb, \
  /usr/bin/dropuser
EOF

chmod 440 "$SUDOERS_FILE"

cat <<'INFO'

Node prepared. The Bakery control plane can now reach this server over SSH.
Return to the Bakery UI and click "Verify & activate" to finish linking the node.

INFO
