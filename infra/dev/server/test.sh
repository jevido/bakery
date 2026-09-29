#!/usr/bin/env bash
# task server:test — install Bakery in the fake server and check it end to
# end: fresh install, reboot, re-install. Cleans up on exit.
set -euo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
SERVER=bakery-test-server
S="$HERE/server.sh"

log() { printf '\n\033[1m[test]\033[0m %s\n' "$*"; }
trap '"$S" down' EXIT

OWNER_EMAIL=owner@bakery.test
OWNER_PASSWORD=$(head -c 32 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c 24)

checks() {
	podman cp "$HERE/checks.sh" "$SERVER:/root/checks.sh"
	podman exec -e OWNER_EMAIL="$OWNER_EMAIL" -e OWNER_PASSWORD="$OWNER_PASSWORD" "$SERVER" bash /root/checks.sh "$1"
}

"$S" down
"$S" up
log "Fresh install"
"$S" install
checks fresh

log "Reboot"
podman restart -t 30 "$SERVER" >/dev/null
podman exec "$SERVER" systemctl is-system-running --wait >/dev/null || true
checks again

log "Re-install"
"$S" install
checks again

log "server:test passed"
