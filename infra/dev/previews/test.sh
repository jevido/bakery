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
create_app 8080 ',"storages":[{"name":"data","mount_path":"/data"}]'

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
[ -n "$(podman volume ls -q --filter "label=bakery.application=$APP_ID" --filter "label=bakery.preview=$PR")" ] || fail "the preview has no volume of its own"
echo "ok: pull request #$PR serves version feature on $PREVIEW_DOMAIN, production still main"

say "The preview comment"
comments() { forgejo GET "/repos/$FORGEJO_USER/$RUN/issues/$PR/comments"; }
bakery_comments() { comments | json "len([c for c in d if c['body'].startswith('**Bakery preview**')])"; }
comment_says() { grep -qF "$1" <<<"$(comments | json "next((c['body'] for c in d if c['body'].startswith('**Bakery preview**')), '')")"; }
PREVIEW_URL=$(preview "['public_url']")
wait_for 30 "a comment with $PREVIEW_URL" comment_says "Deployed: $PREVIEW_URL"
[ "$(bakery_comments)" = 1 ] || fail "$(bakery_comments) bakery comments"
echo "ok: pull request #$PR has one comment with $PREVIEW_URL"

say "A push to the pull request redeploys the preview"
BEFORE=$(preview "['latest_deployment']['id']")
echo "version feature 2" >"$REPO/index.html"
git -C "$REPO" -c user.name="E2E Tester" -c user.email=e2e@example.test commit -qam "Version feature 2"
git -C "$REPO" push -q "http://$FORGEJO_USER:$PASSWORD@127.0.0.1:4950/$FORGEJO_USER/$RUN.git" feature
redeployed() { [ "$(preview "['latest_deployment']['id']")" != "$BEFORE" ]; }
wait_for 60 "a new preview deployment" redeployed
wait_for 300 "the preview redeployment" preview_done
wait_for 30 "version feature 2 on $PREVIEW_DOMAIN" preview_serves "version feature 2"
wait_for 30 "the comment to name the new commit" comment_says "Version feature 2"
[ "$(bakery_comments)" = 1 ] || fail "$(bakery_comments) bakery comments after the push"
serves "version main" || fail "production changed: $(fetch /)"
echo "ok: the push redeployed the preview and edited the one comment"

say "A pull request from a fork deploys nothing"
FORKER=forker
"${COMPOSE[@]}" exec -T forgejo forgejo admin user create --username "$FORKER" --password "$PASSWORD" \
	--email forker@example.test --must-change-password=false >/dev/null 2>&1 || true
FORKER_TOKEN=$("${COMPOSE[@]}" exec -T forgejo forgejo admin user generate-access-token --username "$FORKER" \
	--token-name "$RUN" --scopes all --raw 2>/dev/null | tr -d '\r\n')
forgejo PUT "/repos/$FORGEJO_USER/$RUN/collaborators/$FORKER" '{"permission":"read"}' >/dev/null
as_forker() { curl -sS -f -X "$1" "$FORGEJO/api/v1$2" -H "Authorization: token $FORKER_TOKEN" -H 'Content-Type: application/json' -d "$3"; }
as_forker POST "/repos/$FORGEJO_USER/$RUN/forks" '{}' >/dev/null
git -C "$REPO" checkout -q -b from-fork main
echo "version fork" >"$REPO/index.html"
git -C "$REPO" -c user.name="E2E Forker" -c user.email=forker@example.test commit -qam "Version fork"
wait_for 30 "the fork" git -C "$REPO" push -q "http://$FORKER:$PASSWORD@127.0.0.1:4950/$FORKER/$RUN.git" from-fork
FORK_PR=$(as_forker POST "/repos/$FORGEJO_USER/$RUN/pulls" "{\"title\":\"From a fork\",\"head\":\"$FORKER:from-fork\",\"base\":\"main\"}" | json "d['number']")
BEFORE=$(latest "['id']")
sleep 15
[ "$(previews | json "len([p for p in d['previews'] if p['number']==$FORK_PR])")" = 0 ] || fail "the fork's pull request #$FORK_PR got a preview"
[ "$(latest "['id']")" = "$BEFORE" ] || fail "the fork's pull request started a deployment"
echo "ok: pull request #$FORK_PR from $FORKER/$RUN deployed nothing"

say "Closing pull request #$PR removes the preview"
forgejo PATCH "/repos/$FORGEJO_USER/$RUN/pulls/$PR" '{"state":"closed"}' >/dev/null
preview_closed() { [ "$(preview "['state']")" = closed ]; }
wait_for 60 "preview #$PR closed" preview_closed
preview_gone() { ! fetch_preview / >/dev/null 2>&1; }
wait_for 60 "$PREVIEW_DOMAIN to stop answering" preview_gone
[ -z "$(podman ps -a -q --filter "label=bakery.application=$APP_ID" --filter "label=bakery.preview=$PR")" ] || fail "preview containers left"
[ -z "$(podman volume ls -q --filter "label=bakery.application=$APP_ID" --filter "label=bakery.preview=$PR")" ] || fail "preview volumes left"
serves "version main" || fail "production changed: $(fetch /)"
wait_for 30 "the comment to say removed" comment_says "Removed"
echo "ok: closing #$PR removed its container and route and said so; production still serves main"

say "PASS"
