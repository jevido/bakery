#!/usr/bin/env bash
set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  echo "This installer must be run as root" >&2
  exit 1
fi

REPO_URL=""
INSTALL_DIR="/opt/bakery"
CERTBOT_EMAIL=""
BASE_URL=""
ADMIN_EMAIL="jheremenis@gmail.com"
ADMIN_PASS=""
GITHUB_CLIENT_ID=""
GITHUB_CLIENT_SECRET=""
GITHUB_WEBHOOK_SECRET=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo)
      REPO_URL="$2"
      shift 2
      ;;
    --dir)
      INSTALL_DIR="$2"
      shift 2
      ;;
    --certbot-email)
      CERTBOT_EMAIL="$2"
      shift 2
      ;;
    --base-url)
      BASE_URL="$2"
      shift 2
      ;;
    --admin-email)
      ADMIN_EMAIL="$2"
      shift 2
      ;;
    --admin-pass)
      ADMIN_PASS="$2"
      shift 2
      ;;
    --github-client-id)
      GITHUB_CLIENT_ID="$2"
      shift 2
      ;;
    --github-client-secret)
      GITHUB_CLIENT_SECRET="$2"
      shift 2
      ;;
    --github-webhook-secret)
      GITHUB_WEBHOOK_SECRET="$2"
      shift 2
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 1
      ;;
  esac
done

echo "[1/9] Installing system dependencies"
apt-get update -y
apt-get install -y git curl unzip nginx postgresql postgresql-contrib certbot python3-certbot-nginx docker.io jq build-essential

if ! command -v bun >/dev/null 2>&1; then
  echo "[2/9] Installing Bun runtime"
  curl -fsSL https://bun.sh/install | bash
  export BUN_INSTALL="$HOME/.bun"
  ln -sf "$BUN_INSTALL/bin/bun" /usr/local/bin/bun
else
  export BUN_INSTALL="${BUN_INSTALL:-$HOME/.bun}"
fi
export PATH="$BUN_INSTALL/bin:$PATH"

SYSTEM_USER="bakery"
if ! id "$SYSTEM_USER" >/dev/null 2>&1; then
  useradd --system --create-home --shell /usr/sbin/nologin "$SYSTEM_USER"
fi

if [[ -n "$REPO_URL" ]]; then
  echo "[3/9] Cloning Bakery repository"
  rm -rf "$INSTALL_DIR"
  git clone "$REPO_URL" "$INSTALL_DIR"
else
  REPO_URL="https://github.com/the-bakery-app/bakery.git"
  echo "[3/9] Cloning default Bakery repository $REPO_URL"
  rm -rf "$INSTALL_DIR"
  git clone "$REPO_URL" "$INSTALL_DIR"
fi

cd "$INSTALL_DIR"

echo "[4/9] Installing JavaScript dependencies"
bun install
cd app && bun install && bun run build && cd ..

mkdir -p /var/lib/bakery/data /var/lib/bakery/logs /var/lib/bakery/builds
mkdir -p /var/log/bakery
chown -R bakery:bakery /var/lib/bakery
chown -R bakery:bakery /var/log/bakery
chown -R bakery:bakery "$INSTALL_DIR"

DATABASE_PASSWORD=$(openssl rand -hex 12)
SESSION_SECRET=$(openssl rand -hex 32)
ENCRYPTION_KEY=$(openssl rand -hex 32 | cut -c1-32)
PUBLIC_IP=$(curl -s https://api.ipify.org || echo "127.0.0.1")
ADMIN_PASS=${ADMIN_PASS:-$(openssl rand -base64 12)}

if [[ -n "$BASE_URL" ]]; then
  BAKERY_BASE_URL="$BASE_URL"
else
  BAKERY_BASE_URL="https://${PUBLIC_IP}"
fi

sudo -u postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='bakery'" | grep -q 1 || sudo -u postgres psql -c "CREATE ROLE bakery LOGIN PASSWORD '$DATABASE_PASSWORD';"
sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='bakery'" | grep -q 1 || sudo -u postgres createdb -O bakery bakery

cat > .env <<EOF
NODE_ENV=production
DATABASE_URL=postgres://bakery:${DATABASE_PASSWORD}@localhost:5432/bakery
SESSION_SECRET=${SESSION_SECRET}
ENCRYPTION_KEY=${ENCRYPTION_KEY}
BAKERY_HOST=0.0.0.0
BAKERY_PORT=4100
BAKERY_BASE_URL=${BAKERY_BASE_URL}
BAKERY_PUBLIC_IP=${PUBLIC_IP}
CERTBOT_EMAIL=${CERTBOT_EMAIL}
BAKERY_DATA_DIR=/var/lib/bakery/data
BAKERY_LOGS_DIR=/var/lib/bakery/logs
BAKERY_BUILDS_DIR=/var/lib/bakery/builds
GITHUB_CLIENT_ID=${GITHUB_CLIENT_ID}
GITHUB_CLIENT_SECRET=${GITHUB_CLIENT_SECRET}
GITHUB_APP_WEBHOOK_SECRET=${GITHUB_WEBHOOK_SECRET}
EOF

echo "[5/9] Running database migrations"
bun backend/lib/migrate.js

bun backend/scripts/create-admin.js "$ADMIN_EMAIL" "$ADMIN_PASS"

echo "[6/9] Installing systemd service"
sed "s|{{WORKING_DIR}}|$INSTALL_DIR|g" infrastructure/systemd/bakery.service > /etc/systemd/system/bakery.service
systemctl daemon-reload
systemctl enable bakery.service

echo "[7/9] Starting Bakery service"
systemctl restart bakery.service

echo "[8/9] Preparing nginx base configuration"
mkdir -p /etc/nginx/conf.d
cp infrastructure/nginx/templates/app.conf /etc/nginx/conf.d/bakery.template
systemctl reload nginx || true

echo "[9/9] Enabling automatic updates"
sed "s|{{WORKING_DIR}}|$INSTALL_DIR|g" infrastructure/systemd/bakery-update.service > /etc/systemd/system/bakery-update.service
cp infrastructure/systemd/bakery-update.timer /etc/systemd/system/bakery-update.timer
systemctl daemon-reload
systemctl enable bakery-update.timer
systemctl start bakery-update.timer

DNS_GUIDANCE=""
if [[ -n "$BAKERY_BASE_URL" ]]; then
  BASE_HOST=$(echo "$BAKERY_BASE_URL" | sed -E 's~^[a-zA-Z]+://([^/]+).*~\\1~')
  if [[ -n "$BASE_HOST" && ! "$BASE_HOST" =~ ^[0-9.]+$ ]]; then
    ROOT_HOST=${BASE_HOST#*.}
    DNS_GUIDANCE+=$'\nDNS configuration tips (Namecheap):\n'
    DNS_GUIDANCE+=$"  ${BASE_HOST} -> ${PUBLIC_IP}\n"
    if [[ "$ROOT_HOST" != "$BASE_HOST" && -n "$ROOT_HOST" ]]; then
      DNS_GUIDANCE+=$"  *.${ROOT_HOST} -> ${PUBLIC_IP}   (for app subdomains, e.g. biertje.${ROOT_HOST})\n"
    fi
  fi
fi

cat <<INFO

Bakery installation complete.

Admin credentials:
  email: ${ADMIN_EMAIL}
  password: ${ADMIN_PASS}

Bakery API: http://${PUBLIC_IP}:4100
Bakery UI: https://${PUBLIC_IP}
${DNS_GUIDANCE}

INFO
