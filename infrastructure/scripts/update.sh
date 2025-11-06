#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
APP_DIR=$(cd "$SCRIPT_DIR/../.." && pwd)

echo "Updating Bakery in $APP_DIR"

cd "$APP_DIR"
if [ -d .git ]; then
  git pull --rebase
fi

export BUN_INSTALL="$HOME/.bun"
export PATH="$BUN_INSTALL/bin:$PATH"

bun install
cd app && bun install && bun run build && cd ..

bun run migrate

systemctl daemon-reload >/dev/null 2>&1 || true
systemctl restart bakery.service

echo "Bakery updated and restarted."
