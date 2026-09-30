#!/usr/bin/env bash
# End to end: an Application deployed to a Remote server. The stand-in from
# `task remote:up` (sshd and rootless Podman on 127.0.0.1:4972, its Proxy's
# 80/443 on 4974/4975) is added and validated; an Application targeting it
# is cloned here from the Forgejo stand-in, built and run there with a
# Health check, and served by the stand-in's own Proxy, not the local one.
# A redeploy never fails a request, Rollback skips the build, container
# logs follow the remote Container, a Server with Applications cannot be
# removed, Cleanup applies there, a Server that cannot be reached fails a
# Deployment with the reason, and deleting the Application removes its
# Containers and volumes from the stand-in. Needs `task dev`.
set -euo pipefail

# shellcheck source=SCRIPTDIR/../lib/e2e.sh
. "$(dirname "$0")/../lib/e2e.sh"

REMOTE=(podman compose -f "$ROOT/infra/dev/compose.yml" --profile remote)
STAND_IN=bakery-dev-remote-1
# A TCP forwarder to the stand-in's SSH on this port stands in for a Server
# that goes away after it was validated.
RELAY_PORT=4979
SERVER_ID="" RELAY_ID="" POLLER="" RELAY=""

e2e_cleanup_hook() {
	if [ -n "$POLLER" ]; then kill "$POLLER" 2>/dev/null; fi
	if [ -n "$RELAY" ]; then kill "$RELAY" 2>/dev/null; fi
	# Applications go before their Servers can.
	local entry
	for entry in "${APPS[@]}"; do bakery DELETE "/api/applications/${entry% *}" >/dev/null; done
	APPS=()
	if [ -n "$RELAY_ID" ]; then bakery DELETE "/api/servers/$RELAY_ID" >/dev/null; fi
	if [ -n "$SERVER_ID" ]; then bakery DELETE "/api/servers/$SERVER_ID" >/dev/null; fi
	"${REMOTE[@]}" rm -sf remote >/dev/null 2>&1
}

server() { bakery GET "/api/servers/$1" | json "d['server']$2"; }
code() { curl -s -o /dev/null -w '%{http_code}' -b "$JAR" "$@"; }
# as_podman COMMAND: runs a shell command as the stand-in's podman user.
as_podman() { podman exec "$STAND_IN" su podman -s /bin/sh -c "export XDG_RUNTIME_DIR=/run/user/1000; $1"; }
authorise() { # authorise ID: adds the Server key to the stand-in
	local key
	key=$(server "$1" "['public_key']")
	podman exec "$STAND_IN" sh -c "echo '$key' >> /home/podman/.ssh/authorized_keys"
}
add_server() { # add_server NAME PORT: adds, authorises and validates; prints the id
	local id
	id=$(bakery POST /api/servers "{\"name\":\"$1\",\"host\":\"127.0.0.1\",\"port\":$2,\"user\":\"podman\"}" | json "d['server']['id']")
	authorise "$id"
	bakery POST "/api/servers/$id/validate" >/dev/null
	[ "$(server "$id" "['status']")" = reachable ] || fail "$1 is not reachable: $(server "$id" "['validation']['checks']")"
	echo "$id"
}
# remote_fetch PATH: what the Application answers through the stand-in's
# Proxy (plain HTTP next to HTTPS, as with Internal TLS in development).
remote_fetch() { curl -sf --max-time 5 --resolve "$DOMAIN:4974:127.0.0.1" "http://$DOMAIN:4974$1"; }
serves() { [ "$(remote_fetch /)" = "version $1" ]; }
deploy() { bakery POST "/api/applications/$APP_ID/deploy" | json "d['deployment']['id']"; }
status_is() { [ "$(deployment "$1" "['status']")" = "$2" ]; }

sign_in

