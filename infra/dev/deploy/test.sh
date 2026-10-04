#!/usr/bin/env bash
# End to end against the Forgejo stand-in: Health checks, a redeploy that
# never fails a request, a broken commit that keeps the old version, Cancel,
# Rollback, a build-only variable, Shared variables, the Application status,
# Stop, Restart and Deploy without cache. Needs `task dev`
# running (API on 127.0.0.1:4910, proxy on 4943). Starts Forgejo (compose
# profile git) and removes it and everything the test created when done.
set -euo pipefail

# shellcheck source=SCRIPTDIR/../lib/e2e.sh
. "$(dirname "$0")/../lib/e2e.sh"

POLLER=""
e2e_cleanup_hook() { if [ -n "$POLLER" ]; then kill "$POLLER" 2>/dev/null; fi; }

sign_in
start_forgejo
create_repo
# /health exists unless the commit has a file named broken; a file named
# slow makes the build take two minutes.
cat >"$REPO/Dockerfile" <<'DOCKERFILE'
FROM docker.io/library/busybox:stable
ARG BUILT_WITH
COPY . /src
RUN mkdir -p /www && cp /src/index.html /www/ && echo "$BUILT_WITH" > /www/built \
	&& if [ ! -f /src/broken ]; then echo ok > /www/health; fi \
	&& if [ -f /src/slow ]; then sleep 120; fi
EXPOSE 8080
CMD ["sh", "-c", "echo \"$GREETING\" > /www/greeting; echo \"${BUILT_WITH:-}\" > /www/built-at-runtime; exec httpd -f -p 8080 -h /www"]
DOCKERFILE
commit() { # commit VERSION SUBJECT
	echo "version $1" >"$REPO/index.html"
	push "$2"
}
commit 1 "Version 1"
create_app 8080
serves() { [ "$(fetch /)" = "version $1" ]; }
deploy() { bakery POST "/api/applications/$APP_ID/deploy" | json "d['deployment']['id']"; }
status_is() { [ "$(deployment "$1" "['status']")" = "$2" ]; }

say "Health check, a build-only variable and Shared variables"
bakery PATCH "/api/applications/$APP_ID" "{\"name\":\"$RUN\",\"git_url\":\"ssh://git@127.0.0.1:4952/$FORGEJO_USER/$RUN.git\",\"git_branch\":\"main\",\"port\":8080,\"health_check\":{\"enabled\":true,\"path\":\"/health\",\"interval\":1,\"timeout\":2,\"retries\":5}}" >/dev/null
bakery PUT "/api/applications/$APP_ID/environment-variables" '{"environment_variables":[{"name":"BUILT_WITH","value":"build-arg-ok","build":true,"runtime":false}]}' >/dev/null
bakery PUT "/api/projects/$PROJECT_ID/variables" '{"environment_variables":[{"name":"GREETING","value":"hello from the project"}]}' >/dev/null
bakery PUT "/api/environments/$ENV_ID/variables" '{"environment_variables":[{"name":"GREETING","value":"hello from the environment"}]}' >/dev/null
FIRST=$(deploy)
wait_for 300 "the first deployment" deployment_done
wait_for 30 "version 1" serves 1
[ "$(fetch /built)" = build-arg-ok ] || fail "build arg: $(fetch /built)"
[ -z "$(fetch /built-at-runtime)" ] || fail "a build-only variable reached the container"
[ "$(fetch /greeting)" = "hello from the environment" ] || fail "shared variable: $(fetch /greeting)"
LOG=$(curl -sS -b "$JAR" --max-time 5 "$API/api/deployments/$FIRST/log" || true)
grep -q "Healthy after" <<<"$LOG" || fail "no health check in the log"
echo "ok: healthy, build arg in the image only, the environment's variable wins"

say "Redeploy without a failed request"
commit 2 "Version 2"
: >"$WORK/codes"
(while :; do curl -s -o /dev/null -w '%{http_code}\n' --max-time 5 --resolve "$DOMAIN:4943:127.0.0.1" -k "$PUBLIC_URL/" >>"$WORK/codes"; sleep 0.1; done) &
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
echo "ok: $TOTAL requests during the redeploy, none failed"

say "A failing health check keeps the old version"
touch "$REPO/broken"
commit 3 "Version 3 without /health"
BROKEN=$(deploy)
wait_for 300 "the broken deployment to fail" status_is "$BROKEN" failed
ERROR=$(deployment "$BROKEN" "['error']")
[[ $ERROR == "health check failed after 5 attempts"* ]] || fail "error: $ERROR"
serves 2 || fail "version 2 is no longer served"
echo "ok: $ERROR; version 2 still served"

say "Cancel during the build"
rm "$REPO/broken"
touch "$REPO/slow"
commit 4 "Version 4, slow to build"
SLOW=$(deploy)
wait_for 120 "the slow build" status_is "$SLOW" building
sleep 3
bakery POST "/api/deployments/$SLOW/cancel" >/dev/null
wait_for 30 "the cancel" status_is "$SLOW" cancelled
serves 2 || fail "version 2 is no longer served"
[ "$(podman ps -a --filter "label=bakery.deployment=$SLOW" --format '{{.Names}}')" = "" ] || fail "the cancelled deployment left a container"
echo "ok: cancelled; version 2 still served"

