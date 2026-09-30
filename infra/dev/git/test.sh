#!/usr/bin/env bash
# End to end against the Forgejo stand-in: a private repository deployed
# with the Application's Deploy key, then redeployed by a push through the
# Webhook. Needs `task dev` running (API on 127.0.0.1:4910, proxy on 4943).
# Starts Forgejo (compose profile git) and removes it and everything the
# test created when done.
set -euo pipefail

# shellcheck source=SCRIPTDIR/../lib/e2e.sh
. "$(dirname "$0")/../lib/e2e.sh"

sign_in
start_forgejo
create_repo
cat >"$REPO/Dockerfile" <<'DOCKERFILE'
FROM docker.io/library/busybox:stable
COPY index.html /www/index.html
EXPOSE 8080
CMD ["httpd", "-f", "-p", "8080", "-h", "/www"]
DOCKERFILE
commit() { # commit VERSION SUBJECT
	echo "version $1" >"$REPO/index.html"
	push "$2"
}
commit 1 "Version 1"
[ "$(curl -s -o /dev/null -w '%{http_code}' "$FORGEJO/$FORGEJO_USER/$RUN")" = 404 ] || fail "the repository is not private"

create_app 8080
serves() { [ "$(fetch /)" = "version $1" ]; }

say "Deploy (manual)"
bakery POST "/api/applications/$APP_ID/deploy" >/dev/null
wait_for 300 "the first deployment" deployment_done
[ "$(latest "['commit_message']")" = "Version 1" ] || fail "commit subject: $(latest "['commit_message']")"
wait_for 30 "version 1 on $PUBLIC_URL" serves 1
grep -qF '[127.0.0.1]:4952' <<<"$(bakery GET /api/known-hosts)" || fail "host key not remembered"
echo "ok: deployed version 1 from the private repository"

say "Webhook: push deploys by itself"
HOOK=$(bakery GET "/api/applications/$APP_ID/webhook")
SECRET=$(echo "$HOOK" | json "d['webhook']['secret']")
HOOK_PATH=$(echo "$HOOK" | json "d['webhook']['path']")
forgejo POST "/repos/$FORGEJO_USER/$RUN/hooks" "$(URL="$API$HOOK_PATH" SECRET=$SECRET python3 -c 'import json,os; print(json.dumps({"type":"forgejo","active":True,"events":["push"],"config":{"url":os.environ["URL"],"content_type":"json","secret":os.environ["SECRET"]}}))')" >/dev/null
BEFORE=$(latest "['id']")
commit 2 "Version 2 from a push"
webhook_deployment() { [ "$(latest "['id']")" != "$BEFORE" ]; }
wait_for 60 "a deployment started by the push" webhook_deployment
[ "$(latest "['trigger']")" = webhook ] || fail "trigger: $(latest "['trigger']")"
wait_for 300 "the webhook deployment" deployment_done
[ "$(latest "['commit_message']")" = "Version 2 from a push" ] || fail "commit subject: $(latest "['commit_message']")"
[ "$(latest "['commit_author']")" = "E2E Tester" ] || fail "author: $(latest "['commit_author']")"
wait_for 30 "version 2 on $PUBLIC_URL" serves 2
echo "ok: the push deployed version 2 with trigger webhook"

say "Webhook: a stale secret deploys nothing"
bakery POST "/api/applications/$APP_ID/webhook/secret" >/dev/null
BEFORE=$(latest "['id']")
commit 3 "Version 3 with a stale secret"
sleep 15
[ "$(latest "['id']")" = "$BEFORE" ] || fail "a push signed with the old secret started a deployment"
echo "ok: refused"

say "PASS"
