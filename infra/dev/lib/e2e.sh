# shellcheck shell=bash
# Shared by the end-to-end scripts under infra/dev: the Bakery API with the
# Owner's Session, the Forgejo stand-in, and a private repository deployed
# through a Deploy key. Source it after `set -euo pipefail`; it sets a trap
# that removes everything the test created.

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
COMPOSE=(podman compose -f "$ROOT/infra/dev/compose.yml" --profile git)
API=http://127.0.0.1:4910
FORGEJO=http://127.0.0.1:4950
STATE="$ROOT/.claude/ralph/state"
WORK=$(mktemp -d)
JAR="$WORK/cookies"
RUN="e2e-$(date +%s)-$RANDOM"
APP_ID="" APP_SLUG="" PROJECT_ID="" KEEP_FORGEJO=${KEEP_FORGEJO:-}
# Every "id slug" of an Application the test made, for cleanup.
APPS=()
REPO="$WORK/repo"
FORGEJO_USER=bakery

say() { printf '\n==> %s\n' "$*"; }
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
json() { python3 -c "import json,sys; d=json.load(sys.stdin); print($1)"; }

# Extra cleanup a script wants run first.
e2e_cleanup_hook() { :; }

cleanup() {
	set +e
	say "Cleaning up"
	e2e_cleanup_hook
	local entry
	for entry in "${APPS[@]}"; do
		bakery DELETE "/api/applications/${entry% *}" >/dev/null
		podman images --format '{{.Repository}}:{{.Tag}}' | grep "^localhost/bakery/${entry#* }:" | xargs -r podman rmi -f >/dev/null 2>&1
	done
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

# sign_in signs the Owner in (setting Bakery up first on a fresh database).
sign_in() {
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
	grep -q "\"member\"" <<<"$(bakery POST /api/login "{\"email\":\"$BAKERY_OWNER_EMAIL\",\"password\":\"$BAKERY_OWNER_PASSWORD\"}")" || fail "cannot sign in"
}

# start_forgejo starts Forgejo and sets PASSWORD and TOKEN for its admin.
start_forgejo() {
	say "Forgejo on $FORGEJO"
	"${COMPOSE[@]}" up -d forgejo >/dev/null 2>&1
	wait_for 120 "Forgejo" curl -sf "$FORGEJO/api/healthz" -o /dev/null
	PASSWORD=$(head -c 24 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c 24)
	"${COMPOSE[@]}" exec -T forgejo forgejo admin user create --admin --username "$FORGEJO_USER" \
		--password "$PASSWORD" --email bakery@example.test --must-change-password=false >/dev/null 2>&1 ||
		"${COMPOSE[@]}" exec -T forgejo forgejo admin user change-password --username "$FORGEJO_USER" \
			--password "$PASSWORD" --must-change-password=false >/dev/null
	TOKEN=$("${COMPOSE[@]}" exec -T forgejo forgejo admin user generate-access-token --username "$FORGEJO_USER" \
		--token-name "$RUN" --scopes all --raw | tr -d '\r\n')
}

# create_repo creates the private repository $RUN and an empty clone of it
# in $REPO.
create_repo() {
	say "Private repository $FORGEJO_USER/$RUN"
	forgejo POST /user/repos "{\"name\":\"$RUN\",\"private\":true,\"default_branch\":\"main\"}" >/dev/null
	git init -q -b main "$REPO"
}

# push SUBJECT commits everything in $REPO and pushes it.
push() {
	git -C "$REPO" add -A
	git -C "$REPO" -c user.name="E2E Tester" -c user.email=e2e@example.test commit -qm "$1"
	git -C "$REPO" push -q "http://$FORGEJO_USER:$PASSWORD@127.0.0.1:4950/$FORGEJO_USER/$RUN.git" main
}

# create_app PORT [FIELDS] creates a Project and an Application with the
# repository's SSH URL, and adds its Deploy key to the repository. FIELDS
# are more JSON members for the Application, e.g. ,"build_pack":"static".
# Sets PROJECT_ID, ENV_ID, APP_ID, APP_SLUG, DOMAIN and PUBLIC_URL.
create_app() {
	say "Bakery application with the SSH URL"
	new_app "{\"name\":\"$RUN\",\"git_url\":\"ssh://git@127.0.0.1:4952/$FORGEJO_USER/$RUN.git\",\"git_branch\":\"main\",\"port\":$1${2:-}}"
}

# new_app JSON creates an Application (and the Project, the first time).
# An Application with a Deploy key gets it added to the repository. Sets
# the same variables as create_app.
new_app() {
	if [ -z "$PROJECT_ID" ]; then
		PROJECT_ID=$(bakery POST /api/projects "{\"name\":\"$RUN\"}" | json "d['project']['id']")
		ENV_ID=$(bakery GET "/api/projects/$PROJECT_ID" | json "d['project']['environments'][0]['id']")
	fi
	local app key
	app=$(bakery POST "/api/environments/$ENV_ID/applications" "$1")
	APP_ID=$(echo "$app" | json "d['application']['id']") || fail "creating the application: $app"
	APP_SLUG=$(echo "$app" | json "d['application']['slug']")
	APPS+=("$APP_ID $APP_SLUG")
	DOMAIN=$(echo "$app" | json "d['application']['domains'][0]")
	PUBLIC_URL=$(echo "$app" | json "d['application']['public_url']")
	key=$(echo "$app" | json "d['application']['deploy_key_public']")
	if [[ $1 == *'"ssh://'* ]]; then
		[[ $key == ssh-ed25519\ * ]] || fail "no deploy key: $app"
		forgejo POST "/repos/$FORGEJO_USER/$RUN/keys" "$(KEY=$key TITLE="bakery-$APP_SLUG" python3 -c 'import json,os; print(json.dumps({"title":os.environ["TITLE"],"key":os.environ["KEY"],"read_only":True}))')" >/dev/null
	fi
}

latest() { bakery GET "/api/applications/$APP_ID/deployments" | json "d['deployments'][0]$1"; }
deployment() { bakery GET "/api/deployments/$1" | json "d['deployment']$2"; }
deployment_done() { local s; s=$(latest "['status']"); [ "$s" = finished ] || { [ "$s" = failed ] && fail "deployment failed: $(latest "['error']")"; false; }; }
# fetch PATH prints what the Application answers on PATH through the proxy.
fetch() { curl -sf --resolve "$DOMAIN:4943:127.0.0.1" -k "$PUBLIC_URL$1"; }
