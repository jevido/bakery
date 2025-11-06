#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
APP_DIR=$(cd "$SCRIPT_DIR/../.." && pwd)

render_control_plane_nginx() {
  local host="$1"
  local port="$2"
  local template_path="$APP_DIR/infrastructure/nginx/templates/app.conf"
  local target_path="/etc/nginx/conf.d/bakery.conf"
  local logs_dir="/var/log/bakery"

  mkdir -p "$logs_dir"

  local upstream_name="bakery_control_plane"
  local https_domains
  local http_redirects=""
  local primary_domain

  if [[ -n "$host" ]]; then
    https_domains=$'server_name '"$host"$';'
    http_redirects=$'server {\n  listen 80;\n  server_name '"$host"$';\n  return 301 https://'"$host"$'\\$request_uri;\n}\n'
    primary_domain="$host"
  else
    https_domains=$'server_name _;'
    primary_domain="_"
  fi

  local access_log="$logs_dir/control-plane-access.log"
  local error_log="$logs_dir/control-plane-error.log"

  TEMPLATE_PATH="$template_path" \
  TARGET_PATH="$target_path" \
  UPSTREAM_NAME="$upstream_name" \
  PORT="$port" \
  HTTPS_DOMAINS="$https_domains" \
  HTTP_REDIRECT_BLOCKS="$http_redirects" \
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

render_control_plane_nginx "$CONTROL_PLANE_HOST" "$CONTROL_PLANE_PORT"
if nginx -t >/dev/null 2>&1; then
  systemctl reload nginx
else
  echo "Warning: nginx configuration test failed. Review /etc/nginx/conf.d/bakery.conf and obtain certificates before reloading nginx." >&2
fi

systemctl daemon-reload >/dev/null 2>&1 || true
systemctl restart bakery.service

echo "Bakery updated and restarted."
