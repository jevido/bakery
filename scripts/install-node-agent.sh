#!/usr/bin/env bash

# Todo: this file is a big placeholder

set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  echo "This installer must be run as root (use sudo)." >&2
  exit 1
fi

RESET="\033[0m"
BLUE="\033[34m"
GREEN="\033[32m"
YELLOW="\033[33m"

section() {
  local title="$1"
  echo -e "${BLUE}\n--------------------------------------------------------------------------------\n${title}\n--------------------------------------------------------------------------------${RESET}"
}

info() {
  echo -e "${GREEN}[INFO]${RESET} $1"
}

warn() {
  echo -e "${YELLOW}[WARN]${RESET} $1"
}

prompt_input() {
  local var_name="$1"
  local prompt_text="$2"
  local default_value="$3"
  local required="${4:-false}"
  local secret="${5:-false}"
  local current_value="${!var_name:-}"

  if [[ -n "$current_value" ]]; then
    default_value="$current_value"
  fi

  local suffix=""
  if [[ -n "$default_value" ]]; then
    suffix=" [$default_value]"
  fi

  local value
  while true; do
    if [[ "$secret" == true ]]; then
      read -rsp "${prompt_text}${suffix}: " value
      echo
    else
      read -rp "${prompt_text}${suffix}: " value
    fi

    if [[ -z "$value" ]]; then
      value="$default_value"
    fi

    if [[ -n "$value" || "$required" == false ]]; then
      printf -v "$var_name" '%s' "$value"
      break
    fi

    warn "This value is required."
  done
}

CONTROL_PLANE_URL="${CONTROL_PLANE_URL:-}"
NODE_TOKEN="${NODE_TOKEN:-}"
REPO_URL="${REPO_URL:-https://github.com/jevido/bakery.git}"
INSTALL_DIR="${INSTALL_DIR:-/opt/bakery-agent}"
ROOT_DIR="${ROOT_DIR:-/var/lib/bakery-node}"
LOG_DIR="${LOG_DIR:-/var/log/bakery-agent}"
SYSTEM_USER="${SYSTEM_USER:-bakery-agent}"

section "Collecting configuration"
prompt_input CONTROL_PLANE_URL "Control plane URL (e.g. https://bakery.example.com)" "$CONTROL_PLANE_URL" true
prompt_input NODE_TOKEN "Node installer token (from Servers page)" "$NODE_TOKEN" true true
prompt_input INSTALL_DIR "Agent install directory" "$INSTALL_DIR" true
prompt_input ROOT_DIR "Agent data root" "$ROOT_DIR" true
prompt_input REPO_URL "Repository URL" "$REPO_URL"

CONTROL_PLANE_URL="${CONTROL_PLANE_URL%/}"

cat <<SUMMARY

Configuration summary:
  Control plane API: $CONTROL_PLANE_URL
  Installer token:   (hidden)
  Install dir:       $INSTALL_DIR
  Data root:         $ROOT_DIR
  Repository:        $REPO_URL
SUMMARY

read -rp "Proceed with installation? [y/N]: " answer
case "${answer,,}" in
  y|yes) ;;
  *) warn "Installation aborted."; exit 1 ;;
esac

section "Installing prerequisites"
apt-get update -y
apt-get install -y git curl jq nginx docker.io certbot python3-certbot-nginx build-essential

section "Setting up Bun runtime"
BUN_INSTALL_ROOT="/usr/local/lib/bun"
if ! command -v bun >/dev/null 2>&1; then
  info "Installing Bun into $BUN_INSTALL_ROOT"
  export BUN_INSTALL="$BUN_INSTALL_ROOT"
  curl -fsSL https://bun.sh/install | bash
fi
if [[ -x "$BUN_INSTALL_ROOT/bin/bun" ]]; then
  install -m 755 "$BUN_INSTALL_ROOT/bin/bun" /usr/local/bin/bun
