#!/usr/bin/env bash
# Installs Bakery on this server, or upgrades it when run again.
#
#   sudo bash install.sh --domain bakery.example.com --email me@example.com
#
# Every step checks before it changes, so re-running is safe: secrets and
# data are kept, images are pulled again and bakery-api, bakery-web and
# bakery-proxy are recreated. See infra/install/README.md.
set -euo pipefail

BAKERY_USER=bakery
BAKERY_HOME=/var/lib/bakery
ENV_FILE=$BAKERY_HOME/bakery.env
NETWORK=bakery

DOMAIN=
EMAIL=
API_IMAGE=ghcr.io/jevido/bakery-api:latest
WEB_IMAGE=ghcr.io/jevido/bakery-web:latest
POSTGRES_IMAGE=docker.io/library/postgres:18
ACME_CA=
ACME_CA_ROOT=
PULL=true
LOAD=

usage() {
	cat <<USAGE
Usage: install.sh --domain DOMAIN --email EMAIL [options]

  --domain DOMAIN      domain the dashboard is served on (required)
  --email EMAIL        ACME account email (required unless --acme-ca is set)
  --api-image REF      default $API_IMAGE
  --web-image REF      default $WEB_IMAGE
  --acme-ca URL        ACME directory (default: Let's Encrypt)
  --acme-ca-root FILE  PEM root the ACME directory's HTTPS certificate is signed by
  --load FILE          podman-load images from this archive first
  --no-pull            use images already present instead of pulling
USAGE
}

log() { printf '\033[1m==>\033[0m %s\n' "$*"; }
die() {
	printf 'install.sh: %s\n' "$*" >&2
	exit 1
}

parse_args() {
	while [ $# -gt 0 ]; do
		case $1 in
		--domain) DOMAIN=${2:?}; shift ;;
		--email) EMAIL=${2:?}; shift ;;
		--api-image) API_IMAGE=${2:?}; shift ;;
		--web-image) WEB_IMAGE=${2:?}; shift ;;
		--acme-ca) ACME_CA=${2:?}; shift ;;
		--acme-ca-root) ACME_CA_ROOT=${2:?}; shift ;;
		--load) LOAD=${2:?}; shift ;;
		--no-pull) PULL=false ;;
		-h | --help) usage; exit 0 ;;
		*) usage >&2; die "unknown option: $1" ;;
		esac
		shift
	done
	[ -n "$DOMAIN" ] || { usage >&2; die "--domain is required"; }
	[ -n "$EMAIL" ] || [ -n "$ACME_CA" ] || die "--email is required (Let's Encrypt needs an account email)"
	[ -z "$ACME_CA_ROOT" ] || [ -f "$ACME_CA_ROOT" ] || die "--acme-ca-root $ACME_CA_ROOT: no such file"
	[ -z "$LOAD" ] || [ -f "$LOAD" ] || die "--load $LOAD: no such file"
}

install_podman() {
	if command -v podman >/dev/null; then
		return
	fi
	log "Installing Podman"
	if command -v dnf >/dev/null; then
		dnf install -y podman passt shadow-utils
	elif command -v apt-get >/dev/null; then
		apt-get update
		DEBIAN_FRONTEND=noninteractive apt-get install -y podman passt uidmap dbus-user-session
	else
		die "no dnf or apt-get: install Podman yourself and run this again"
	fi
}

# Rootless containers may then bind 80 and 443.
allow_low_ports() {
	local conf=/etc/sysctl.d/90-bakery.conf
	if [ "$(sysctl -n net.ipv4.ip_unprivileged_port_start)" -le 80 ] && [ -f "$conf" ]; then
		return
	fi
	log "Allowing unprivileged ports from 80"
	echo 'net.ipv4.ip_unprivileged_port_start=80' >"$conf"
	sysctl -q -p "$conf"
}

create_user() {
	if ! id "$BAKERY_USER" >/dev/null 2>&1; then
		log "Creating user $BAKERY_USER"
		useradd --system --create-home --home-dir "$BAKERY_HOME" --shell /usr/sbin/nologin "$BAKERY_USER"
	fi
	# Rootless Podman maps container users onto these ranges.
	if ! grep -q "^$BAKERY_USER:" /etc/subuid; then
		usermod --add-subuids 200000-265535 "$BAKERY_USER"
	fi
	if ! grep -q "^$BAKERY_USER:" /etc/subgid; then
		usermod --add-subgids 200000-265535 "$BAKERY_USER"
	fi
	BAKERY_UID=$(id -u "$BAKERY_USER")
	RUNTIME_DIR=/run/user/$BAKERY_UID
	# Linger keeps the user's systemd (and so its containers) running
	# without a login, and starts it at boot.
	loginctl enable-linger "$BAKERY_USER"
	for _ in $(seq 60); do
		[ -S "$RUNTIME_DIR/bus" ] && return
		sleep 1
	done
	die "the systemd user instance of $BAKERY_USER did not start"
}

# as_bakery runs a command as the bakery user with its systemd session.
as_bakery() {
	(cd "$BAKERY_HOME" && runuser -u "$BAKERY_USER" -- env \
		HOME="$BAKERY_HOME" XDG_RUNTIME_DIR="$RUNTIME_DIR" \
		DBUS_SESSION_BUS_ADDRESS="unix:path=$RUNTIME_DIR/bus" "$@")
}

enable_user_units() {
	log "Enabling podman.socket and podman-restart.service for $BAKERY_USER"
	# podman-restart starts every container with --restart=always at boot.
	as_bakery systemctl --user enable --now podman.socket
	as_bakery systemctl --user enable podman-restart.service
}

random() { head -c 48 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c "$1"; }

