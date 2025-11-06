#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
APP_DIR=$(cd "$SCRIPT_DIR/../.." && pwd)

echo "Updating Bakery in $APP_DIR"

cd "$APP_DIR"
if [ -d .git ]; then
  git config --global --add safe.directory "$APP_DIR" >/dev/null 2>&1 || true
  git pull --rebase
fi

BUN_INSTALL_ROOT="/usr/local/lib/bun"
export BUN_INSTALL="${BUN_INSTALL:-$BUN_INSTALL_ROOT}"
export PATH="/usr/local/bin:$BUN_INSTALL/bin:$PATH"

bun --bun install
cd app && bun --bun install && bun run build && cd ..

bun run migrate
sed "s|{{WORKING_DIR}}|$APP_DIR|g" infrastructure/systemd/bakery.service > /etc/systemd/system/bakery.service

systemctl daemon-reload >/dev/null 2>&1 || true
systemctl restart bakery.service

echo "Bakery updated and restarted."
