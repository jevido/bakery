#!/usr/bin/env bash
set -euo pipefail

REPO_URL="https://github.com/the-bakery-app/bakery.git"
INSTALL_DIR="/opt/bakery"
CERTBOT_EMAIL=""
BASE_URL=""
ADMIN_EMAIL=""
ADMIN_PASS=""
GITHUB_CLIENT_ID=""
GITHUB_CLIENT_SECRET=""

print_usage() {
  cat <<'USAGE'
Usage: curl -fsSL https://raw.githubusercontent.com/the-bakery-app/bakery/main/scripts/install-control-plane.sh | sudo bash -s -- [options]

Options:
  --repo URL                    Alternate Bakery repository URL
  --dir PATH                    Target install directory (default: /opt/bakery)
  --certbot-email EMAIL         Email for certbot registrations
  --base-url URL                Public base URL for Bakery (e.g. https://bakery.example.com)
  --admin-email EMAIL           Initial admin email (default from installer)
  --admin-pass PASSWORD         Initial admin password (random if omitted)
  --github-client-id ID         GitHub OAuth app client ID
  --github-client-secret SECRET GitHub OAuth app client secret
    --help                        Show this help message
USAGE
}

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
    -h|--help)
      print_usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      print_usage
      exit 1
      ;;
  esac
done

if [[ $EUID -ne 0 ]]; then
  echo "This script must be run as root (use sudo)." >&2
  exit 1
fi

apt-get update -y
apt-get install -y git curl

if [[ ! -d "$INSTALL_DIR/.git" ]]; then
  rm -rf "$INSTALL_DIR"
  git clone "$REPO_URL" "$INSTALL_DIR"
else
  cd "$INSTALL_DIR"
  git remote set-url origin "$REPO_URL"
  git fetch --all
  git reset --hard origin/HEAD
fi

cd "$INSTALL_DIR"

ARGS=(--dir "$INSTALL_DIR" --repo "$REPO_URL")
if [[ -n "$CERTBOT_EMAIL" ]]; then
  ARGS+=(--certbot-email "$CERTBOT_EMAIL")
fi
if [[ -n "$BASE_URL" ]]; then
  ARGS+=(--base-url "$BASE_URL")
fi
if [[ -n "$ADMIN_EMAIL" ]]; then
  ARGS+=(--admin-email "$ADMIN_EMAIL")
fi
if [[ -n "$ADMIN_PASS" ]]; then
  ARGS+=(--admin-pass "$ADMIN_PASS")
fi
if [[ -n "$GITHUB_CLIENT_ID" ]]; then
  ARGS+=(--github-client-id "$GITHUB_CLIENT_ID")
fi
if [[ -n "$GITHUB_CLIENT_SECRET" ]]; then
  ARGS+=(--github-client-secret "$GITHUB_CLIENT_SECRET")
fi
infrastructure/scripts/install.sh "${ARGS[@]}"
