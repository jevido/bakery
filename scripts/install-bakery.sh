#!/usr/bin/env bash
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

generate_password() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -base64 24 | tr -d '=+/' | cut -c1-24
  else
    head -c 64 /dev/urandom | base64 | tr -d '=+/' | cut -c1-24
  fi
}

confirm_prompt() {
  local answer
  read -rp "Proceed with installation? [y/N]: " answer
  case "${answer,,}" in
    y|yes) return 0 ;;
    *) warn "Installation aborted."; exit 1 ;;
  esac
}

REPO_URL="${REPO_URL:-https://github.com/jevido/bakery.git}"
INSTALL_DIR="${INSTALL_DIR:-/opt/bakery}"
CERTBOT_EMAIL="${CERTBOT_EMAIL:-}"
BASE_URL="${BASE_URL:-}"
ADMIN_EMAIL="${ADMIN_EMAIL:-}"
ADMIN_PASS="${ADMIN_PASS:-}"
GITHUB_CLIENT_ID="${GITHUB_CLIENT_ID:-}"
GITHUB_CLIENT_SECRET="${GITHUB_CLIENT_SECRET:-}"

section "Collecting configuration"
prompt_input REPO_URL "Repository URL" "$REPO_URL"
prompt_input INSTALL_DIR "Install directory" "$INSTALL_DIR" true
prompt_input BASE_URL "Public base URL (e.g. https://bakery.example.com)" "$BASE_URL" true
prompt_input CERTBOT_EMAIL "Certbot email" "$CERTBOT_EMAIL" true
prompt_input ADMIN_EMAIL "Initial admin email" "$ADMIN_EMAIL" true
prompt_input ADMIN_PASS "Initial admin password (leave blank for random)" "$ADMIN_PASS" false true
prompt_input GITHUB_CLIENT_ID "GitHub OAuth app client ID" "$GITHUB_CLIENT_ID" true
prompt_input GITHUB_CLIENT_SECRET "GitHub OAuth app client secret" "$GITHUB_CLIENT_SECRET" true true

if [[ -z "$ADMIN_PASS" ]]; then
  ADMIN_PASS=$(generate_password)
  info "Generated admin password: $ADMIN_PASS"
fi

cat <<SUMMARY

Configuration summary:
  Repository:      $REPO_URL
  Install dir:     $INSTALL_DIR
  Base URL:        $BASE_URL
  Certbot email:   $CERTBOT_EMAIL
  Admin email:     $ADMIN_EMAIL
  GitHub Client ID:     ${GITHUB_CLIENT_ID:-<not set>}
SUMMARY

confirm_prompt

INSTALL_PARENT=$(dirname "$INSTALL_DIR")
INSTALL_NAME=$(basename "$INSTALL_DIR")

section "Installing prerequisites"
info "Updating apt cache and installing git/curl"
apt-get update -y >/dev/null
apt-get install -y git curl ca-certificates >/dev/null

section "Fetching Bakery"
mkdir -p "$INSTALL_PARENT"
cd "$INSTALL_PARENT"
if [[ -d "$INSTALL_DIR/.git" ]]; then
  info "Existing checkout detected, syncing latest changes"
  cd "$INSTALL_DIR"
  git fetch --all --tags
  git reset --hard origin/HEAD
else
  info "Cloning $REPO_URL into $INSTALL_DIR"
  rm -rf "$INSTALL_NAME"
  git clone "$REPO_URL" "$INSTALL_NAME"
  cd "$INSTALL_DIR"
fi

section "Running Bakery installer"
ARGS=("--dir" "$INSTALL_DIR" "--repo" "$REPO_URL" "--certbot-email" "$CERTBOT_EMAIL" "--base-url" "$BASE_URL" "--admin-email" "$ADMIN_EMAIL" "--admin-pass" "$ADMIN_PASS" "--github-client-id" "$GITHUB_CLIENT_ID" "--github-client-secret" "$GITHUB_CLIENT_SECRET")
bash infrastructure/scripts/install.sh "${ARGS[@]}"

section "Installation complete"
cat <<INFO
Bakery has been installed at $INSTALL_DIR.
Admin login: $ADMIN_EMAIL
Admin password: $ADMIN_PASS
Control plane URL: $BASE_URL
INFO

info "Remember to configure DNS records for $BASE_URL and any wildcard app domains."