say "Roll back to the first deployment"
ROLLBACK=$(bakery POST "/api/deployments/$FIRST/rollback" | json "d['deployment']['id']")
wait_for 60 "the rollback" status_is "$ROLLBACK" finished
[ "$(deployment "$ROLLBACK" "['trigger']")" = rollback ] || fail "trigger: $(deployment "$ROLLBACK" "['trigger']")"
[ "$(deployment "$ROLLBACK" "['commit_message']")" = "Version 1" ] || fail "commit: $(deployment "$ROLLBACK" "['commit_message']")"
wait_for 30 "version 1 again" serves 1
[ "$(fetch /greeting)" = "hello from the environment" ] || fail "rollback lost the runtime variables"
echo "ok: version 1 is back without a build (deployment $ROLLBACK, second was $SECOND)"
# The Rollback page: the rollback's Image is the current one, and the second
# Deployment's is offered; the first shares the rollback's Image.
IMAGES=$(bakery GET "/api/applications/$APP_ID/images" | json "' '.join(f\"{i['deployment']['id']}:{i['current']}\" for i in d['images'])")
case " $IMAGES " in *" $ROLLBACK:True "*) ;; *) fail "images: $IMAGES" ;; esac
case " $IMAGES " in *" $SECOND:False "*) ;; *) fail "images: $IMAGES" ;; esac
case " $IMAGES " in *" $FIRST:"*) fail "images list the first deployment: $IMAGES" ;; esac
# The Deployment history: a page, its total and the filters.
HISTORY=$(bakery GET "/api/applications/$APP_ID/deployments?take=1&filters=1")
[ "$(json "len(d['deployments']), d['count'] >= 3, 'rollback' in d['filters']['sources']" <<<"$HISTORY")" = "1 True True" ] || fail "history: $HISTORY"
[ "$(bakery GET "/api/applications/$APP_ID/deployments?source=rollback&status=finished" | json "[x['id'] for x in d['deployments']], d['count']")" = "[$ROLLBACK] 1" ] ||
  fail "history filtered by rollback"
echo "ok: the Rollback page offers the second deployment's image, the history pages and filters"

say "Status, Stop, Deploy, Restart"
rm "$REPO/slow"
commit 5 "Version 5"
app_status() { bakery GET "/api/applications/$APP_ID/status" | json "d['status']"; }
status_now() { [ "$(app_status)" = "$1" ]; }
wait_for 30 "running:healthy" status_now running:healthy
STOPPED=$(bakery POST "/api/applications/$APP_ID/stop")
[ "$(json "d['status'], d['container_present']" <<<"$STOPPED")" = "exited False" ] || fail "stop answered $STOPPED"
[ "$(podman ps -a --filter "label=bakery.application=$APP_ID" --format '{{.Names}}')" = "" ] || fail "stop left a container"
! fetch / >/dev/null || fail "the domain still answers after stop"
echo "ok: stopped, status exited, nothing left, the domain no longer answers"
FIFTH=$(deploy)
wait_for 300 "the deployment after stop" status_is "$FIFTH" finished
wait_for 30 "version 5" serves 5
status_now running:healthy || fail "status after deploy: $(app_status)"
RESTART=$(bakery POST "/api/applications/$APP_ID/restart" | json "d['deployment']['id']")
wait_for 60 "the restart" status_is "$RESTART" finished
[ "$(deployment "$RESTART" "['trigger']")" = restart ] || fail "trigger: $(deployment "$RESTART" "['trigger']")"
[ "$(deployment "$RESTART" "['rollback_of']")" = "$FIFTH" ] || fail "restart of: $(deployment "$RESTART" "['rollback_of']")"
serves 5 || fail "version 5 is not served after the restart"
status_now running:healthy || fail "status after restart: $(app_status)"
[ "$(podman ps --filter "label=bakery.application=$APP_ID" --format '{{.Names}}')" = "bakery-app-$APP_ID-$RESTART" ] || fail "restart did not replace the container"
echo "ok: deployed again after stop, restarted without a build (deployment $RESTART)"

say "Deploy with and without the build cache"
log_of() { curl -sS -b "$JAR" --max-time 5 "$API/api/deployments/$1/log" || true; }
CACHED=$(deploy)
wait_for 300 "the cached redeploy" status_is "$CACHED" finished
grep -q "Using cache" <<<"$(log_of "$CACHED")" || fail "a plain redeploy of the same commit used no cached layer"
FORCED=$(bakery POST "/api/applications/$APP_ID/deploy" '{"force_rebuild":true}' | json "d['deployment']['id']")
[ "$(deployment "$FORCED" "['force_rebuild']")" = True ] || fail "force_rebuild not recorded"
wait_for 300 "the redeploy without cache" status_is "$FORCED" finished
FORCED_LOG=$(log_of "$FORCED")
grep -q "Building without the build cache" <<<"$FORCED_LOG" || fail "no-cache build not logged"
! grep -q "Using cache" <<<"$FORCED_LOG" || fail "the redeploy without cache used cached layers"
serves 5 || fail "version 5 is not served after the redeploy without cache"
echo "ok: a plain redeploy reuses layers, force_rebuild builds them all"

say "PASS"
