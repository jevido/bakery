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
  --debug                   Verbose logging / trace mode

Example:
  $(basename "$0")     --host bakery.jevido.nl     --repo https://github.com/your-org/bakery.git     --certbot-email you@example.com     --base-url https://bakery.jevido.nl
USAGE
}

is_local_host() {
  local target="$1"
  if [[ -z "$target" ]]; then
    return 1
  fi

  case "$target" in
    localhost|127.0.0.1|::1)
      return 0
      ;;
  esac

  local host_short
  host_short=$(hostname 2>/dev/null || true)
  if [[ -n "$host_short" && "$target" == "$host_short" ]]; then
    return 0
  fi

  local host_fqdn
  host_fqdn=$(hostname -f 2>/dev/null || true)
  if [[ -n "$host_fqdn" && "$target" == "$host_fqdn" ]]; then
    return 0
  fi

  if command -v getent >/dev/null 2>&1 && command -v ip >/dev/null 2>&1; then
    local -a host_ips
    mapfile -t host_ips < <(getent hosts "$target" 2>/dev/null | awk '{print $1}')
    if [[ ${#host_ips[@]} -gt 0 ]]; then
      local -a local_ips
      mapfile -t local_ips < <(ip -o addr show 2>/dev/null | awk '{split($4, a, "/"); print a[1]}')
      for hip in "${host_ips[@]}"; do
        for lip in "${local_ips[@]}"; do
          if [[ "$hip" == "$lip" ]]; then
            return 0
          fi
        done
      done
    fi
  fi

  return 1
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
BOOTSTRAP_DEBUG_VALUE="${BOOTSTRAP_DEBUG:-}"
DEBUG_MODE=false

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
    --debug)
      DEBUG_MODE=true
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

if $DEBUG_MODE; then
  BOOTSTRAP_DEBUG_VALUE=1
fi

LOCAL_EXEC=false
if is_local_host "$HOST"; then
  LOCAL_EXEC=true
fi

SSH_TARGET="${SSH_USER}@${HOST}"
SSH_CMD=(ssh -p "$SSH_PORT")
if [[ -n "$SSH_IDENTITY" ]]; then
  SSH_CMD+=(-i "$SSH_IDENTITY")
fi

INSTALL_PARENT=$(dirname "$INSTALL_DIR")
INSTALL_NAME=$(basename "$INSTALL_DIR")

REMOTE_SCRIPT=$(cat <<SCRIPT
set -euo pipefail
BOOTSTRAP_DEBUG="${BOOTSTRAP_DEBUG_VALUE}"
REPO_URL="${REPO_URL}"
INSTALL_DIR="${INSTALL_DIR}"
CERTBOT_EMAIL="${CERTBOT_EMAIL}"
BASE_URL="${BASE_URL}"
ADMIN_EMAIL="${ADMIN_EMAIL}"
ADMIN_PASS="${ADMIN_PASS}"
GITHUB_CLIENT_ID="${GITHUB_CLIENT_ID}"
GITHUB_CLIENT_SECRET="${GITHUB_CLIENT_SECRET}"

if [[ -n "\${BOOTSTRAP_DEBUG}" ]]; then
  set -x
  echo "[debug] bootstrap remote script starting in \$(pwd)"
fi

INSTALL_PARENT="${INSTALL_PARENT}"
INSTALL_NAME="${INSTALL_NAME}"
mkdir -p "$INSTALL_PARENT"
cd "$INSTALL_PARENT"

if ! command -v git >/dev/null 2>&1; then
  apt-get update -y >/dev/null
  apt-get install -y git >/dev/null
fi

if [[ -n "\${BOOTSTRAP_DEBUG}" ]]; then
  echo "[debug] after cd: \$(pwd)"
  ls -lad .
fi

if command -v git >/dev/null 2>&1; then
  git config --global --add safe.directory "$INSTALL_DIR" >/dev/null 2>&1 || true
fi

if [[ ! -d "$INSTALL_DIR/.git" ]]; then
  rm -rf "$INSTALL_NAME"
  if [[ -n "\${BOOTSTRAP_DEBUG}" ]]; then
    echo "[debug] cloning into $INSTALL_NAME from \$(pwd)"
  fi
  git clone "$REPO_URL" "$INSTALL_NAME"
else
  cd "$INSTALL_DIR"
  git remote set-url origin "$REPO_URL"
  git fetch --all
  git reset --hard origin/HEAD
fi

cd "$INSTALL_DIR"
if [[ -n "\${BOOTSTRAP_DEBUG}" ]]; then
  echo "[debug] entered install dir: \$(pwd)"
fi
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

infrastructure/scripts/install.sh "\${ARGS[@]}"
SCRIPT
)

if $DRY_RUN; then
  if $LOCAL_EXEC; then
    echo "# Commands that would be executed locally:" >&2
    echo "$REMOTE_SCRIPT" >&2
  else
    echo "# Commands that would be executed:" >&2
    printf '%q ' "${SSH_CMD[@]}" "$SSH_TARGET" "bash -s" >&2
    echo >&2
    echo "$REMOTE_SCRIPT" >&2
  fi
  exit 0
fi

if $LOCAL_EXEC; then
  echo "Target host ${HOST} resolves to this machine; running install locally."
  bash -s <<REMOTE
$REMOTE_SCRIPT
REMOTE
else
  "${SSH_CMD[@]}" "$SSH_TARGET" "bash -s" <<REMOTE
$REMOTE_SCRIPT
REMOTE
fi

echo "Bakery control plane provisioned on ${HOST}."
if [[ -z "$BASE_URL" ]]; then
  echo "Tip: pass --base-url to set the public URL in .env" >&2
fi
