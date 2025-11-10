#!/usr/bin/env bash
set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  echo "This installer must be run as root" >&2
  exit 1
fi

RESET="\033[0m"
BLUE="\033[34m"
GREEN="\033[32m"
YELLOW="\033[33m"

section() {
  echo -e "${BLUE}\n--------------------------------------------------------------------------------\n$1\n--------------------------------------------------------------------------------${RESET}"
}

info() {
  echo -e "${GREEN}[INFO]${RESET} $1"
}

warn() {
  echo -e "${YELLOW}[WARN]${RESET} $1"
}

read_env_value() {
  local file="$1"
  local key="$2"
  local value
  [[ -f "$file" ]] || return 1
  local status=0
  value=$(awk -v target="$key" '
    BEGIN { FS = "=" }
    /^[[:space:]]*#/ { next }
    /^[[:space:]]*$/ { next }
    {
      line = $0
      sub(/^[[:space:]]+/, "", line)
      if (index(line, "export ") == 1) {
        line = substr(line, 8)
        sub(/^[[:space:]]+/, "", line)
      }
      split(line, parts, "=")
      if (length(parts) < 2) next
      k = parts[1]
      sub(/[[:space:]]+$/, "", k)
      if (k != target) next
      v = substr(line, length(k) + 2)
      sub(/^[[:space:]]+/, "", v)
      sub(/[[:space:]]+$/, "", v)
      if ((substr(v,1,1) == "\"" && substr(v,length(v),1) == "\"") ||
          (substr(v,1,1) == "'" && substr(v,length(v),1) == "'")) {
        v = substr(v, 2, length(v) - 2)
      }
      print v
      exit 0
    }
    END { exit 1 }
  ' "$file") || status=$?
  if [[ $status -ne 0 ]]; then
    return 1
  fi
  printf '%s\n' "$value"
  return 0
}

render_template() {
  local template="$1"
  local target="$2"
  : >"$target"
  while IFS= read -r line || [[ -n "$line" ]]; do
    while [[ "$line" =~ \{\{([A-Za-z0-9_]+)\}\} ]]; do
      local key="${BASH_REMATCH[1]}"
      if [[ -z "${!key+x}" ]]; then
        echo "Missing template variable $key" >&2
        exit 1
      fi
      line="${line//\{\{$key\}\}/${!key}}"
    done
    printf '%s\n' "$line" >>"$target"
  done <"$template"
}

is_ip() {
  local value="$1"
  [[ -z "$value" ]] && return 1
  [[ "$value" =~ ^[0-9.]+$ || "$value" == *:* ]]
}

locate_certbot_tls_file() {
  local filename="$1" candidate
  shopt -s nullglob
  for pattern in \
    /usr/lib/python*/dist-packages \
    /usr/lib/python*/site-packages \
    /usr/local/lib/python*/dist-packages \
    /usr/local/lib/python*/site-packages; do
    for candidate in $pattern/certbot_nginx/_internal/tls_configs/"$filename"; do
      if [[ -f "$candidate" ]]; then
        echo "$candidate"
        shopt -u nullglob
        return 0
      fi
    done
  done
  shopt -u nullglob
  return 1
}

ensure_tls_defaults() {
  local option_target="/etc/letsencrypt/options-ssl-nginx.conf"
  local dh_target="/etc/letsencrypt/ssl-dhparams.pem"
  mkdir -p /etc/letsencrypt
  if [[ ! -f "$option_target" ]]; then
    if src=$(locate_certbot_tls_file options-ssl-nginx.conf); then
      cp "$src" "$option_target"
    fi
  fi
  if [[ ! -f "$dh_target" ]]; then
    if src=$(locate_certbot_tls_file ssl-dhparams.pem); then
      cp "$src" "$dh_target"
    fi
  fi
}

ensure_certificate() {
  local host="$1"
  local email="$2"
  local cert_dir="/etc/letsencrypt/live/$host"
  if [[ -z "$host" ]]; then
    return 1
  fi
  if [[ -f "$cert_dir/fullchain.pem" && -f "$cert_dir/privkey.pem" ]]; then
    info "[nginx] certificate already present for $host"
    return 0
  fi
  if [[ -z "$email" ]]; then
    warn "Cannot request certificate for $host without --certbot-email"
    return 1
  fi
  info "[nginx] requesting Let's Encrypt certificate for $host"
  systemctl stop nginx >/dev/null 2>&1 || true
  if certbot certonly --standalone --preferred-challenges http --agree-tos --non-interactive -m "$email" -d "$host"; then
    ensure_tls_defaults || true
    systemctl start nginx >/dev/null 2>&1 || true
    return 0
  fi
  warn "Certbot failed for $host"
  systemctl start nginx >/dev/null 2>&1 || true
  return 1
}

render_control_plane_nginx() {
  local host="$1"
  local port="$2"
  local certbot_email="$3"
  local template_path="$INSTALL_DIR/infrastructure/nginx/templates/app.conf"
  local target_path="/etc/nginx/conf.d/bakery.conf"
  local logs_dir="/var/log/bakery"

  mkdir -p "$logs_dir"

  local wants_https=true
  local http_redirects=""
  local listen_directive='listen 80;'
  local http2_directive='# http/1.1 only'
  local ssl_directives=""
  local https_domains="server_name ${host:-_};"
  local primary_domain="${host:-_}"

  if [[ -z "$host" ]] || is_ip "$host"; then
    wants_https=false
  fi

  if $wants_https; then
    http_redirects=$'server {\n  listen 80;\n  server_name '"$host"$';\n  return 301 https://'"$host"$'$request_uri;\n}\n'
    if ensure_certificate "$host" "$certbot_email"; then
      ensure_tls_defaults || true
      listen_directive='listen 443 ssl;'
      http2_directive='http2 on;'
      ssl_directives=$'    ssl_certificate /etc/letsencrypt/live/'"$host"$'/fullchain.pem;\n'
      ssl_directives+=$'    ssl_certificate_key /etc/letsencrypt/live/'"$host"$'/privkey.pem;\n'
      if [[ -f /etc/letsencrypt/options-ssl-nginx.conf ]]; then
        ssl_directives+=$'    include /etc/letsencrypt/options-ssl-nginx.conf;\n'
      fi
      if [[ -f /etc/letsencrypt/ssl-dhparams.pem ]]; then
        ssl_directives+=$'    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;\n'
      fi
    else
      wants_https=false
      http_redirects=""
    fi
  fi

  UPSTREAM_NAME="bakery_control_plane" \
  PORT="$port" \
  HTTPS_DOMAINS="$https_domains" \
  HTTP_REDIRECT_BLOCKS="$http_redirects" \
  LISTEN_DIRECTIVE="$listen_directive" \
  HTTP2_DIRECTIVE="$http2_directive" \
  SSL_DIRECTIVES="$ssl_directives" \
  ACCESS_LOG="$logs_dir/control-plane-access.log" \
  ERROR_LOG="$logs_dir/control-plane-error.log" \
  PRIMARY_DOMAIN="$primary_domain" \
    render_template "$template_path" "$target_path"

  if $wants_https; then
    ensure_certificate "$host" "$certbot_email" || true
  fi
}

REPO_URL=""
INSTALL_DIR="/opt/bakery"
CERTBOT_EMAIL=""
BASE_URL=""
ADMIN_EMAIL=""
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

REPO_URL=${REPO_URL:-"https://github.com/jevido/bakery.git"}
if [[ -z "$INSTALL_DIR" ]]; then
  echo "Error: --dir is required" >&2
  exit 1
fi
if [[ -z "$ADMIN_EMAIL" ]]; then
  echo "Error: --admin-email is required" >&2
  exit 1
fi
if [[ -z "$ADMIN_PASS" ]]; then
  ADMIN_PASS=$(openssl rand -base64 12)
  warn "Generated temporary admin password"
fi

INSTALL_PARENT=$(dirname "$INSTALL_DIR")
INSTALL_NAME=$(basename "$INSTALL_DIR")
SYSTEM_USER="bakery"
BUN_INSTALL_ROOT="/usr/local/lib/bun"

install_packages() {
  section "Installing system packages"
  apt-get update -y
  apt-get install -y git curl unzip nginx postgresql postgresql-contrib certbot python3-certbot-nginx docker.io jq build-essential
}

install_bun() {
  section "Ensuring Bun runtime"
  if command -v bun >/dev/null 2>&1; then
    info "Bun $(bun --version) already installed"
    BUN_INSTALL="${BUN_INSTALL:-$BUN_INSTALL_ROOT}"
  else
    info "Installing Bun into $BUN_INSTALL_ROOT"
    export BUN_INSTALL="$BUN_INSTALL_ROOT"
    curl -fsSL https://bun.sh/install | bash
  fi
  if [[ -x "/usr/local/bin/bun" ]]; then
    local target
    target=$(readlink -f /usr/local/bin/bun 2>/dev/null || true)
    if [[ -n "$target" && "$target" == /root/* ]]; then
      install -m 755 "$target" /usr/local/bin/bun
    fi
  elif [[ -x "$BUN_INSTALL_ROOT/bin/bun" ]]; then
    install -m 755 "$BUN_INSTALL_ROOT/bin/bun" /usr/local/bin/bun
  fi
  export PATH="/usr/local/bin:$BUN_INSTALL_ROOT/bin:$PATH"
}

prepare_system_user() {
  section "Preparing system user and directories"
  if ! id "$SYSTEM_USER" >/dev/null 2>&1; then
    useradd --system --create-home --shell /usr/sbin/nologin "$SYSTEM_USER"
    info "Created system user $SYSTEM_USER"
  fi
  mkdir -p "$INSTALL_PARENT" /var/lib/bakery/{data,logs,builds} /var/log/bakery
  chown -R "$SYSTEM_USER:$SYSTEM_USER" /var/lib/bakery /var/log/bakery
}

sync_repository() {
  section "Fetching Bakery source"
  cd "$INSTALL_PARENT"
  if [[ -d "$INSTALL_DIR/.git" ]]; then
    info "Existing checkout detected; syncing"
    cd "$INSTALL_DIR"
    git fetch --all --tags
    git reset --hard origin/HEAD
  else
    rm -rf "$INSTALL_NAME"
    git clone "$REPO_URL" "$INSTALL_NAME"
    cd "$INSTALL_DIR"
  fi
}

install_dependencies() {
  section "Installing application dependencies"
  bun --bun install
  pushd app >/dev/null
  bun --bun install
  bun run build
  popd >/dev/null
  chmod 755 app/build/index.js
  chown -R "$SYSTEM_USER:$SYSTEM_USER" "$INSTALL_DIR"
}

configure_environment() {
  section "Configuring application environment"
  local env_file="$INSTALL_DIR/.env"
  local existing_database_url=""
  local existing_session_secret=""
  local existing_encryption_key=""
  local existing_base_url=""
  local existing_public_ip=""
  local existing_public_ipv6=""

  if [[ -f "$env_file" ]]; then
    existing_database_url=$(read_env_value "$env_file" "DATABASE_URL" || true)
    existing_session_secret=$(read_env_value "$env_file" "SESSION_SECRET" || true)
    existing_encryption_key=$(read_env_value "$env_file" "ENCRYPTION_KEY" || true)
    existing_base_url=$(read_env_value "$env_file" "BAKERY_BASE_URL" || true)
    existing_public_ip=$(read_env_value "$env_file" "BAKERY_PUBLIC_IP" || true)
    existing_public_ipv6=$(read_env_value "$env_file" "BAKERY_PUBLIC_IPV6" || true)
  fi

  if [[ -n "$existing_session_secret" ]]; then
    SESSION_SECRET="$existing_session_secret"
  else
    SESSION_SECRET=$(openssl rand -hex 32)
  fi

  if [[ -n "$existing_encryption_key" ]]; then
    ENCRYPTION_KEY="$existing_encryption_key"
  else
    ENCRYPTION_KEY=$(openssl rand -hex 32 | cut -c1-32)
  fi

  PUBLIC_IP=$(curl -s https://api.ipify.org || echo "127.0.0.1")
  PUBLIC_IPV6=$(curl -s https://api64.ipify.org || true)
  if [[ -n "$PUBLIC_IPV6" && "$PUBLIC_IPV6" == "$PUBLIC_IP" ]]; then
    PUBLIC_IPV6=""
  fi
  if [[ -n "$existing_public_ip" ]]; then
    PUBLIC_IP="$existing_public_ip"
  fi
  if [[ -n "$existing_public_ipv6" ]]; then
    PUBLIC_IPV6="$existing_public_ipv6"
  fi

  if [[ -n "$BASE_URL" ]]; then
    BAKERY_BASE_URL="$BASE_URL"
  elif [[ -n "$existing_base_url" ]]; then
    BAKERY_BASE_URL="$existing_base_url"
  else
    BAKERY_BASE_URL="https://${PUBLIC_IP}"
  fi
  BASE_HOST=""
  if [[ -n "$BAKERY_BASE_URL" ]]; then
    BASE_HOST="${BAKERY_BASE_URL#*://}"
    BASE_HOST="${BASE_HOST%%/*}"
    BASE_HOST="${BASE_HOST%%:*}"
  fi

  local generated_db_password=""
  if [[ -n "$existing_database_url" ]]; then
    DATABASE_URL="$existing_database_url"
    info "Preserving existing DATABASE_URL from $env_file"
  else
    generated_db_password=$(openssl rand -hex 12)
    DATABASE_URL="postgres://bakery:${generated_db_password}@localhost:5432/bakery"
  fi

  if [[ -n "$generated_db_password" ]]; then
    sudo -u postgres psql <<SQL
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'bakery') THEN
    EXECUTE format('ALTER ROLE bakery WITH PASSWORD %L', '${generated_db_password}');
  ELSE
    EXECUTE format('CREATE ROLE bakery LOGIN PASSWORD %L', '${generated_db_password}');
  END IF;
END
$$;
SQL
    sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='bakery'" | grep -q 1 || sudo -u postgres createdb -O bakery bakery
  fi

  DATABASE_URL="$DATABASE_URL" \
  BAKERY_BASE_URL="$BAKERY_BASE_URL" \
  PUBLIC_IP="$PUBLIC_IP" \
  PUBLIC_IPV6="$PUBLIC_IPV6" \
  SESSION_SECRET="$SESSION_SECRET" \
  ENCRYPTION_KEY="$ENCRYPTION_KEY" \
    render_template "$INSTALL_DIR/infrastructure/templates/install.env" "$env_file"
  chown "$SYSTEM_USER:$SYSTEM_USER" "$env_file"
}

run_migrations_and_seed() {
  section "Applying database migrations"
  bun run migrate
  bun backend/scripts/create-admin.js "$ADMIN_EMAIL" "$ADMIN_PASS"
}

configure_systemd() {
  section "Configuring systemd service"
  sed "s|{{WORKING_DIR}}|$INSTALL_DIR|g" "$INSTALL_DIR/infrastructure/systemd/bakery.service" > /etc/systemd/system/bakery.service
  systemctl daemon-reload
  systemctl enable bakery.service
  systemctl restart bakery.service
}

configure_nginx_stage() {
  section "Configuring nginx"
  mkdir -p /etc/nginx/conf.d
  cp "$INSTALL_DIR/infrastructure/nginx/templates/app.conf" /etc/nginx/conf.d/bakery.template
  render_control_plane_nginx "$BASE_HOST" "$BAKERY_LISTEN_PORT" "$CERTBOT_EMAIL"
  if nginx -t >/dev/null 2>&1; then
    if systemctl is-active --quiet nginx; then
      systemctl reload nginx >/dev/null 2>&1 || systemctl restart nginx >/dev/null 2>&1 || systemctl start nginx >/dev/null 2>&1 || true
    else
      systemctl start nginx >/dev/null 2>&1 || true
    fi
  else
    warn "nginx configuration test failed; review /etc/nginx/conf.d/bakery.conf"
  fi
}

configure_updates() {
  section "Enabling automatic updates"
  sed "s|{{WORKING_DIR}}|$INSTALL_DIR|g" "$INSTALL_DIR/infrastructure/systemd/bakery-update.service" > /etc/systemd/system/bakery-update.service
  cp "$INSTALL_DIR/infrastructure/systemd/bakery-update.timer" /etc/systemd/system/bakery-update.timer
  systemctl daemon-reload
  systemctl enable bakery-update.timer
  systemctl start bakery-update.timer
}

print_summary() {
  local dns=""
  if [[ -n "$BASE_HOST" && "$BASE_HOST" =~ [a-zA-Z] ]]; then
    local root_host=${BASE_HOST#*.}
    dns+=$'DNS configuration tips:\n'
    dns+=$"  A record    : ${BASE_HOST} -> ${PUBLIC_IP}\n"
    if [[ -n "$PUBLIC_IPV6" ]]; then
      dns+=$"  AAAA record : ${BASE_HOST} -> ${PUBLIC_IPV6}\n"
    fi
    if [[ "$root_host" != "$BASE_HOST" && -n "$root_host" ]]; then
      dns+=$"  A record    : *.${root_host} -> ${PUBLIC_IP}\n"
      if [[ -n "$PUBLIC_IPV6" ]]; then
        dns+=$"  AAAA record : *.${root_host} -> ${PUBLIC_IPV6}\n"
      fi
    fi
  fi

  section "Installation complete"
  cat <<SUMMARY
Bakery installation complete.

Admin credentials:
  email: ${ADMIN_EMAIL}
  password: ${ADMIN_PASS}

Service status commands:
  systemctl status bakery.service
  systemctl status bakery-update.timer

Access:
  API: http://${PUBLIC_IP}:${BAKERY_LISTEN_PORT}
  UI : ${BAKERY_BASE_URL}

${dns}
SUMMARY
}

install_packages
install_bun
prepare_system_user
sync_repository
install_dependencies
configure_environment
run_migrations_and_seed
configure_systemd
configure_nginx_stage
configure_updates
print_summary
