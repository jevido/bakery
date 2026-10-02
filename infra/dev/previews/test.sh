#!/usr/bin/env bash
# End to end against the Forgejo stand-in: Previews. A pull request from a
# branch of the Application's own repository gets its own running copy on
# pr-<n>.<domain>, redeployed on every push to it and announced with one
# Preview comment; a fork's pull request deploys nothing; closing the pull
# request removes the Preview. Needs `task dev` running (API on
# 127.0.0.1:4910, proxy on 4943). Starts Forgejo (compose profile git) and
# removes it and everything the test created when done.
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
echo "version main" >"$REPO/index.html"
push "Version main"
create_app 8080

say "Deploy the application itself"
bakery POST "/api/applications/$APP_ID/deploy" >/dev/null
wait_for 300 "the first deployment" deployment_done
serves() { [ "$(fetch /)" = "$1" ]; }
wait_for 30 "version main on $PUBLIC_URL" serves "version main"
echo "ok: deployed main"

say "Previews on, the webhook sends pull request events"
HOOK=$(bakery PATCH "/api/applications/$APP_ID/webhook" "{\"previews\":true,\"git_host_token\":\"$TOKEN\"}")
[ "$(echo "$HOOK" | json "d['webhook']['previews']")" = True ] || fail "previews not on: $HOOK"
[ "$(echo "$HOOK" | json "d['webhook']['has_git_host_token']")" = True ] || fail "token not saved: $HOOK"
grep -qF "$TOKEN" <<<"$HOOK" && fail "the git host token was returned"
SECRET=$(echo "$HOOK" | json "d['webhook']['secret']")
HOOK_PATH=$(echo "$HOOK" | json "d['webhook']['path']")
forgejo POST "/repos/$FORGEJO_USER/$RUN/hooks" "$(URL="$API$HOOK_PATH" SECRET=$SECRET python3 -c 'import json,os; print(json.dumps({"type":"forgejo","active":True,"events":["push","pull_request","pull_request_sync"],"config":{"url":os.environ["URL"],"content_type":"json","secret":os.environ["SECRET"]}}))')" >/dev/null

say "A pull request from branch feature"
git -C "$REPO" checkout -q -b feature
echo "version feature" >"$REPO/index.html"
git -C "$REPO" -c user.name="E2E Tester" -c user.email=e2e@example.test commit -qam "Version feature"
git -C "$REPO" push -q "http://$FORGEJO_USER:$PASSWORD@127.0.0.1:4950/$FORGEJO_USER/$RUN.git" feature
PR=$(forgejo POST "/repos/$FORGEJO_USER/$RUN/pulls" '{"title":"Feature","head":"feature","base":"main"}' | json "d['number']")
PREVIEW_DOMAIN="pr-$PR.$DOMAIN"
previews() { bakery GET "/api/applications/$APP_ID/previews"; }
preview() { previews | json "next((p for p in d['previews'] if p['number']==$PR), {})$1"; }
preview_done() {
	local s
	s=$(preview "['latest_deployment']['status']" 2>/dev/null) || return 1
	[ "$s" = finished ] || { [ "$s" = failed ] && fail "preview deployment failed: $(preview "['latest_deployment']['error']")"; false; }
}
wait_for 300 "the preview deployment of #$PR" preview_done
[ "$(preview "['domain']")" = "$PREVIEW_DOMAIN" ] || fail "preview domain: $(preview "['domain']")"
[ "$(preview "['latest_deployment']['branch']")" = feature ] || fail "preview branch: $(preview "['latest_deployment']['branch']")"
fetch_preview() { curl -sf --resolve "$PREVIEW_DOMAIN:4943:127.0.0.1" -k "https://$PREVIEW_DOMAIN:4943$1"; }
preview_serves() { [ "$(fetch_preview /)" = "$1" ]; }
wait_for 30 "version feature on $PREVIEW_DOMAIN" preview_serves "version feature"
serves "version main" || fail "production changed: $(fetch /)"
echo "ok: pull request #$PR serves version feature on $PREVIEW_DOMAIN, production still main"
