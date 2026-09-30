#!/usr/bin/env bash
# End to end: Servers. The Local server is reachable with metrics; a Remote
# server (the stand-in from `task remote:up`, sshd and rootless Podman on
# 127.0.0.1:4972) is added, its Server key authorised, validated and
# observed, including a Bakery container running on it; a replaced host key
# makes it unreachable until the host key is forgotten; Cleanup frees space
# on both without touching unlabelled images; the Local server cannot be
# deleted. Needs `task dev`.
set -euo pipefail

KEEP_FORGEJO=1 # this test does not use Forgejo
# shellcheck source=SCRIPTDIR/../lib/e2e.sh
. "$(dirname "$0")/../lib/e2e.sh"

REMOTE=(podman compose -f "$ROOT/infra/dev/compose.yml" --profile remote)
STAND_IN=bakery-dev-remote-1
SERVER_ID=""

e2e_cleanup_hook() {
	if [ -n "$SERVER_ID" ]; then bakery DELETE "/api/servers/$SERVER_ID" >/dev/null; fi
	"${REMOTE[@]}" rm -sf remote >/dev/null 2>&1
}

server() { bakery GET "/api/servers/$1" | json "d['server']$2"; }
check() { bakery GET "/api/servers/$1" | json "next(c for c in d['server']['validation']['checks'] if c['name']=='$2')$3"; }
code() { curl -s -o /dev/null -w '%{http_code}' -b "$JAR" "$@"; }
# as_podman COMMAND: runs a shell command as the stand-in's podman user.
as_podman() { podman exec "$STAND_IN" su podman -s /bin/sh -c "export XDG_RUNTIME_DIR=/run/user/1000; $1"; }
start_stand_in() {
	"${REMOTE[@]}" up -d --build --force-recreate remote >/dev/null 2>&1
	wait_for 60 "the stand-in's Podman socket" podman exec "$STAND_IN" test -S /run/user/1000/podman/podman.sock
}
authorise() { # authorise ID: adds the Server key to the stand-in
	local key
	key=$(server "$1" "['public_key']")
	podman exec "$STAND_IN" sh -c "echo '$key' >> /home/podman/.ssh/authorized_keys"
}

sign_in

say "The Local server"
LOCAL_ID=$(bakery GET /api/servers | json "next(s['id'] for s in d['servers'] if s['kind']=='local')")
[ "$(server "$LOCAL_ID" "['status']")" = reachable ] || bakery POST "/api/servers/$LOCAL_ID/validate" >/dev/null
[ "$(server "$LOCAL_ID" "['status']")" = reachable ] || fail "the local server is $(server "$LOCAL_ID" "['status']")"
[ "$(bakery GET "/api/servers/$LOCAL_ID/metrics" | json "d['server']['cpus'] > 0 and d['server']['disk_total_bytes'] > 0")" = True ] ||
	fail "local metrics are missing"
[ "$(code -X DELETE "$API/api/servers/$LOCAL_ID")" = 409 ] || fail "the local server could be deleted"

say "The Remote server stand-in"
start_stand_in
as_podman "podman pull -q docker.io/library/busybox:1.36" >/dev/null

say "Adding and validating it"
SERVER_ID=$(bakery POST /api/servers "{\"name\":\"$RUN\",\"host\":\"127.0.0.1\",\"port\":4972,\"user\":\"podman\"}" | json "d['server']['id']")
[ "$(server "$SERVER_ID" "['status']")" = unvalidated ] || fail "a new server is not unvalidated"
bakery POST "/api/servers/$SERVER_ID/validate" >/dev/null
[ "$(server "$SERVER_ID" "['status']")" = unreachable ] || fail "validated without its key authorised"
authorise "$SERVER_ID"
bakery POST "/api/servers/$SERVER_ID/validate" >/dev/null
[ "$(server "$SERVER_ID" "['status']")" = reachable ] ||
	fail "not reachable: $(server "$SERVER_ID" "['validation']['checks']")"
FINGERPRINT=$(server "$SERVER_ID" "['host_key_fingerprint']")
[ -n "$FINGERPRINT" ] || fail "no host key pinned"
[ "$(check "$SERVER_ID" podman "['detail']" | cut -d' ' -f1)" = Podman ] || fail "no Podman version"

say "Metrics, with a Bakery container on it"
as_podman "podman run -d --name bakery-e2e-sleeper --label bakery.managed=true --label bakery.test=true docker.io/library/busybox:1.36 sleep 600" >/dev/null
bakery GET "/api/servers/$SERVER_ID/metrics" >"$WORK/metrics.json"
[ "$(json "d['server']['memory_total_bytes'] > 0 and d['server']['cpus'] > 0" <"$WORK/metrics.json")" = True ] ||
	fail "remote metrics are missing: $(cat "$WORK/metrics.json")"
[ "$(json "any(c['name']=='bakery-e2e-sleeper' for c in d['containers'])" <"$WORK/metrics.json")" = True ] ||
	fail "the Bakery container is not in the remote metrics"

say "Cleanup on both, unlabelled images stay"
for id in "$LOCAL_ID" "$SERVER_ID"; do
	[ "$(bakery POST "/api/servers/$id/cleanup" | json "d['cleanup']['reclaimed_bytes'] >= 0")" = True ] || fail "cleanup of $id failed"
	[ -n "$(server "$id" "['last_cleanup']['at']")" ] || fail "cleanup of $id not recorded"
done
as_podman "podman image exists docker.io/library/busybox:1.36" || fail "cleanup removed an unlabelled image"

say "A replaced host key is refused until forgotten"
start_stand_in
authorise "$SERVER_ID"
bakery POST "/api/servers/$SERVER_ID/validate" >/dev/null
[ "$(server "$SERVER_ID" "['status']")" = unreachable ] || fail "a changed host key was accepted"
check "$SERVER_ID" ssh "['detail']" | grep -q "host key changed" || fail "no host key reason: $(check "$SERVER_ID" ssh "['detail']")"
bakery DELETE "/api/servers/$SERVER_ID/host-key" >/dev/null
bakery POST "/api/servers/$SERVER_ID/validate" >/dev/null
[ "$(server "$SERVER_ID" "['status']")" = reachable ] || fail "not reachable after forgetting the host key"
[ "$(server "$SERVER_ID" "['host_key_fingerprint']")" != "$FINGERPRINT" ] || fail "the old host key is still pinned"

say "Removing it"
bakery DELETE "/api/servers/$SERVER_ID" >/dev/null
[ "$(code "$API/api/servers/$SERVER_ID")" = 404 ] || fail "the server is still there"
SERVER_ID=""

say "PASS"