# Secrets are generated once and kept on every later run.
write_secrets() {
	if [ -f "$ENV_FILE" ]; then
		return
	fi
	log "Generating secrets in $ENV_FILE"
	(
		umask 077
		{
			echo "APP_KEY=$(random 32)"
			echo "JWT_SECRET=$(random 32)"
			echo "DB_PASSWORD=$(random 32)"
		} >"$ENV_FILE"
	)
	chown "$BAKERY_USER:" "$ENV_FILE"
}

db_password() { sed -n 's/^DB_PASSWORD=//p' "$ENV_FILE"; }

container_exists() { as_bakery podman container exists "$1"; }

prepare_images() {
	if [ -n "$LOAD" ]; then
		log "Loading images from $LOAD"
		as_bakery podman load --quiet <"$LOAD"
	fi
	if [ "$PULL" = true ]; then
		log "Pulling images"
		for image in "$POSTGRES_IMAGE" "$API_IMAGE" "$WEB_IMAGE"; do
			as_bakery podman pull --quiet "$image" >/dev/null
		done
	fi
	for image in "$API_IMAGE" "$WEB_IMAGE"; do
		as_bakery podman image exists "$image" || die "image $image is not present (drop --no-pull or --load it)"
	done
}

start_postgres() {
	as_bakery podman network exists "$NETWORK" || as_bakery podman network create "$NETWORK" >/dev/null
	# Never recreated: it holds the data, and it does not change with Bakery.
	if ! container_exists bakery-postgres; then
		log "Starting bakery-postgres"
		as_bakery podman run -d --name bakery-postgres --network "$NETWORK" \
			--restart=always --label bakery.managed=true --label bakery.role=database \
			-e POSTGRES_USER=bakery -e POSTGRES_DB=bakery -e POSTGRES_PASSWORD="$(db_password)" \
			-v bakery-postgres:/var/lib/postgresql \
			"$POSTGRES_IMAGE" >/dev/null
	else
		as_bakery podman start bakery-postgres >/dev/null
	fi
	for _ in $(seq 60); do
		as_bakery podman exec bakery-postgres pg_isready -U bakery >/dev/null 2>&1 && return
		sleep 1
	done
	die "bakery-postgres did not become ready"
}

start_bakery() {
	local root_mount=()
	local acme_root=
	if [ -n "$ACME_CA_ROOT" ]; then
		install -m 644 -o "$BAKERY_USER" "$ACME_CA_ROOT" "$BAKERY_HOME/acme-root.pem"
		root_mount=(-v "$BAKERY_HOME/acme-root.pem:/etc/bakery/acme-root.pem:ro")
		acme_root=/etc/bakery/acme-root.pem
	fi
	# The API creates bakery-proxy itself on start; removing it here makes
	# it come back with this run's settings and image. Its volumes (the
	# certificates) stay.
	for name in bakery-api bakery-web bakery-proxy; do
		if container_exists "$name"; then
			log "Removing $name for recreation"
			as_bakery podman rm -f "$name" >/dev/null
		fi
	done

	log "Starting bakery-web"
	as_bakery podman run -d --name bakery-web --network "$NETWORK" \
		--restart=always --label bakery.managed=true --label bakery.role=web \
		"$WEB_IMAGE" >/dev/null

	log "Starting bakery-api"
	# The API drives the bakery user's own Podman through its socket; root
	# in this container is that user, so it gains nothing beyond it.
	as_bakery podman run -d --name bakery-api --network "$NETWORK" \
		--restart=always --label bakery.managed=true --label bakery.role=api \
		--security-opt label=disable \
		-v "$RUNTIME_DIR/podman/podman.sock:/run/podman/podman.sock" \
		"${root_mount[@]}" \
		--env-file "$ENV_FILE" \
		-e APP_URL="https://$DOMAIN" \
		-e DB_HOST=bakery-postgres -e DB_PORT=5432 -e DB_DATABASE=bakery -e DB_USERNAME=bakery \
		-e BAKERY_NETWORK="$NETWORK" \
		-e BAKERY_DOMAIN_SUFFIX="$DOMAIN" \
		-e BAKERY_DASHBOARD_DOMAIN="$DOMAIN" \
		-e BAKERY_PROXY_HTTP_PORT=80 -e BAKERY_PROXY_HTTPS_PORT=443 \
		-e BAKERY_PROXY_INTERNAL_TLS=false \
		-e BAKERY_PROXY_ADMIN_URL=http://bakery-proxy:2019 \
		-e BAKERY_ACME_CA="$ACME_CA" -e BAKERY_ACME_EMAIL="$EMAIL" -e BAKERY_ACME_CA_ROOT="$acme_root" \
		"$API_IMAGE" >/dev/null
}

wait_ready() {
	log "Waiting for https://$DOMAIN"
	for _ in $(seq 120); do
		# -k: only asks whether it is up; the certificate may still be on its way.
		if curl -fsk --max-time 3 --resolve "$DOMAIN:443:127.0.0.1" "https://$DOMAIN/api/health" >/dev/null 2>&1; then
			log "Bakery is running at https://$DOMAIN"
			return
		fi
		sleep 2
	done
	as_bakery podman logs --tail 30 bakery-api >&2 || true
	die "https://$DOMAIN/api/health did not answer; see the log above"
}

main() {
	[ "$(id -u)" -eq 0 ] || die "run as root (sudo bash install.sh ...)"
	parse_args "$@"
	command -v curl >/dev/null || die "curl is required"
	install_podman
	allow_low_ports
	create_user
	enable_user_units
	write_secrets
	prepare_images
	start_postgres
	start_bakery
	wait_ready
}

main "$@"
