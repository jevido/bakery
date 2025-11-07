#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
APP_DIR=$(cd "$SCRIPT_DIR/../.." && pwd)

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

ensure_certbot_ssl_defaults() {
  python3 - <<'PY'
import os
import shutil

try:
    from certbot_nginx._internal import tls_configs
except Exception:
    raise SystemExit(0)

base = os.path.dirname(tls_configs.__file__)
targets = {
    'options-ssl-nginx.conf': os.path.join('/etc/letsencrypt', 'options-ssl-nginx.conf'),
    'ssl-dhparams.pem': os.path.join('/etc/letsencrypt', 'ssl-dhparams.pem')
}

os.makedirs('/etc/letsencrypt', exist_ok=True)

for name, destination in targets.items():
    source = os.path.join(base, name)
    if os.path.exists(source) and not os.path.exists(destination):
        shutil.copyfile(source, destination)
PY
}

ensure_control_plane_certificate() {
  local host="$1"
  local email="$2"
  local cert_dir="/etc/letsencrypt/live/$host"

  if [[ -z "$host" ]]; then
    return 0
  fi

  if [[ -f "$cert_dir/fullchain.pem" && -f "$cert_dir/privkey.pem" ]]; then
    echo "[nginx] Existing certificate detected for $host"
    return 0
  fi

  if [[ -z "$email" ]]; then
    echo "Error: Cannot request a Let's Encrypt certificate for $host without CERTBOT_EMAIL" >&2
    exit 1
  fi

  echo "[nginx] Requesting Let's Encrypt certificate for $host"
  certbot certonly \
    --nginx \
    --agree-tos \
    --non-interactive \
    -m "$email" \
    -d "$host"
}

render_control_plane_nginx() {
  local host="$1"
  local port="$2"
  local certbot_email="$3"
  local template_path="$APP_DIR/infrastructure/nginx/templates/app.conf"
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

  if [[ -z "$host" ]]; then
    wants_https=false
  elif is_ip_address "$host"; then
    wants_https=false
  fi

  if [[ "$wants_https" == true ]]; then
    ensure_certbot_ssl_defaults
    https_domains=$'server_name '"$host"$';'
    http_redirects=$'server {\n  listen 80;\n  server_name '"$host"$';\n  return 301 https://'"$host"$'\\$request_uri;\n}\n'
    primary_domain="$host"
    listen_directive=$'listen 443 ssl;'
    http2_directive=$'http2 on;'
    ssl_directives=$'    ssl_certificate /etc/letsencrypt/live/'"$host"$'/fullchain.pem;\n'
    ssl_directives+=$'    ssl_certificate_key /etc/letsencrypt/live/'"$host"$'/privkey.pem;\n'
    ssl_directives+=$'    include /etc/letsencrypt/options-ssl-nginx.conf;\n'
    ssl_directives+=$'    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;\n'
  else
    https_domains=$'server_name '"${host:-_}"$';'
    primary_domain="${host:-_}"
    listen_directive=$'listen 80;'
    http2_directive=$'# http/1.1 only'
    http_redirects=""
  fi

  local access_log="$logs_dir/control-plane-access.log"
  local error_log="$logs_dir/control-plane-error.log"

  TEMPLATE_PATH="$template_path" \
  TARGET_PATH="$target_path" \
  UPSTREAM_NAME="$upstream_name" \
  PORT="$port" \
  HTTPS_DOMAINS="$https_domains" \
  HTTP_REDIRECT_BLOCKS="$http_redirects" \
  LISTEN_DIRECTIVE="$listen_directive" \
  HTTP2_DIRECTIVE="$http2_directive" \
  SSL_DIRECTIVES="$ssl_directives" \
  ACCESS_LOG="$access_log" \
  ERROR_LOG="$error_log" \
  PRIMARY_DOMAIN="$primary_domain" \
  python3 - <<'PY'
import os
import re

template_path = os.environ["TEMPLATE_PATH"]
target_path = os.environ["TARGET_PATH"]

with open(template_path, "r", encoding="utf-8") as fh:
    template = fh.read()

variables = {
    "UPSTREAM_NAME": os.environ["UPSTREAM_NAME"],
    "PORT": os.environ["PORT"],
    "HTTPS_DOMAINS": os.environ["HTTPS_DOMAINS"],
    "HTTP_REDIRECT_BLOCKS": os.environ["HTTP_REDIRECT_BLOCKS"],
    "LISTEN_DIRECTIVE": os.environ["LISTEN_DIRECTIVE"],
    "HTTP2_DIRECTIVE": os.environ["HTTP2_DIRECTIVE"],
    "SSL_DIRECTIVES": os.environ["SSL_DIRECTIVES"],
    "ACCESS_LOG": os.environ["ACCESS_LOG"],
    "ERROR_LOG": os.environ["ERROR_LOG"],
    "PRIMARY_DOMAIN": os.environ["PRIMARY_DOMAIN"],
}

def replace(match):
    key = match.group(1).strip()
    try:
        return variables[key]
    except KeyError:
        raise SystemExit(f"Missing template variable {key}")

rendered = re.sub(r"\{\{(.*?)\}\}", replace, template)

with open(target_path, "w", encoding="utf-8") as fh:
    fh.write(rendered)
PY

  if [[ "$wants_https" == true ]]; then
    ensure_control_plane_certificate "$host" "$certbot_email"
  fi
}

