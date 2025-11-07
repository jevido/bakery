#!/usr/bin/env bash
set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  echo "This installer must be run as root" >&2
  exit 1
fi

render_template() {
  local template="$1"
  local target="$2"
  shift 2
  local -a env_pairs=("$@")
  env "${env_pairs[@]}" perl -0pe '
    s/\{\{([^}]+)\}\}/
      exists $ENV{$1} ? $ENV{$1} : die "Missing template variable $1\n"
    /gex
  ' "$template" >"$target"
}

is_ip_address() {
  local value="$1"
  [[ -z "$value" ]] && return 1
  if [[ "$value" =~ ^[0-9.]+$ ]]; then
    return 0
  fi
  if [[ "$value" == *:* ]]; then
    return 0
  fi
  return 1
}

ensure_control_plane_certificate() {
  local host="$1"
  local email="$2"
  local cert_dir="/etc/letsencrypt/live/$host"

  if [[ -z "$host" ]]; then
    return 1
  fi

  if [[ -f "$cert_dir/fullchain.pem" && -f "$cert_dir/privkey.pem" ]]; then
    echo "[nginx] Existing certificate detected for $host"
    return 0
  fi

  if [[ -z "$email" ]]; then
    echo "Error: Cannot request a Let's Encrypt certificate for $host without --certbot-email" >&2
    return 1
  fi

  echo "[nginx] Requesting Let's Encrypt certificate for $host (standalone)"
  systemctl stop nginx >/dev/null 2>&1 || true
  if certbot certonly \
    --standalone \
    --preferred-challenges http \
    --agree-tos \
    --non-interactive \
    -m "$email" \
    -d "$host"; then
    echo "[nginx] Certificate issued for $host"
    systemctl start nginx >/dev/null 2>&1 || true
    return 0
  else
    echo "Warning: Certbot failed for $host; continuing without HTTPS." >&2
    systemctl start nginx >/dev/null 2>&1 || true
    return 1
  fi
}

render_control_plane_nginx() {
  local host="$1"
  local port="$2"
  local certbot_email="$3"
  local template_path="$INSTALL_DIR/infrastructure/nginx/templates/app.conf"
  local target_path="/etc/nginx/conf.d/bakery.conf"
  local logs_dir="/var/log/bakery"

  mkdir -p "$logs_dir"

  local upstream_name="bakery_control_plane"
  local https_domains
  local http_redirects=""
  local primary_domain
  local listen_directive
  local ssl_directives=""
  local http2_directive=$'# http/1.1 only'
  local wants_https=true
  local cert_ready=false

  if [[ -z "$host" ]]; then
    wants_https=false
  elif is_ip_address "$host"; then
    wants_https=false
  fi

  if [[ "$wants_https" == true ]]; then
    if ensure_control_plane_certificate "$host" "$certbot_email"; then
      cert_ready=true
    fi
  fi

  if [[ "$wants_https" == true && "$cert_ready" == true ]]; then
    https_domains=$'server_name '"$host"$';'
    http_redirects=$'server {\n  listen 80;\n  server_name '"$host"$';\n  return 301 https://'"$host"$'\\$request_uri;\n}\n'
    primary_domain="$host"
    listen_directive=$'listen 443 ssl;'
    http2_directive=$'http2 on;'
    ssl_directives=$'    ssl_certificate /etc/letsencrypt/live/'"$host"$'/fullchain.pem;\n'
    ssl_directives+=$'    ssl_certificate_key /etc/letsencrypt/live/'"$host"$'/privkey.pem;\n'
    if [[ -f /etc/letsencrypt/options-ssl-nginx.conf ]]; then
      ssl_directives+=$'    include /etc/letsencrypt/options-ssl-nginx.conf;\n'
    fi
    if [[ -f /etc/letsencrypt/ssl-dhparams.pem ]]; then
      ssl_directives+=$'    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;\n'
    fi
  else
    https_domains=$'server_name '"${host:-_}"$';'
    primary_domain="${host:-_}"
    listen_directive=$'listen 80;'
    http2_directive=$'# http/1.1 only'
    http_redirects=""
  fi

  local access_log="$logs_dir/control-plane-access.log"
  local error_log="$logs_dir/control-plane-error.log"

  render_template "$template_path" "$target_path" \
    UPSTREAM_NAME="$upstream_name" \
    PORT="$port" \
    HTTPS_DOMAINS="$https_domains" \
    HTTP_REDIRECT_BLOCKS="$http_redirects" \
    LISTEN_DIRECTIVE="$listen_directive" \
    HTTP2_DIRECTIVE="$http2_directive" \
    SSL_DIRECTIVES="$ssl_directives" \
    ACCESS_LOG="$access_log" \
    ERROR_LOG="$error_log" \
    PRIMARY_DOMAIN="$primary_domain"

  if [[ "$wants_https" == true ]]; then
    ensure_control_plane_certificate "$host" "$certbot_email"
  fi
}

