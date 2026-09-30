#!/usr/bin/env bash
# End to end against the Forgejo stand-in: Application settings. Several
# Domains (and a change that needs no deploy), the Www redirect, a Response
# header, Basic auth, Persistent storage across a redeploy and a rollback,
# Resource limits, and the volume going with the Application. Needs
# `task dev` running (API on 127.0.0.1:4910, proxy on 4943). Starts Forgejo
# (compose profile git) and removes it and everything the test created.
set -euo pipefail

# shellcheck source=SCRIPTDIR/../lib/e2e.sh
. "$(dirname "$0")/../lib/e2e.sh"

sign_in
start_forgejo
create_repo
# httpd serves the Persistent storage at /data itself.
cat >"$REPO/Dockerfile" <<'DOCKERFILE'
FROM docker.io/library/busybox:stable
COPY index.html /index.html
EXPOSE 8080
CMD ["sh", "-c", "mkdir -p /data && cp /index.html /data/index.html && exec httpd -f -p 8080 -h /data"]
DOCKERFILE
commit() { # commit VERSION SUBJECT
	echo "version $1" >"$REPO/index.html"
	push "$2"
}
commit 1 "Version 1"

A="a-$RUN.localhost" B="b-$RUN.localhost" APEX="$RUN.test"
create_app 8080 ",\"domains\":[\"$A\",\"$B\"],\"storages\":[{\"name\":\"data\",\"mount_path\":\"/data\"}]"
# app_json DOMAINS [FIELDS]: the whole Application, as PATCH wants it.
app_json() { echo "{\"name\":\"$RUN\",\"git_url\":\"ssh://git@127.0.0.1:4952/$FORGEJO_USER/$RUN.git\",\"git_branch\":\"main\",\"port\":8080,\"domains\":$1${2:-}}"; }
deploy() { bakery POST "/api/applications/$APP_ID/deploy" | json "d['deployment']['id']"; }
status_is() { [ "$(deployment "$1" "['status']")" = "$2" ]; }
deployments() { bakery GET "/api/applications/$APP_ID/deployments" | json "len(d['deployments'])"; }
# on HOST [CURL ARGS...]: the proxy's answer for HOST on /, as "code body".
on() {
	local host=$1
	shift
	curl -sk --max-time 5 --resolve "$host:4943:127.0.0.1" -o "$WORK/body" -w '%{http_code}' "$@" "https://$host:4943/" 2>/dev/null || true
	printf ' %s' "$(cat "$WORK/body" 2>/dev/null)"
}
answers() { [ "$(on "$1")" = "200 version $2" ]; }
not_served() { [ "$(on "$1" | cut -c1-3)" != 200 ]; }
routing() { bakery PUT "/api/applications/$APP_ID/routing" "$1"; }

say "Two Domains"
FIRST=$(deploy)
wait_for 300 "the first deployment" status_is "$FIRST" finished
wait_for 30 "$A" answers "$A" 1
wait_for 30 "$B" answers "$B" 1
echo "ok: both domains serve version 1"

say "Removing a Domain needs no deploy"
bakery PATCH "/api/applications/$APP_ID" "$(app_json "[\"$A\"]")" >/dev/null
wait_for 20 "$B to stop answering" not_served "$B"
answers "$A" 1 || fail "$A stopped answering"
[ "$(deployments)" = 1 ] || fail "changing the domains started a deployment"
echo "ok: $B gone, $A still served, no new deployment"

say "Www redirect"
bakery PATCH "/api/applications/$APP_ID" "$(app_json "[\"$A\",\"$APEX\"]")" >/dev/null
routing '{"www_redirect":"to_apex"}' >/dev/null
redirect() { curl -sk --max-time 5 --resolve "www.$APEX:4943:127.0.0.1" -o /dev/null -w '%{http_code} %{redirect_url}' "https://www.$APEX:4943/some/path?q=1" || true; }
redirects() { [ "$(redirect)" = "308 https://$APEX:4943/some/path?q=1" ]; }
wait_for 20 "www.$APEX to redirect" redirects
wait_for 20 "$APEX" answers "$APEX" 1
echo "ok: www.$APEX answers 308 to $APEX"