fi
export BUN_INSTALL="${BUN_INSTALL:-$BUN_INSTALL_ROOT}"
export PATH="/usr/local/bin:$BUN_INSTALL/bin:$PATH"

section "Creating system user and directories"
if ! id "$SYSTEM_USER" >/dev/null 2>&1; then
  useradd --system --create-home --shell /usr/sbin/nologin "$SYSTEM_USER"
fi
mkdir -p "$ROOT_DIR" "$ROOT_DIR/data" "$ROOT_DIR/logs" "$ROOT_DIR/builds" "$LOG_DIR"
chown -R "$SYSTEM_USER:$SYSTEM_USER" "$ROOT_DIR" "$LOG_DIR"

section "Fetching Bakery agent"
mkdir -p "$(dirname "$INSTALL_DIR")"
rm -rf "$INSTALL_DIR"
git clone "$REPO_URL" "$INSTALL_DIR"
cd "$INSTALL_DIR"

section "Installing dependencies"
bun --bun install --no-save

section "Registering node with control plane"
PUBLIC_IP=$(curl -fsSL https://api.ipify.org || echo "unknown")
REGISTER_PAYLOAD=$(cat <<JSON
{"token":"$NODE_TOKEN","hostname":"$(hostname)","platform":"$(uname -s)","arch":"$(uname -m)","version":"$(bun --version 2>/dev/null || echo unknown)","publicIp":"$PUBLIC_IP"}
JSON
)
REGISTER_RESPONSE=$(curl -fsSL -H "Content-Type: application/json" -X POST "$CONTROL_PLANE_URL/api/agent/register" -d "$REGISTER_PAYLOAD") || {
  echo "Failed to register agent" >&2
  exit 1
}
API_TOKEN=$(echo "$REGISTER_RESPONSE" | jq -r '.apiToken')
PAIRING_CODE=$(echo "$REGISTER_RESPONSE" | jq -r '.pairingCode')
NODE_ID=$(echo "$REGISTER_RESPONSE" | jq -r '.nodeId')

if [[ -z "$API_TOKEN" || "$API_TOKEN" == "null" ]]; then
  echo "Registration failed. Response: $REGISTER_RESPONSE" >&2
  exit 1
fi

ENV_FILE="$INSTALL_DIR/.env.agent"
cat > "$ENV_FILE" <<ENV
NODE_ENV=production
BAKERY_AGENT_API=$CONTROL_PLANE_URL
BAKERY_AGENT_TOKEN=$API_TOKEN
BAKERY_AGENT_POLL_INTERVAL=3000
BAKERY_NODE_ID=$NODE_ID
BAKERY_ROOT=$ROOT_DIR
BAKERY_DATA_DIR=$ROOT_DIR/data
BAKERY_LOGS_DIR=$ROOT_DIR/logs
BAKERY_BUILDS_DIR=$ROOT_DIR/builds
BAKERY_SYSTEMD_DIR=/etc/systemd/system
BAKERY_NGINX_SITES_DIR=/etc/nginx/conf.d
BAKERY_NGINX_TEMPLATE_DIR=$INSTALL_DIR/infrastructure/nginx/templates
ENV

chown -R "$SYSTEM_USER:$SYSTEM_USER" "$INSTALL_DIR"

section "Configuring systemd service"
SERVICE_PATH="/etc/systemd/system/bakery-agent.service"
sed -e "s|{{WORKING_DIR}}|$INSTALL_DIR|g" -e "s|{{ENV_FILE}}|$ENV_FILE|g" "$INSTALL_DIR/infrastructure/systemd/bakery-agent.service" > "$SERVICE_PATH"
chmod 644 "$SERVICE_PATH"

systemctl daemon-reload
systemctl enable bakery-agent.service
systemctl restart bakery-agent.service

section "Installation complete"
cat <<INFO
Node ID: $NODE_ID
Pairing code: $PAIRING_CODE

Paste the pairing code into the Bakery UI to activate this node.
INFO
