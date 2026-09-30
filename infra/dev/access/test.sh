#!/usr/bin/env bash
# End to end: Members, Roles, Invitations and API tokens. The Owner invites
# a member and a viewer, each Role is held to what it may do, API tokens act
# as their Member (or as a viewer when read-only), and a removed Member's
# Session and tokens stop working. Needs `task dev` running (API on
# 127.0.0.1:4910). No git host: the Application is an image, from ghcr.io
# (Docker Hub pulls fail on a machine with a stale Docker Hub login).
set -euo pipefail

# Nothing here uses Forgejo; leave it as it is.
KEEP_FORGEJO=1
# shellcheck source=SCRIPTDIR/../lib/e2e.sh
. "$(dirname "$0")/../lib/e2e.sh"

DB_ID="" MEMBER_IDS=()
e2e_cleanup_hook() {
	[ -z "$DB_ID" ] || bakery DELETE "/api/databases/$DB_ID" >/dev/null
	local id
	for id in "${MEMBER_IDS[@]}"; do bakery DELETE "/api/members/$id" >/dev/null; done
	for id in $(bakery GET /api/invitations 2>/dev/null | json "' '.join(str(i['id']) for i in d['invitations'] if '$RUN' in i['email'])"); do
		bakery DELETE "/api/invitations/$id" >/dev/null
	done
}