say "Response header and Basic auth"
routing '{"www_redirect":"off","response_headers":[{"name":"X-Frame-Options","value":"DENY"}],"basic_auth":{"enabled":true,"username":"visitor","password":"let-me-in"}}' >/dev/null
locked() { [ "$(on "$A" | cut -c1-3)" = 401 ]; }
wait_for 20 "basic auth" locked
[ "$(on "$A" -u visitor:wrong | cut -c1-3)" = 401 ] || fail "a wrong password got in"
[ "$(on "$A" -u visitor:let-me-in)" = "200 version 1" ] || fail "the right password: $(on "$A" -u visitor:let-me-in)"
grep -qi '^x-frame-options: DENY' <<<"$(curl -skI -u visitor:let-me-in --resolve "$A:4943:127.0.0.1" "https://$A:4943/")" || fail "no X-Frame-Options header"
grep -q password_set <<<"$(bakery GET "/api/applications/$APP_ID/routing")" || fail "the settings do not say a password is set"
# shellcheck disable=SC2016 # a literal bcrypt prefix
if grep -qF '$2a$' <<<"$(bakery GET "/api/applications/$APP_ID/routing")"; then fail "the password hash leaves the API"; fi
routing '{"www_redirect":"off"}' >/dev/null
wait_for 20 "basic auth off" answers "$A" 1
echo "ok: 401 without, 200 with the password, header set; switched off again"

say "Persistent storage survives a redeploy and a rollback"
CONTAINER=$(latest "['container']")
podman exec "$CONTAINER" sh -c "echo 'kept $RUN' > /data/kept.txt"
commit 2 "Version 2"
SECOND=$(deploy)
wait_for 300 "the second deployment" status_is "$SECOND" finished
wait_for 30 "version 2" answers "$A" 2
kept() { [ "$(curl -sk --resolve "$A:4943:127.0.0.1" "https://$A:4943/kept.txt")" = "kept $RUN" ]; }
kept || fail "the file in /data is gone after a redeploy"
ROLLBACK=$(bakery POST "/api/deployments/$FIRST/rollback" | json "d['deployment']['id']")
wait_for 60 "the rollback" status_is "$ROLLBACK" finished
wait_for 30 "version 1 again" answers "$A" 1
kept || fail "the file in /data is gone after a rollback"
echo "ok: /data/kept.txt survived both"

say "Resource limits"
bakery PATCH "/api/applications/$APP_ID" "$(app_json "[\"$A\"]" ',"resource_limits":{"memory_mb":128,"cpus":0.5}')" >/dev/null
LIMITED=$(deploy)
wait_for 300 "the limited deployment" status_is "$LIMITED" finished
LIMITS=$(podman inspect "$(latest "['container']")" --format '{{.HostConfig.Memory}} {{.HostConfig.CpuQuota}}')
[ "$LIMITS" = "134217728 50000" ] || fail "limits: $LIMITS"
grep -q "Limits: 128 MB memory, 0.5 CPU" <<<"$(bakery GET "/api/deployments/$LIMITED/log")" || fail "the log does not show the limits"
echo "ok: running with 128 MB and half a CPU"

say "Deleting the Application removes its volume"
[ -n "$(podman volume ls -q --filter "label=bakery.application=$APP_ID")" ] || fail "no volume before the delete"
bakery DELETE "/api/applications/$APP_ID" >/dev/null
no_volume() { [ -z "$(podman volume ls -q --filter "label=bakery.application=$APP_ID")" ]; }
wait_for 30 "the volume to go" no_volume
APPS=()
echo "ok: volume removed"

say "PASS"