say "The Remote server stand-in"
"${REMOTE[@]}" up -d --build --force-recreate remote >/dev/null 2>&1
wait_for 60 "the stand-in's Podman socket" podman exec "$STAND_IN" test -S /run/user/1000/podman/podman.sock
SERVER_ID=$(add_server "$RUN" 4972)
echo "ok: server $SERVER_ID reachable"

start_forgejo
create_repo
cat >"$REPO/Dockerfile" <<'DOCKERFILE'
FROM docker.io/library/busybox:stable
COPY index.html /www/index.html
RUN echo ok > /www/health
EXPOSE 8080
CMD ["sh", "-c", "echo serving; exec httpd -f -vv -p 8080 -h /www"]
DOCKERFILE
commit() { # commit VERSION SUBJECT
	echo "version $1" >"$REPO/index.html"
	push "$2"
}
commit 1 "Version 1"
create_app 8080 ",\"server_id\":$SERVER_ID,\"storages\":[{\"name\":\"data\",\"mount_path\":\"/data\"}],\"health_check\":{\"enabled\":true,\"path\":\"/health\",\"interval\":1,\"timeout\":2,\"retries\":10}"
[ "$(bakery GET "/api/applications/$APP_ID" | json "d['application']['server_id']")" = "$SERVER_ID" ] || fail "the application does not target the server"

