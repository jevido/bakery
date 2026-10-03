#!/usr/bin/env bash
# End to end: Environments are added, renamed and deleted. The Owner adds
# `staging` to a Project, a second `Staging` is refused, renaming and
# describing it works, an Application or a Database in it keeps it from
# being deleted (409) until they are gone, and a viewer is refused every
# write. Needs `task dev` running.
set -euo pipefail

KEEP_FORGEJO=1 # this test does not use Forgejo
# shellcheck source=SCRIPTDIR/../lib/e2e.sh
. "$(dirname "$0")/../lib/e2e.sh"

DB_ID="" VIEWER_ID=""
e2e_cleanup_hook() {
	[ -z "$DB_ID" ] || bakery DELETE "/api/databases/$DB_ID" >/dev/null
	[ -z "$VIEWER_ID" ] || bakery DELETE "/api/members/$VIEWER_ID" >/dev/null
}

# as JAR METHOD PATH [JSON]: the status code of a request with JAR's
# Session; the body is in $WORK/body.
as() {
	local args=(-s -o "$WORK/body" -w '%{http_code}' -b "$1" -c "$1" -X "$2" "$API$3")
	if [ $# -ge 4 ]; then args+=(-H 'Content-Type: application/json' -d "$4"); fi
	curl "${args[@]}"
}
body() { json "$1" <"$WORK/body"; }
expect() { # expect DESCRIPTION WANT GOT
	[ "$2" = "$3" ] || fail "$1: got $3, want $2 ($(head -c 300 "$WORK/body" 2>/dev/null))"
	echo "ok: $1"
}

sign_in
PROJECT_ID=$(bakery POST /api/projects "{\"name\":\"$RUN\"}" | json "d['project']['id']")
PRODUCTION=$(bakery GET "/api/projects/$PROJECT_ID" | json "d['project']['environments'][0]['id']")

say "Adding, renaming and describing"
expect "staging is added" 201 "$(as "$JAR" POST "/api/projects/$PROJECT_ID/environments" '{"name":"staging","description":"pre-production"}')"
STAGING=$(body "d['environment']['id']")
expect "it belongs to the Project" "$PROJECT_ID" "$(body "d['environment']['project_id']")"
expect "a second Staging is refused" 422 "$(as "$JAR" POST "/api/projects/$PROJECT_ID/environments" '{"name":"Staging"}')"
expect "on its name" True "$(body "'name' in str(d)")"
expect "a two-letter name is refused" 422 "$(as "$JAR" POST "/api/projects/$PROJECT_ID/environments" '{"name":"qa"}')"
expect "an unknown Project is 404" 404 "$(as "$JAR" POST "/api/projects/999999999/environments" '{"name":"staging"}')"
expect "renaming onto production is refused" 422 "$(as "$JAR" PATCH "/api/environments/$STAGING" '{"name":"Production"}')"
expect "staging is renamed" 200 "$(as "$JAR" PATCH "/api/environments/$STAGING" '{"name":"Staging","description":"the last stop"}')"
expect "with its new name" Staging "$(body "d['environment']['name']")"
expect "the Environment reads" 200 "$(as "$JAR" GET "/api/environments/$STAGING")"
expect "with its description" "the last stop" "$(body "d['environment']['description']")"
expect "with its Project's name" "$RUN" "$(body "d['environment']['project_name']")"
expect "with its Applications" "[]" "$(body "d['environment']['applications']")"
expect "the Project lists both, with descriptions" "production:,Staging:the last stop" \
	"$(bakery GET "/api/projects/$PROJECT_ID" | json "','.join(e['name']+':'+e['description'] for e in d['project']['environments'])")"
expect "an unknown Environment is 404" 404 "$(as "$JAR" GET "/api/environments/999999999")"

say "A viewer changes nothing"
VIEWER=$WORK/viewer
V_TOKEN=$(bakery POST /api/invitations "{\"email\":\"viewer-$RUN@example.com\",\"role\":\"viewer\"}" | json "d['path'].rsplit('/', 1)[1]")
expect "viewer accepts" 201 "$(as "$VIEWER" POST "/api/invitations/by-token/$V_TOKEN/accept" '{"name":"Viewer","password":"correct horse battery"}')"
VIEWER_ID=$(body "d['member']['id']")
expect "viewer reads the Environment" 200 "$(as "$VIEWER" GET "/api/environments/$STAGING")"
expect "viewer cannot add one" 403 "$(as "$VIEWER" POST "/api/projects/$PROJECT_ID/environments" '{"name":"viewers"}')"
expect "viewer cannot rename one" 403 "$(as "$VIEWER" PATCH "/api/environments/$STAGING" '{"name":"viewers"}')"
expect "viewer cannot delete one" 403 "$(as "$VIEWER" DELETE "/api/environments/$STAGING")"

say "Deleting"
APP_ID=$(bakery POST "/api/environments/$STAGING/applications" \
	"{\"name\":\"$RUN\",\"build_pack\":\"dockerimage\",\"docker_image\":\"ghcr.io/traefik/whoami:v1.10\",\"port\":80}" | json "d['application']['id']")
APPS+=("$APP_ID $(bakery GET "/api/applications/$APP_ID" | json "d['application']['slug']")")
expect "an Environment with an Application stays" 409 "$(as "$JAR" DELETE "/api/environments/$STAGING")"
expect "the Environment lists it" "$APP_ID" "$(bakery GET "/api/environments/$STAGING" | json "d['environment']['applications'][0]['id']")"
expect "the Application goes" 204 "$(as "$JAR" DELETE "/api/applications/$APP_ID")"
APPS=()
DB_ID=$(bakery POST "/api/environments/$STAGING/databases" "{\"name\":\"$RUN\",\"type\":\"redis\"}" | json "d['database']['id']")
expect "an Environment with a Database stays" 409 "$(as "$JAR" DELETE "/api/environments/$STAGING")"
expect "and says why" "Environment Staging has resources, delete them first." "$(body "d.get('error') or d.get('message')")"
expect "the Database goes" 204 "$(as "$JAR" DELETE "/api/databases/$DB_ID")"
DB_ID=""
expect "then the Environment goes" 204 "$(as "$JAR" DELETE "/api/environments/$STAGING")"
expect "and is gone" 404 "$(as "$JAR" GET "/api/environments/$STAGING")"
expect "the last Environment may go too" 204 "$(as "$JAR" DELETE "/api/environments/$PRODUCTION")"
expect "leaving the Project empty" 0 "$(bakery GET "/api/projects/$PROJECT_ID" | json "len(d['project'].get('environments') or [])")"

say "Environments: all passed"
