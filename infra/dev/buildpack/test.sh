#!/usr/bin/env bash
# End to end against the Forgejo stand-in: every Build pack. A static site
# from a repository. Needs `task dev` running (API on 127.0.0.1:4910, proxy
# on 4943). Starts Forgejo (compose profile git) and removes it and
# everything the test created when done.
set -euo pipefail

# shellcheck source=SCRIPTDIR/../lib/e2e.sh
. "$(dirname "$0")/../lib/e2e.sh"

sign_in
start_forgejo
create_repo

say "Static site"
mkdir -p "$REPO/public"
echo "static marker $RUN" >"$REPO/public/index.html"
echo "not published" >"$REPO/secret.txt"
push "A static site"
create_app 80 ',"build_pack":"static","publish_directory":"public","health_check":{"enabled":true,"path":"/","interval":1,"timeout":2,"retries":5}'
bakery POST "/api/applications/$APP_ID/deploy" >/dev/null
wait_for 300 "the static deployment" deployment_done
grep -q "Healthy after" <<<"$(bakery GET "/api/deployments/$(latest "['id']")/log")" || fail "the static site was not health checked"
[ "$(fetch /)" = "static marker $RUN" ] || fail "static site answers: $(fetch / || true)"
if fetch /secret.txt >/dev/null; then fail "a file outside the publish directory is served"; fi

say "PASS"