# as JAR METHOD PATH [JSON]: the status code of a request with another
# Member's Session; the body is in $WORK/body.
as() {
	local args=(-s -o "$WORK/body" -w '%{http_code}' -b "$1" -c "$1" -X "$2" "$API$3")
	if [ $# -ge 4 ]; then args+=(-H 'Content-Type: application/json' -d "$4"); fi
	curl "${args[@]}"
}
# with TOKEN METHOD PATH [JSON]: the status code of a request with an API token.
with() {
	local args=(-s -o "$WORK/body" -w '%{http_code}' -H "Authorization: Bearer $1" -X "$2" "$API$3")
	if [ $# -ge 4 ]; then args+=(-H 'Content-Type: application/json' -d "$4"); fi
	curl "${args[@]}"
}
body() { json "$1" <"$WORK/body"; }
expect() { # expect DESCRIPTION WANT GOT
	[ "$2" = "$3" ] || fail "$1: got $3, want $2 ($(head -c 300 "$WORK/body" 2>/dev/null))"
	echo "ok: $1"
}
# invite EMAIL ROLE: the token of a new Invitation's link.
invite() {
	bakery POST /api/invitations "{\"email\":\"$1\",\"role\":\"$2\"}" | json "d['path'].rsplit('/', 1)[1]"
}
# join TOKEN JAR NAME: accepts an Invitation, signing in with JAR.
join() {
	as "$2" POST "/api/invitations/by-token/$1/accept" "{\"name\":\"$3\",\"password\":\"correct horse battery\"}"
}

sign_in

say "Owner sets up a project with an Application, a Database and an Env var"
PROJECT_ID=$(bakery POST /api/projects "{\"name\":\"$RUN\"}" | json "d['project']['id']")
ENV_ID=$(bakery GET "/api/projects/$PROJECT_ID" | json "d['project']['environments'][0]['id']")
APP_ID=$(bakery POST "/api/environments/$ENV_ID/applications" \
	"{\"name\":\"$RUN\",\"build_pack\":\"image\",\"image_reference\":\"ghcr.io/traefik/whoami:v1.10\",\"port\":80}" | json "d['application']['id']")
APP_SLUG=$(bakery GET "/api/applications/$APP_ID" | json "d['application']['slug']")
APPS+=("$APP_ID $APP_SLUG")
bakery PUT "/api/applications/$APP_ID/env" '{"env":[{"name":"SECRET","value":"hunter2hunter2","runtime":true}]}' >/dev/null
DB_ID=$(bakery POST "/api/environments/$ENV_ID/databases" "{\"name\":\"$RUN\",\"engine\":\"redis\"}" | json "d['database']['id']")

say "Invitations"
MEMBER=$WORK/member VIEWER=$WORK/viewer
M_TOKEN=$(invite "member-$RUN@example.com" member)
V_TOKEN=$(invite "viewer-$RUN@example.com" viewer)
expect "the link shows who is invited" 200 "$(as "$WORK/anon" GET "/api/invitations/by-token/$M_TOKEN")"
expect "the link names the Role" member "$(body "d['invitation']['role']")"
expect "member accepts" 201 "$(join "$M_TOKEN" "$MEMBER" Member)"
MEMBER_IDS+=("$(body "d['member']['id']")")
expect "viewer accepts" 201 "$(join "$V_TOKEN" "$VIEWER" Viewer)"
VIEWER_ID=$(body "d['member']['id']")
MEMBER_IDS+=("$VIEWER_ID")
expect "an accepted link is refused" 410 "$(join "$M_TOKEN" "$WORK/again" Again)"
GONE=$(invite "gone-$RUN@example.com" viewer)
GONE_ID=$(bakery GET /api/invitations | json "[i['id'] for i in d['invitations'] if i['email']=='gone-$RUN@example.com'][0]")
bakery DELETE "/api/invitations/$GONE_ID" >/dev/null
expect "a revoked link is refused" 410 "$(join "$GONE" "$WORK/gone" Gone)"
expect "nobody is invited as owner" 422 "$(as "$JAR" POST /api/invitations "{\"email\":\"x-$RUN@example.com\",\"role\":\"owner\"}")"
as "$MEMBER" GET /api/me >/dev/null
expect "the member is a member" member "$(body "d['member']['role']")"
as "$VIEWER" GET /api/me >/dev/null
expect "the viewer is a viewer" viewer "$(body "d['member']['role']")"

say "What a viewer may do"
expect "viewer reads the project" 200 "$(as "$VIEWER" GET "/api/projects/$PROJECT_ID")"
expect "viewer cannot read Env vars" 403 "$(as "$VIEWER" GET "/api/applications/$APP_ID/env")"
expect "viewer reads the Database" 200 "$(as "$VIEWER" GET "/api/databases/$DB_ID")"
expect "viewer sees no Database credentials" "None True" "$(body "d['database'].get('credentials'), d['database'].get('secrets_hidden')")"
expect "viewer cannot deploy" 403 "$(as "$VIEWER" POST "/api/applications/$APP_ID/deploy")"
expect "viewer cannot make a project" 403 "$(as "$VIEWER" POST /api/projects '{"name":"nope"}')"

say "What a member may do"
expect "member reads Env vars" 200 "$(as "$MEMBER" GET "/api/applications/$APP_ID/env")"
expect "member sees Database credentials" True "$(as "$MEMBER" GET "/api/databases/$DB_ID" >/dev/null; body "d['database'].get('credentials') is not None")"
expect "member deploys" 201 "$(as "$MEMBER" POST "/api/applications/$APP_ID/deploy")"
wait_for 180 "the member's deployment" deployment_done
echo "ok: the deployment finished"
expect "member cannot add a Server" 403 "$(as "$MEMBER" POST /api/servers '{"name":"x","host":"192.0.2.1","user":"x"}')"
expect "member cannot change S3 storages" 403 "$(as "$MEMBER" POST /api/s3-storages '{}')"
expect "member may list S3 storages" 200 "$(as "$MEMBER" GET /api/s3-storages)"
expect "member cannot list Members" 403 "$(as "$MEMBER" GET /api/members)"
expect "member cannot read Known hosts" 403 "$(as "$MEMBER" GET /api/known-hosts)"

say "Admins and the Owner"
expect "owner reads Known hosts" 200 "$(as "$JAR" GET /api/known-hosts)"
A_TOKEN=$(invite "admin-$RUN@example.com" admin)
ADMIN=$WORK/admin
expect "admin accepts" 201 "$(join "$A_TOKEN" "$ADMIN" Admin)"
MEMBER_IDS+=("$(body "d['member']['id']")")
OWNER_ID=$(bakery GET /api/me | json "d['member']['id']")
expect "admin cannot change the Owner's Role" 403 "$(as "$ADMIN" PATCH "/api/members/$OWNER_ID" '{"role":"member"}')"
expect "admin cannot remove the Owner" 403 "$(as "$ADMIN" DELETE "/api/members/$OWNER_ID")"
expect "admin makes the viewer a member" 200 "$(as "$ADMIN" PATCH "/api/members/$VIEWER_ID" '{"role":"member"}')"
expect "the promoted viewer may change things at once" 201 "$(as "$VIEWER" POST /api/projects "{\"name\":\"$RUN-2\"}")"
bakery DELETE "/api/projects/$(body "d['project']['id']")" >/dev/null
bakery PATCH "/api/members/$VIEWER_ID" '{"role":"viewer"}' >/dev/null

say "API tokens"
expect "member makes a token" 201 "$(as "$MEMBER" POST /api/api-tokens '{"name":"ci"}')"
FULL=$(body "d['token']")
expect "member makes a read-only token" 201 "$(as "$MEMBER" POST /api/api-tokens '{"name":"read","read_only":true}')"
READ=$(body "d['token']")
FULL_ID=$(as "$MEMBER" GET /api/api-tokens >/dev/null; body "[t['id'] for t in d['api_tokens'] if t['name']=='ci'][0]")
expect "the token lists projects" 200 "$(with "$FULL" GET /api/projects)"
expect "the token deploys" 201 "$(with "$FULL" POST "/api/applications/$APP_ID/deploy")"
wait_for 180 "the token's deployment" deployment_done
echo "ok: the deployment finished"
expect "the read-only token reads" 200 "$(with "$READ" GET "/api/projects/$PROJECT_ID")"
expect "the read-only token cannot change anything" 403 "$(with "$READ" POST /api/projects '{"name":"nope"}')"
expect "the read-only token sees no Secrets" 403 "$(with "$READ" GET "/api/applications/$APP_ID/env")"
expect "a token cannot make a token" 403 "$(with "$FULL" POST /api/api-tokens '{"name":"more"}')"
expect "the token was last used just now" True "$(as "$MEMBER" GET /api/api-tokens >/dev/null; body "[t for t in d['api_tokens'] if t['name']=='ci'][0]['last_used_at'] is not None")"
expect "the member revokes the token" 204 "$(as "$MEMBER" DELETE "/api/api-tokens/$FULL_ID")"
expect "a revoked token is refused" 401 "$(with "$FULL" GET /api/projects)"
expect "a made-up token is refused" 401 "$(with "bky_madeup" GET /api/projects)"

say "Removing a Member"
MEMBER_ID=${MEMBER_IDS[0]}
expect "the Owner removes the member" 204 "$(as "$JAR" DELETE "/api/members/$MEMBER_ID")"
expect "the removed member's Session stops working" 401 "$(as "$MEMBER" GET /api/projects)"
expect "the removed member's tokens stop working" 401 "$(with "$READ" GET /api/projects)"

say "PASS"
