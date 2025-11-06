#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: $(basename "$0") --host <host> --repo <url> [options]

Required:
  --host HOSTNAME           Remote host (e.g. hetzner.example.com)
  --repo URL                Git repository URL containing Bakery

Optional:
  --ssh-user USER           SSH user (default: root)
  --ssh-port PORT           SSH port (default: 22)
  --install-dir PATH        Target directory on remote server (default: /opt/bakery)
  --certbot-email EMAIL     Email for certbot registrations
  --base-url URL            Public base URL for Bakery (e.g. https://bakery.example.com)
  --admin-email EMAIL       Initial admin email
  --admin-pass PASSWORD     Initial admin password (random if omitted)
  --ssh-identity PATH       SSH identity file passed to ssh
  --github-client-id ID     GitHub OAuth app client ID
  --github-client-secret SECRET GitHub OAuth app client secret
  --dry-run                 Show the commands without executing them

Example:
  $(basename "$0")     --host bakery.jevido.nl     --repo https://github.com/your-org/bakery.git     --certbot-email you@example.com     --base-url https://bakery.jevido.nl
USAGE
}

HOST=""
REPO_URL=""
SSH_USER="root"
SSH_PORT="22"
INSTALL_DIR="/opt/bakery"
CERTBOT_EMAIL=""
BASE_URL=""
ADMIN_EMAIL=""
ADMIN_PASS=""
GITHUB_CLIENT_ID=""
GITHUB_CLIENT_SECRET=""
SSH_IDENTITY=""
DRY_RUN=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --host)
      HOST="$2"
      shift 2
      ;;
    --repo)
      REPO_URL="$2"
      shift 2
      ;;
    --ssh-user)
      SSH_USER="$2"
      shift 2
      ;;
    --ssh-port)
      SSH_PORT="$2"
      shift 2
      ;;
    --install-dir)
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
    --ssh-identity)
      SSH_IDENTITY="$2"
      shift 2
      ;;
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage
      exit 1
      ;;
  esac
done

if [[ -z "$HOST" || -z "$REPO_URL" ]]; then
  echo "--host and --repo are required" >&2
  usage
  exit 1
fi

SSH_TARGET="${SSH_USER}@${HOST}"
SSH_CMD=(ssh -p "$SSH_PORT")
if [[ -n "$SSH_IDENTITY" ]]; then
  SSH_CMD+=(-i "$SSH_IDENTITY")
fi

read -r -d '' REMOTE_SCRIPT <<SCRIPT
set -euo pipefail
REPO_URL="${REPO_URL}"
INSTALL_DIR="${INSTALL_DIR}"
CERTBOT_EMAIL="${CERTBOT_EMAIL}"
BASE_URL="${BASE_URL}"
ADMIN_EMAIL="${ADMIN_EMAIL}"
ADMIN_PASS="${ADMIN_PASS}"
GITHUB_CLIENT_ID="${GITHUB_CLIENT_ID}"
GITHUB_CLIENT_SECRET="${GITHUB_CLIENT_SECRET}"

if ! command -v git >/dev/null 2>&1; then
  apt-get update -y >/dev/null
  apt-get install -y git >/dev/null
fi

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
ARGS=("--dir" "$INSTALL_DIR" "--repo" "$REPO_URL")
if [[ -n "$CERTBOT_EMAIL" ]]; then
  ARGS+=("--certbot-email" "$CERTBOT_EMAIL")
fi
if [[ -n "$BASE_URL" ]]; then
  ARGS+=("--base-url" "$BASE_URL")
fi
if [[ -n "$ADMIN_EMAIL" ]]; then
  ARGS+=("--admin-email" "$ADMIN_EMAIL")
fi
if [[ -n "$ADMIN_PASS" ]]; then
  ARGS+=("--admin-pass" "$ADMIN_PASS")
fi
if [[ -n "$GITHUB_CLIENT_ID" ]]; then
  ARGS+=("--github-client-id" "$GITHUB_CLIENT_ID")
fi
if [[ -n "$GITHUB_CLIENT_SECRET" ]]; then
  ARGS+=("--github-client-secret" "$GITHUB_CLIENT_SECRET")
fi

infrastructure/scripts/install.sh "${ARGS[@]}"
SCRIPT

if $DRY_RUN; then
  echo "# Commands that would be executed:" >&2
  printf '%q ' "${SSH_CMD[@]}" "$SSH_TARGET" "bash -s" >&2
  echo >&2
  echo "$REMOTE_SCRIPT" >&2
  exit 0
fi

"${SSH_CMD[@]}" "$SSH_TARGET" "bash -s" <<REMOTE
$REMOTE_SCRIPT
REMOTE

echo "Bakery control plane provisioned on ${HOST}."
if [[ -z "$BASE_URL" ]]; then
  echo "Tip: pass --base-url to set the public URL in .env" >&2
fi
