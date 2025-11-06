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
    *)
      echo "Unknown argument: $1" >&2
      exit 1
      ;;
  esac
done

INSTALL_PARENT=$(dirname "$INSTALL_DIR")
INSTALL_NAME=$(basename "$INSTALL_DIR")

echo "[1/9] Installing system dependencies"
apt-get update -y
apt-get install -y git curl unzip nginx postgresql postgresql-contrib certbot python3-certbot-nginx docker.io jq build-essential

BUN_INSTALL_ROOT="/usr/local/lib/bun"
if ! command -v bun >/dev/null 2>&1; then
  echo "[2/9] Installing Bun runtime"
  export BUN_INSTALL="$BUN_INSTALL_ROOT"
  curl -fsSL https://bun.sh/install | bash
  install -m 755 "$BUN_INSTALL/bin/bun" /usr/local/bin/bun
else
  export BUN_INSTALL="${BUN_INSTALL:-$BUN_INSTALL_ROOT}"
fi
export PATH="/usr/local/bin:$BUN_INSTALL/bin:$PATH"

SYSTEM_USER="bakery"
if ! id "$SYSTEM_USER" >/dev/null 2>&1; then
  useradd --system --create-home --shell /usr/sbin/nologin "$SYSTEM_USER"
fi

mkdir -p "$INSTALL_PARENT"
cd "$INSTALL_PARENT"

if [[ -n "$REPO_URL" ]]; then
  echo "[3/9] Cloning Bakery repository"
else
  REPO_URL="https://github.com/the-bakery-app/bakery.git"
  echo "[3/9] Cloning default Bakery repository $REPO_URL"
fi

rm -rf "$INSTALL_NAME"
git clone "$REPO_URL" "$INSTALL_NAME"

cd "$INSTALL_DIR"

echo "[4/9] Installing JavaScript dependencies"
bun --bun install
cd app && bun --bun install && bun run build && cd ..

mkdir -p /var/lib/bakery/data /var/lib/bakery/logs /var/lib/bakery/builds
mkdir -p /var/log/bakery
chown -R bakery:bakery /var/lib/bakery
chown -R bakery:bakery /var/log/bakery
chown -R bakery:bakery "$INSTALL_DIR"

DATABASE_PASSWORD=$(openssl rand -hex 12)
SESSION_SECRET=$(openssl rand -hex 32)
ENCRYPTION_KEY=$(openssl rand -hex 32 | cut -c1-32)
PUBLIC_IP=$(curl -s https://api.ipify.org || echo "127.0.0.1")
PUBLIC_IPV6=$(curl -s https://api64.ipify.org || true)
if [[ -n "$PUBLIC_IPV6" && "$PUBLIC_IPV6" == "$PUBLIC_IP" ]]; then
  PUBLIC_IPV6=""
fi
ADMIN_PASS=${ADMIN_PASS:-$(openssl rand -base64 12)}

if [[ -n "$BASE_URL" ]]; then
  BAKERY_BASE_URL="$BASE_URL"
else
  BAKERY_BASE_URL="https://${PUBLIC_IP}"
fi

sudo -u postgres psql <<SQL
DO \$\$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'bakery') THEN
    EXECUTE format('ALTER ROLE bakery WITH PASSWORD %L', '${DATABASE_PASSWORD}');
  ELSE
    EXECUTE format('CREATE ROLE bakery LOGIN PASSWORD %L', '${DATABASE_PASSWORD}');
  END IF;
END
\$\$;
SQL
sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='bakery'" | grep -q 1 || sudo -u postgres createdb -O bakery bakery

cat > .env <<EOF
NODE_ENV=production
DATABASE_URL=postgres://bakery:${DATABASE_PASSWORD}@localhost:5432/bakery
SESSION_SECRET=${SESSION_SECRET}
ENCRYPTION_KEY=${ENCRYPTION_KEY}
BAKERY_HOST=0.0.0.0
BAKERY_PORT=4100
HOST=0.0.0.0
PORT=4100
BAKERY_BASE_URL=${BAKERY_BASE_URL}
BAKERY_PUBLIC_IP=${PUBLIC_IP}
BAKERY_PUBLIC_IPV6=${PUBLIC_IPV6}
CERTBOT_EMAIL=${CERTBOT_EMAIL}
BAKERY_DATA_DIR=/var/lib/bakery/data
BAKERY_LOGS_DIR=/var/lib/bakery/logs
BAKERY_BUILDS_DIR=/var/lib/bakery/builds
GITHUB_CLIENT_ID=${GITHUB_CLIENT_ID}
GITHUB_CLIENT_SECRET=${GITHUB_CLIENT_SECRET}
EOF

echo "[5/9] Running database migrations"
bun run migrate

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
  BASE_HOST="${BAKERY_BASE_URL#*://}"
  BASE_HOST="${BASE_HOST%%/*}"
  BASE_HOST="${BASE_HOST%%:*}"
  if [[ -n "$BASE_HOST" && "$BASE_HOST" =~ [a-zA-Z] ]]; then
    ROOT_HOST=${BASE_HOST#*.}
    DNS_GUIDANCE+=$'\nDNS configuration tips:\n'
    DNS_GUIDANCE+=$"  A record    : ${BASE_HOST} -> ${PUBLIC_IP}\n"
    if [[ -n "$PUBLIC_IPV6" ]]; then
      DNS_GUIDANCE+=$"  AAAA record : ${BASE_HOST} -> ${PUBLIC_IPV6}\n"
    fi
    if [[ "$ROOT_HOST" != "$BASE_HOST" && -n "$ROOT_HOST" ]]; then
      DNS_GUIDANCE+=$"  A record    : *.${ROOT_HOST} -> ${PUBLIC_IP}\n"
      if [[ -n "$PUBLIC_IPV6" ]]; then
        DNS_GUIDANCE+=$"  AAAA record : *.${ROOT_HOST} -> ${PUBLIC_IPV6}\n"
      fi
      DNS_GUIDANCE+=$"  (wildcard enables app subdomains such as app.${ROOT_HOST})\n"
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