say "Deploying to it"
FIRST=$(deploy)
wait_for 300 "the first deployment" deployment_done
[ "$(deployment "$FIRST" "['server_id']")" = "$SERVER_ID" ] || fail "the deployment ran on server $(deployment "$FIRST" "['server_id']")"
LOG=$(curl -sS -b "$JAR" --max-time 5 "$API/api/deployments/$FIRST/log" || true)
grep -q "Deploying on server $RUN" <<<"$LOG" || fail "the log does not name the server"
grep -q "Healthy after" <<<"$LOG" || fail "no health check in the log"
wait_for 60 "version 1 through the stand-in's proxy" serves 1
as_podman "podman container exists bakery-app-$APP_ID-$FIRST" || fail "the container is not on the stand-in"
! podman container exists "bakery-app-$APP_ID-$FIRST" || fail "the container runs locally"
as_podman "podman container exists bakery-proxy" || fail "no proxy on the stand-in"
[ "$(curl -sk --max-time 5 --resolve "$DOMAIN:4943:127.0.0.1" "https://$DOMAIN:4943/" || true)" != "version 1" ] ||
	fail "the local proxy serves the remote application"
echo "ok: built and running on the stand-in, served by its proxy only"

say "Redeploy without a failed request"
commit 2 "Version 2"
: >"$WORK/codes"
(while :; do curl -s -o /dev/null -w '%{http_code}\n' --max-time 5 --resolve "$DOMAIN:4974:127.0.0.1" "http://$DOMAIN:4974/" >>"$WORK/codes"; sleep 0.1; done) &
POLLER=$!
SECOND=$(deploy)
wait_for 300 "the second deployment" deployment_done
wait_for 30 "version 2" serves 2
sleep 2
kill "$POLLER"
POLLER=""
TOTAL=$(wc -l <"$WORK/codes")
FAILED=$(grep -vc '^200$' "$WORK/codes" || true)
[ "$TOTAL" -gt 20 ] || fail "only $TOTAL requests made"
[ "$FAILED" -eq 0 ] || fail "$FAILED of $TOTAL requests failed during the redeploy: $(sort "$WORK/codes" | uniq -c | tr '\n' ' ')"
! as_podman "podman container exists bakery-app-$APP_ID-$FIRST" || fail "the first container is still there"
echo "ok: $TOTAL requests during the redeploy, none failed"

say "Roll back to the first deployment"
ROLLBACK=$(bakery POST "/api/deployments/$FIRST/rollback" | json "d['deployment']['id']")
wait_for 60 "the rollback" status_is "$ROLLBACK" finished
LOG=$(curl -sS -b "$JAR" --max-time 5 "$API/api/deployments/$ROLLBACK/log" || true)
! grep -q "Building image" <<<"$LOG" || fail "the rollback built"
wait_for 30 "version 1 again" serves 1
echo "ok: version 1 is back without a build (deployment $ROLLBACK, second was $SECOND)"

say "Container logs of the remote container"
remote_fetch / >/dev/null
LOGS=$(curl -sS -b "$JAR" --max-time 4 "$API/api/applications/$APP_ID/logs" 2>/dev/null || true)
grep -q "serving" <<<"$LOGS" || fail "no container logs: $LOGS"
echo "ok: the logs follow the container on the stand-in"

say "A server with applications cannot be removed; Cleanup there"
[ "$(code -X DELETE "$API/api/servers/$SERVER_ID")" = 409 ] || fail "the server could be removed"
[ "$(bakery POST "/api/servers/$SERVER_ID/cleanup" | json "d['cleanup']['reclaimed_bytes'] >= 0")" = True ] || fail "cleanup failed"
as_podman "podman image exists localhost/bakery/$APP_SLUG:$FIRST" || fail "cleanup removed an image a rollback needs"
echo "ok: refused with applications; cleanup kept the rollback images"

say "A server that cannot be reached fails a deployment"
python3 - "$RELAY_PORT" <<'PY' &
import socket, sys, threading
def pipe(a, b):
    try:
        while (data := a.recv(65536)):
            b.sendall(data)
    except OSError:
        pass
    finally:
        a.close(); b.close()
server = socket.create_server(("127.0.0.1", int(sys.argv[1])))
while True:
    client, _ = server.accept()
    upstream = socket.create_connection(("127.0.0.1", 4972))
    threading.Thread(target=pipe, args=(client, upstream), daemon=True).start()
    threading.Thread(target=pipe, args=(upstream, client), daemon=True).start()
PY
RELAY=$!
wait_for 10 "the relay" bash -c "exec 3<>/dev/tcp/127.0.0.1/$RELAY_PORT" 2>/dev/null
RELAY_ID=$(add_server "$RUN-relay" "$RELAY_PORT")
kill "$RELAY"
RELAY=""
FIRST_APP="$APP_ID $APP_SLUG $DOMAIN"
new_app "{\"name\":\"$RUN-gone\",\"build_pack\":\"image\",\"image_reference\":\"docker.io/traefik/whoami:v1.10\",\"port\":80,\"server_id\":$RELAY_ID}"
GONE=$(deploy)
wait_for 60 "the deployment to fail" status_is "$GONE" failed
ERROR=$(deployment "$GONE" "['error']")
[[ $ERROR == *"is not reachable"* ]] || fail "error: $ERROR"
echo "ok: $ERROR"
bakery DELETE "/api/applications/$APP_ID" >/dev/null
unset 'APPS[-1]'
bakery DELETE "/api/servers/$RELAY_ID" >/dev/null
RELAY_ID=""
read -r APP_ID APP_SLUG DOMAIN <<<"$FIRST_APP"

say "Deleting the application removes it from the stand-in"
bakery DELETE "/api/applications/$APP_ID" >/dev/null
APPS=()
wait_for 30 "the containers to go" bash -c "! podman exec $STAND_IN su podman -s /bin/sh -c 'XDG_RUNTIME_DIR=/run/user/1000 podman ps -a --format {{.Names}}' | grep -q bakery-app-$APP_ID-"
[ -z "$(as_podman "podman volume ls -q --filter label=bakery.application=$APP_ID")" ] || fail "volumes left on the stand-in"
wait_for 30 "the route to go" bash -c "! curl -sf --max-time 5 --resolve $DOMAIN:4974:127.0.0.1 http://$DOMAIN:4974/ | grep -q version"
bakery DELETE "/api/servers/$SERVER_ID" >/dev/null
[ "$(code "$API/api/servers/$SERVER_ID")" = 404 ] || fail "the server is still there"
SERVER_ID=""
echo "ok: nothing of the application is left on the stand-in"

say "PASS"
