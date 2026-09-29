#!/usr/bin/env bash
# The fake server: a systemd Fedora container (bakery-test-server) that
# install.sh runs in for real, with Pebble (bakery-test-pebble, alias
# "pebble") as its ACME CA. Only touches bakery-test-* resources.
#
#   server.sh up | install | down | exec CMD...
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/../../.." && pwd)
WORK=${TMPDIR:-/tmp}/bakery-test-server
SERVER=bakery-test-server
PEBBLE=bakery-test-pebble
NET=bakery-test-net
STORAGE=bakery-test-server-storage
IMAGE=localhost/bakery-test-server:dev
PEBBLE_IMAGE=ghcr.io/letsencrypt/pebble:latest
API_IMAGE=localhost/bakery-api:dev
WEB_IMAGE=localhost/bakery-web:dev
DOMAIN=bakery.test

log() { printf '\033[1m[server]\033[0m %s\n' "$*"; }

up() {
	mkdir -p "$WORK"
	log "Building $IMAGE"
	podman build -q -t "$IMAGE" "$ROOT/infra/dev/server" >/dev/null
	podman network exists "$NET" || podman network create "$NET" >/dev/null

	if ! podman container exists "$PEBBLE"; then
		log "Starting Pebble"
		# Every challenge counts as valid, so no DNS stand-in is needed.
		podman run -d --name "$PEBBLE" --network "$NET" --network-alias pebble \
			--label bakery.test=server \
			-e PEBBLE_VA_ALWAYS_VALID=1 -e PEBBLE_VA_NOSLEEP=1 "$PEBBLE_IMAGE" >/dev/null
	fi
	# Signs Pebble's own HTTPS certificate (for "pebble"), not the
	# certificates it issues.
	podman cp "$PEBBLE:/test/certs/pebble.minica.pem" "$WORK/pebble-minica.pem"

	if ! podman container exists "$SERVER"; then
		log "Starting $SERVER"
		# /var/lib/bakery is a volume: nested overlay storage cannot sit on
		# the container's own overlay root.
		podman run -d --name "$SERVER" --hostname "$SERVER" --privileged --systemd=always \
			--network "$NET" --label bakery.test=server \
			-v "$STORAGE:/var/lib/bakery" "$IMAGE" >/dev/null
	fi
	podman exec "$SERVER" systemctl is-system-running --wait >/dev/null || true
	log "Up"
}

install() {
	for image in "$API_IMAGE" "$WEB_IMAGE"; do
		podman image exists "$image" || { echo "missing $image: run task images" >&2; exit 1; }
	done
	log "Copying images and install.sh into $SERVER"
	rm -f "$WORK/images.tar"
	podman save -q -m "$API_IMAGE" "$WEB_IMAGE" -o "$WORK/images.tar"
	podman cp "$WORK/images.tar" "$SERVER:/root/bakery-images.tar"
	podman cp "$WORK/pebble-minica.pem" "$SERVER:/root/pebble-minica.pem"
	podman cp "$ROOT/infra/install/install.sh" "$SERVER:/root/install.sh"
	log "Running install.sh"
	# --subids: the fake server has only 65536 IDs of its own to hand out.
	podman exec "$SERVER" bash /root/install.sh \
		--domain "$DOMAIN" --email admin@bakery.test \
		--acme-ca https://pebble:14000/dir --acme-ca-root /root/pebble-minica.pem \
		--api-image "$API_IMAGE" --web-image "$WEB_IMAGE" \
		--load /root/bakery-images.tar --no-pull --subids 1000-65536
	podman exec "$SERVER" rm -f /root/bakery-images.tar
}

down() {
	log "Removing the fake server"
	podman rm -f -t 5 "$SERVER" "$PEBBLE" >/dev/null 2>&1 || true
	podman volume rm -f "$STORAGE" >/dev/null 2>&1 || true
	podman network rm -f "$NET" >/dev/null 2>&1 || true
	rm -rf "$WORK"
}

case ${1:-} in
up) up ;;
install) install ;;
down) down ;;
exec) shift; podman exec -i "$SERVER" "$@" ;;
*) echo "usage: $0 up|install|down|exec CMD..." >&2; exit 2 ;;
esac
