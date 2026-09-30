#!/usr/bin/env bash
# Runs inside bakery-test-server (copied there by test.sh). Checks one stage
# of the install from the server's own point of view:
#
#   checks.sh fresh      after the first install: TLS, dashboard, setup, deploy
#   checks.sh again      after a reboot or a re-install: everything came back
#
# OWNER_EMAIL and OWNER_PASSWORD come from the environment; never printed.
set -euo pipefail

DOMAIN=bakery.test
APP_DOMAIN=whoami.bakery.test
JAR=/root/bakery-jar
ROOTS=/root/issuing-root.pem
FAILED=0

pass() { printf '  \033[32mok\033[0m   %s\n' "$*"; }
fail() {
	printf '  \033[31mFAIL\033[0m %s\n' "$*"
	FAILED=1
}

# expect DESCRIPTION CMD...: pass when CMD succeeds.
expect() {
	local what=$1
	shift
	if "$@"; then pass "$what"; else fail "$what"; fi
}
# shellcheck disable=SC2329 # called through expect
contains() { grep -q -- "$2" <<<"$1"; }
# shellcheck disable=SC2329
not() { ! "$@"; }

# Pebble's issuing root; distinct from the minica root of its own HTTPS.
curl -fsS --cacert /root/pebble-minica.pem https://pebble:15000/roots/0 >"$ROOTS"

# Strict TLS against Pebble's root: only a Pebble-issued certificate passes.
c() { curl -sS --max-time 10 --cacert "$ROOTS" --resolve "$DOMAIN:443:127.0.0.1" --resolve "$APP_DOMAIN:443:127.0.0.1" "$@"; }
api() { c -b "$JAR" -c "$JAR" -H 'content-type: application/json' "$@"; }

as_bakery() { (cd /var/lib/bakery && runuser -u bakery -- env XDG_RUNTIME_DIR="/run/user/$(id -u bakery)" "$@"); }

wait_for() { # wait_for SECONDS CMD...
	local deadline=$((SECONDS + $1))
	shift
	until "$@" >/dev/null 2>&1; do
		[ $SECONDS -lt "$deadline" ] || return 1
		sleep 2
	done
}

check_running() {
	local want=(bakery-postgres bakery-api bakery-web bakery-proxy)
	[ "${1:-}" = with-app ] && want+=(bakery-app-)
	local running
	running=$(as_bakery podman ps --format '{{.Names}}')
	for name in "${want[@]}"; do
		expect "$name runs as bakery" contains "$running" "^$name"
	done
}

check_platform() {
	if wait_for 120 c -f "https://$DOMAIN/api/health"; then
		expect "/api/health answers" [ "$(c "https://$DOMAIN/api/health")" = '{"ok":true}' ]
	else
		fail "https://$DOMAIN/api/health never answered with a trusted certificate"
		return
	fi
	expect "dashboard served on https://$DOMAIN" contains "$(c "https://$DOMAIN/")" '<div id="app"'
	local issuer
	issuer=$(echo | openssl s_client -connect 127.0.0.1:443 -servername "$DOMAIN" 2>/dev/null | openssl x509 -noout -issuer)
	expect "certificate issued by Pebble ($issuer)" contains "$issuer" Pebble
	local redirect
	redirect=$(curl -s -o /dev/null -w '%{http_code} %{redirect_url}' --resolve "$DOMAIN:80:127.0.0.1" "http://$DOMAIN/")
	expect "http redirects to https ($redirect)" [ "$redirect" = "308 https://$DOMAIN/" ]
	expect "admin API not published" not contains "$(as_bakery podman port bakery-proxy)" 2019
	expect "admin API unreachable from the host" not curl -s --max-time 2 -o /dev/null http://127.0.0.1:2019/config/
}

setup_owner() {
	[ "$(c "https://$DOMAIN/api/setup")" = '{"needed":true}' ] || { fail "setup not offered on a fresh install"; return; }
	local headers
	headers=$(api -D - -o /dev/null -d "{\"name\":\"Owner\",\"email\":\"$OWNER_EMAIL\",\"password\":\"$OWNER_PASSWORD\"}" "https://$DOMAIN/api/setup")
	expect "setup sets a Secure session cookie" contains "$headers" '^[Ss]et-[Cc]ookie: bakery_session=.*[Ss]ecure'
	expect "session works through the proxy" contains "$(api "https://$DOMAIN/api/me")" "$OWNER_EMAIL"
}

login() {
	rm -f "$JAR"
	expect "owner can still log in" api -f -o /dev/null -d "{\"email\":\"$OWNER_EMAIL\",\"password\":\"$OWNER_PASSWORD\"}" "https://$DOMAIN/api/login"
}

deploy_whoami() {
	local project env app deployment status=
	project=$(api -f -d '{"name":"Server test"}' "https://$DOMAIN/api/projects" | jq -r .project.id)
	env=$(api -f "https://$DOMAIN/api/projects/$project" | jq -r '.project.environments[0].id')
	app=$(api -f -d "{\"name\":\"whoami\",\"git_url\":\"https://github.com/traefik/whoami\",\"git_branch\":\"master\",\"port\":80,\"domains\":[\"$APP_DOMAIN\"]}" \
		"https://$DOMAIN/api/environments/$env/applications" | jq -r .application.id)
	api -f -o /dev/null -X PUT -d '{"env":[{"name":"HELLO","value":"from-bakery"}]}' "https://$DOMAIN/api/applications/$app/env"
	deployment=$(api -f -X POST "https://$DOMAIN/api/applications/$app/deploy" | jq -r .deployment.id)
	echo "$app" >/root/bakery-app-id
	for _ in $(seq 150); do
		status=$(api -f "https://$DOMAIN/api/deployments/$deployment" | jq -r .deployment.status)
		[ "$status" = finished ] || [ "$status" = failed ] && break
		sleep 4
	done
	if [ "$status" != finished ]; then
		fail "deployment ended $status: $(api "https://$DOMAIN/api/deployments/$deployment" | jq -r .deployment.error)"
		return
	fi
	pass "whoami deployed"
	check_app
}

check_app() {
	if wait_for 90 c -f "https://$APP_DOMAIN/"; then
		expect "https://$APP_DOMAIN answers with a Pebble certificate" contains "$(c "https://$APP_DOMAIN/")" '^Hostname:'
	else
		fail "https://$APP_DOMAIN never answered"
	fi
}

check_app_listed() {
	local app
	app=$(cat /root/bakery-app-id)
	expect "whoami and its deployment are still there" [ "$(api "https://$DOMAIN/api/applications/$app/deployments" | jq '.deployments | length')" -ge 1 ]
}

case ${1:-} in
fresh)
	check_running
	check_platform
	setup_owner
	deploy_whoami
	;;
again)
	check_platform
	check_running with-app
	login
	check_app_listed
	check_app
	;;
*) echo "usage: $0 fresh|again" >&2; exit 2 ;;
esac
exit $FAILED