echo "Updating Bakery in $APP_DIR"

cd "$APP_DIR"
if [ -d .git ]; then
  git config --global --add safe.directory "$APP_DIR" >/dev/null 2>&1 || true
  git pull --rebase
fi

if [[ -f .env ]]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi

BUN_INSTALL_ROOT="/usr/local/lib/bun"
export BUN_INSTALL="${BUN_INSTALL:-$BUN_INSTALL_ROOT}"
export PATH="/usr/local/bin:$BUN_INSTALL/bin:$PATH"

if [[ -x "/usr/local/bin/bun" ]]; then
  BUN_TARGET=$(readlink -f /usr/local/bin/bun 2>/dev/null || true)
  if [[ -n "$BUN_TARGET" && "$BUN_TARGET" == /root/* ]]; then
    install -m 755 "$BUN_TARGET" /usr/local/bin/bun
  fi
elif [[ -x "$BUN_INSTALL/bin/bun" ]]; then
  install -m 755 "$BUN_INSTALL/bin/bun" /usr/local/bin/bun
fi

bun --bun install
cd app && bun --bun install && bun run build && cd ..

chmod 755 app/build/index.js

bun run migrate
sed "s|{{WORKING_DIR}}|$APP_DIR|g" infrastructure/systemd/bakery.service > /etc/systemd/system/bakery.service

CONTROL_PLANE_PORT="${BAKERY_PORT:-4100}"
CONTROL_PLANE_HOST=""
if [[ -n "${BAKERY_BASE_URL:-}" ]]; then
  CONTROL_PLANE_HOST="${BAKERY_BASE_URL#*://}"
  CONTROL_PLANE_HOST="${CONTROL_PLANE_HOST%%/*}"
  CONTROL_PLANE_HOST="${CONTROL_PLANE_HOST%%:*}"
fi

render_control_plane_nginx "$CONTROL_PLANE_HOST" "$CONTROL_PLANE_PORT" "${CERTBOT_EMAIL:-}"
if nginx -t >/dev/null 2>&1; then
  systemctl reload nginx
else
  echo "Warning: nginx configuration test failed. Review /etc/nginx/conf.d/bakery.conf and obtain certificates before reloading nginx." >&2
fi

systemctl daemon-reload >/dev/null 2>&1 || true
systemctl restart bakery.service

echo "Bakery updated and restarted."
