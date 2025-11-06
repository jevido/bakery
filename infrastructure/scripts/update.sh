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

if [[ -x "/usr/local/bin/bun" ]]; then
  BUN_TARGET=$(readlink -f /usr/local/bin/bun 2>/dev/null || true)
  if [[ -n "$BUN_TARGET" && "$BUN_TARGET" == /root/* ]]; then
    install -m 755 "$BUN_TARGET" /usr/local/bin/bun
  fi
elif [[ -x "$BUN_INSTALL/bin/bun" ]]; then
  install -m 755 "$BUN_INSTALL/bin/bun" /usr/local/bin/bun
fi

bun --bun install
cd app && bun --bun install && bun run build && cd ..

chmod 755 app/build/index.js

bun run migrate
sed "s|{{WORKING_DIR}}|$APP_DIR|g" infrastructure/systemd/bakery.service > /etc/systemd/system/bakery.service

systemctl daemon-reload >/dev/null 2>&1 || true
systemctl restart bakery.service

echo "Bakery updated and restarted."
