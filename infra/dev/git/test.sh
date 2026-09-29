#!/usr/bin/env bash
# End to end against the Forgejo stand-in: a private repository deployed
# with the Application's Deploy key, then redeployed by a push through the
# Webhook. Needs `task dev` running (API on 127.0.0.1:4910, proxy on 4943).
# Starts Forgejo (compose profile git) and removes it and everything the
# test created when done.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/../../.." && pwd)
COMPOSE=(podman compose -f "$ROOT/infra/dev/compose.yml" --profile git)
API=http://127.0.0.1:4910
FORGEJO=http://127.0.0.1:4950
STATE="$ROOT/.claude/ralph/state"
WORK=$(mktemp -d)
JAR="$WORK/cookies"
RUN="e2e-$(date +%s)-$RANDOM"
APP_ID="" PROJECT_ID="" KEEP_FORGEJO=${KEEP_FORGEJO:-}

say() { printf '\n==> %s\n' "$*"; }
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
json() { python3 -c "import json,sys; d=json.load(sys.stdin); print($1)"; }

cleanup() {
	set +e
	say "Cleaning up"
	if [ -n "$APP_ID" ]; then bakery DELETE "/api/applications/$APP_ID" >/dev/null; fi
	if [ -n "$PROJECT_ID" ]; then bakery DELETE "/api/projects/$PROJECT_ID" >/dev/null; fi
	for id in $(bakery GET /api/known-hosts 2>/dev/null | json "' '.join(str(h['id']) for h in d['known_hosts'] if h['host']=='[127.0.0.1]:4952')"); do
		bakery DELETE "/api/known-hosts/$id" >/dev/null
	done
	if [ -z "$KEEP_FORGEJO" ]; then
		"${COMPOSE[@]}" rm -sf forgejo >/dev/null 2>&1
		podman volume rm -f bakery-dev_bakery-forgejo-data bakery-dev_bakery-forgejo-config >/dev/null 2>&1
	fi
	rm -rf "$WORK"
}
trap cleanup EXIT

