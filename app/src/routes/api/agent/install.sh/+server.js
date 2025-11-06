const defaultRepo =
	process.env.BAKERY_INSTALL_REPO || 'https://github.com/the-bakery-app/bakery.git';

export const GET = async () => {
	const script = `#!/usr/bin/env bash
set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  echo "This installer must be run as root" >&2
  exit 1
fi

TOKEN=""
API_BASE=""
REPO="${defaultRepo}"
INSTALL_DIR="/opt/bakery-agent"
ROOT_DIR="/var/lib/bakery-node"
LOG_DIR="/var/log/bakery-agent"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --token)
      TOKEN="$2"
      shift 2
      ;;
    --api)
      API_BASE="$2"
      shift 2
      ;;
    --repo)
      REPO="$2"
      shift 2
      ;;
    --dir)
      INSTALL_DIR="$2"
      shift 2
      ;;
    --root)
      ROOT_DIR="$2"
      shift 2
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 1
      ;;
  esac
done

if [[ -z "$TOKEN" ]]; then
  echo "--token is required" >&2
  exit 1
fi

if [[ -z "$API_BASE" ]]; then
  echo "--api is required" >&2
  exit 1
fi

API_BASE="\${API_BASE%/}"

SYSTEM_USER="bakery-agent"
if ! id "$SYSTEM_USER" >/dev/null 2>&1; then
  useradd --system --create-home --shell /usr/sbin/nologin "$SYSTEM_USER"
fi

mkdir -p "$ROOT_DIR" "$LOG_DIR"
chown "$SYSTEM_USER:$SYSTEM_USER" "$ROOT_DIR" "$LOG_DIR"

echo "[1/7] Installing system dependencies"
apt-get update -y
apt-get install -y git curl jq nginx docker.io certbot python3-certbot-nginx build-essential

if ! command -v bun >/dev/null 2>&1; then
  echo "[2/7] Installing Bun runtime"
  curl -fsSL https://bun.sh/install | bash
  export BUN_INSTALL="$HOME/.bun"
  ln -sf "$BUN_INSTALL/bin/bun" /usr/local/bin/bun
else
  export BUN_INSTALL="\${BUN_INSTALL:-$HOME/.bun}"
fi
export PATH="$BUN_INSTALL/bin:$PATH"

PUBLIC_IP=$(curl -fsSL https://api.ipify.org || echo "unknown")

echo "[3/7] Fetching Bakery source"
rm -rf "$INSTALL_DIR"
git clone "$REPO" "$INSTALL_DIR"
cd "$INSTALL_DIR"

echo "[4/7] Installing dependencies"
bun install

mkdir -p "$ROOT_DIR/data" "$ROOT_DIR/logs" "$ROOT_DIR/builds"
chown -R "$SYSTEM_USER:$SYSTEM_USER" "$ROOT_DIR"

echo "[5/7] Registering node with control plane"
REGISTER_PAYLOAD=$(cat <<JSON
{"token":"$TOKEN","hostname":"$(hostname)","platform":"$(uname -s)","arch":"$(uname -m)","version":"$(bun --version 2>/dev/null || echo unknown)","publicIp":"$PUBLIC_IP"}
JSON
)
REGISTER_RESPONSE=$(curl -fsSL -H "Content-Type: application/json" -X POST "$API_BASE/api/agent/register" -d "$REGISTER_PAYLOAD") || {
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

cat > "$INSTALL_DIR/.env.agent" <<ENV
NODE_ENV=production
BAKERY_AGENT_API=$API_BASE
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

echo "[6/7] Configuring systemd service"
SERVICE_PATH="/etc/systemd/system/bakery-agent.service"
sed \
  -e "s|{{WORKING_DIR}}|$INSTALL_DIR|g" \
  -e "s|{{ENV_FILE}}|$INSTALL_DIR/.env.agent|g" \
  "$INSTALL_DIR/infrastructure/systemd/bakery-agent.service" > "$SERVICE_PATH"
chmod 644 "$SERVICE_PATH"

systemctl daemon-reload
systemctl enable bakery-agent.service
systemctl restart bakery-agent.service

cat <<INFO
[7/7] Bakery agent installed

Node ID: $NODE_ID
Pairing code: $PAIRING_CODE

Paste the pairing code into the Bakery UI to activate this node.
INFO
`;
	return new Response(script, {
		headers: {
			'content-type': 'text/x-shellscript'
		}
	});
};