REPO_URL=""
INSTALL_DIR="/opt/bakery"
CERTBOT_EMAIL=""
BASE_URL=""
ADMIN_EMAIL="jheremenis@gmail.com"
ADMIN_PASS=""
GITHUB_CLIENT_ID=""
GITHUB_CLIENT_SECRET=""
BAKERY_LISTEN_HOST="0.0.0.0"
BAKERY_LISTEN_PORT="4100"

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
else
  export BUN_INSTALL="${BUN_INSTALL:-$BUN_INSTALL_ROOT}"
fi

# ensure bun binary is accessible to non-root users (avoid /root owned symlink)
if [[ -x "/usr/local/bin/bun" ]]; then
  BUN_TARGET=$(readlink -f /usr/local/bin/bun 2>/dev/null || true)
  if [[ -n "$BUN_TARGET" && "$BUN_TARGET" == /root/* ]]; then
    install -m 755 "$BUN_TARGET" /usr/local/bin/bun
  fi
elif [[ -x "$BUN_INSTALL/bin/bun" ]]; then
  install -m 755 "$BUN_INSTALL/bin/bun" /usr/local/bin/bun
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

chmod 755 app/build/index.js

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

BASE_HOST=""
if [[ -n "$BAKERY_BASE_URL" ]]; then
  BASE_HOST="${BAKERY_BASE_URL#*://}"
  BASE_HOST="${BASE_HOST%%/*}"
  BASE_HOST="${BASE_HOST%%:*}"
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
BAKERY_HOST=${BAKERY_LISTEN_HOST}
BAKERY_PORT=${BAKERY_LISTEN_PORT}
HOST=${BAKERY_LISTEN_HOST}
PORT=${BAKERY_LISTEN_PORT}
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
render_control_plane_nginx "$BASE_HOST" "$BAKERY_LISTEN_PORT" "$CERTBOT_EMAIL"
if nginx -t >/dev/null 2>&1; then
  systemctl reload nginx
else
  echo "Warning: nginx configuration test failed. Review /etc/nginx/conf.d/bakery.conf and obtain certificates before reloading nginx." >&2
fi

echo "[9/9] Enabling automatic updates"
sed "s|{{WORKING_DIR}}|$INSTALL_DIR|g" infrastructure/systemd/bakery-update.service > /etc/systemd/system/bakery-update.service
cp infrastructure/systemd/bakery-update.timer /etc/systemd/system/bakery-update.timer
systemctl daemon-reload
systemctl enable bakery-update.timer
systemctl start bakery-update.timer

DNS_GUIDANCE=""
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

cat <<INFO

Bakery installation complete.

Admin credentials:
  email: ${ADMIN_EMAIL}
  password: ${ADMIN_PASS}

Bakery API: http://${PUBLIC_IP}:4100
Bakery UI: https://${PUBLIC_IP}
${DNS_GUIDANCE}

INFO
