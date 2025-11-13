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
apt-get install -y curl git docker.io build-essential nginx certbot python3-certbot-nginx

section "Creating system user and directories"
if ! id "$SSH_USER" >/dev/null 2>&1; then
  useradd --system --create-home --shell /bin/bash "$SSH_USER"
else
  usermod --shell /bin/bash "$SSH_USER" || true
fi
mkdir -p "$DATA_ROOT"/{data,logs,builds} "$LOG_ROOT"
chown -R "$SSH_USER:$SSH_USER" "$DATA_ROOT" "$LOG_ROOT"

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

cat <<'INFO'

Node prepared. The Bakery control plane can now reach this server over SSH.
Return to the Bakery UI and click "Verify & activate" to finish linking the node.

INFO