# bakery METHOD PATH [JSON]: the Bakery API with the Owner's Session.
bakery() {
	local args=(-sS -b "$JAR" -c "$JAR" -X "$1" "$API$2")
	if [ $# -ge 3 ]; then args+=(-H 'Content-Type: application/json' -d "$3"); fi
	curl "${args[@]}"
}
# forgejo METHOD PATH [JSON]: the Forgejo API as the test's admin.
forgejo() {
	local args=(-sS -f -X "$1" "$FORGEJO/api/v1$2" -H "Authorization: token $TOKEN")
	if [ $# -ge 3 ]; then args+=(-H 'Content-Type: application/json' -d "$3"); fi
	curl "${args[@]}"
}
wait_for() { # wait_for SECONDS DESCRIPTION COMMAND...
	local until=$((SECONDS + $1)) what=$2
	shift 2
	until "$@"; do
		[ $SECONDS -lt $until ] || fail "timed out waiting for $what"
		sleep 2
	done
}

say "Bakery API"
curl -sf "$API/api/health" >/dev/null || fail "the API is not running on $API (start task dev)"
if [ -z "${BAKERY_OWNER_EMAIL:-}" ] && [ -f "$STATE/owner.env" ]; then
	# shellcheck disable=SC1091
	. "$STATE/owner.env"
	BAKERY_OWNER_EMAIL=$OWNER_EMAIL BAKERY_OWNER_PASSWORD=$OWNER_PASSWORD
fi
[ -n "${BAKERY_OWNER_EMAIL:-}" ] || fail "set BAKERY_OWNER_EMAIL and BAKERY_OWNER_PASSWORD"
if [ "$(bakery GET /api/setup | json "d.get('needed', d.get('setup_needed', False))")" = True ]; then
	bakery POST /api/setup "{\"name\":\"Owner\",\"email\":\"$BAKERY_OWNER_EMAIL\",\"password\":\"$BAKERY_OWNER_PASSWORD\"}" >/dev/null
fi
grep -q owner <<<"$(bakery POST /api/login "{\"email\":\"$BAKERY_OWNER_EMAIL\",\"password\":\"$BAKERY_OWNER_PASSWORD\"}")" || fail "cannot sign in"

say "Forgejo on $FORGEJO"
"${COMPOSE[@]}" up -d forgejo >/dev/null 2>&1
wait_for 120 "Forgejo" curl -sf "$FORGEJO/api/healthz" -o /dev/null
USER=bakery
PASSWORD=$(head -c 24 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c 24)
"${COMPOSE[@]}" exec -T forgejo forgejo admin user create --admin --username "$USER" \
	--password "$PASSWORD" --email bakery@example.test --must-change-password=false >/dev/null 2>&1 ||
	"${COMPOSE[@]}" exec -T forgejo forgejo admin user change-password --username "$USER" \
		--password "$PASSWORD" --must-change-password=false >/dev/null
TOKEN=$("${COMPOSE[@]}" exec -T forgejo forgejo admin user generate-access-token --username "$USER" \
	--token-name "$RUN" --scopes all --raw | tr -d '\r\n')

say "Private repository $USER/$RUN"
forgejo POST /user/repos "{\"name\":\"$RUN\",\"private\":true,\"default_branch\":\"main\"}" >/dev/null
REPO="$WORK/repo"
git init -q -b main "$REPO"
cat >"$REPO/Dockerfile" <<'DOCKERFILE'
FROM docker.io/library/busybox:stable
COPY index.html /www/index.html
EXPOSE 8080
CMD ["httpd", "-f", "-p", "8080", "-h", "/www"]
DOCKERFILE
commit() { # commit VERSION SUBJECT
	echo "version $1" >"$REPO/index.html"
	git -C "$REPO" add -A
	git -C "$REPO" -c user.name="E2E Tester" -c user.email=e2e@example.test commit -qm "$2"
	git -C "$REPO" push -q "http://$USER:$PASSWORD@127.0.0.1:4950/$USER/$RUN.git" main
}
commit 1 "Version 1"
[ "$(curl -s -o /dev/null -w '%{http_code}' "$FORGEJO/$USER/$RUN")" = 404 ] || fail "the repository is not private"

say "Bakery application with the SSH URL"
PROJECT_ID=$(bakery POST /api/projects "{\"name\":\"$RUN\"}" | json "d['project']['id']")
ENV_ID=$(bakery GET "/api/projects/$PROJECT_ID" | json "d['project']['environments'][0]['id']")
APP=$(bakery POST "/api/environments/$ENV_ID/applications" \
	"{\"name\":\"$RUN\",\"git_url\":\"ssh://git@127.0.0.1:4952/$USER/$RUN.git\",\"git_branch\":\"main\",\"port\":8080}")
APP_ID=$(echo "$APP" | json "d['application']['id']")
DOMAIN=$(echo "$APP" | json "d['application']['domain']")
PUBLIC_URL=$(echo "$APP" | json "d['application']['public_url']")
KEY=$(echo "$APP" | json "d['application']['deploy_key_public']")
[[ $KEY == ssh-ed25519\ * ]] || fail "no deploy key: $APP"
forgejo POST "/repos/$USER/$RUN/keys" "$(KEY=$KEY python3 -c 'import json,os; print(json.dumps({"title":"bakery","key":os.environ["KEY"],"read_only":True}))')" >/dev/null

latest() { bakery GET "/api/applications/$APP_ID/deployments" | json "d['deployments'][0]$1"; }
deployment_done() { local s; s=$(latest "['status']"); [ "$s" = finished ] || { [ "$s" = failed ] && fail "deployment failed: $(latest "['error']")"; false; }; }
serves() { [ "$(curl -sf --resolve "$DOMAIN:4943:127.0.0.1" -k "$PUBLIC_URL")" = "version $1" ]; }

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
forgejo POST "/repos/$USER/$RUN/hooks" "$(URL="$API$HOOK_PATH" SECRET=$SECRET python3 -c 'import json,os; print(json.dumps({"type":"forgejo","active":True,"events":["push"],"config":{"url":os.environ["URL"],"content_type":"json","secret":os.environ["SECRET"]}}))')" >/dev/null
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
