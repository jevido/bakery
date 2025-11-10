#!/usr/bin/env bash
set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  echo "This updater must be run as root (use sudo)." >&2
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

prompt_dir() {
  local var_name="$1"
  local prompt_text="$2"
  local default_value="$3"
  local current_value="${!var_name:-}"

  if [[ -n "$current_value" ]]; then
    default_value="$current_value"
  fi

  local value
  while true; do
    read -rp "${prompt_text} [${default_value}]: " value
    if [[ -z "$value" ]]; then
      value="$default_value"
    fi
    if [[ -d "$value" ]]; then
      printf -v "$var_name" '%s' "$value"
      break
    fi
    warn "Directory '$value' does not exist."
  done
}

ensure_bun() {
  local bun_install_root="/usr/local/lib/bun"
  if command -v bun >/dev/null 2>&1; then
    info "Bun already installed ($(bun --version))"
    return
  fi

  section "Installing Bun runtime"
  info "Installing Bun into $bun_install_root"
  export BUN_INSTALL="$bun_install_root"
  curl -fsSL https://bun.sh/install | bash
  if [[ -x "$BUN_INSTALL/bin/bun" ]]; then
    install -m 755 "$BUN_INSTALL/bin/bun" /usr/local/bin/bun
  fi
}

INSTALL_DIR="${INSTALL_DIR:-/opt/bakery}"

section "Locate Bakery installation"
prompt_dir INSTALL_DIR "Path to the existing Bakery install" "$INSTALL_DIR"
cd "$INSTALL_DIR"

section "Ensuring prerequisites"
apt-get update -y >/dev/null
apt-get install -y git curl ca-certificates >/dev/null
ensure_bun

section "Running Bakery updater"
bun infrastructure/scripts/update.js

section "Update complete"
info "Bakery at $INSTALL_DIR is now up to date."
