#!/usr/bin/env bash
# End to end: Members, Roles, Invitations and API tokens. The Owner invites
# a member and a viewer (and later someone who already has an account), each Role is held to what it may do, API tokens act
# as their Member within their Permissions (read, read:sensitive, write,
# deploy, root) until they expire, and a Member removed from the Guild keeps
# their account but their Session reaches nothing and their tokens stop
# working. Needs `task dev` running (API on
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

say "Owner sets up a project with an Application, a Database and an Environment variable"
PROJECT_ID=$(bakery POST /api/projects "{\"name\":\"$RUN\"}" | json "d['project']['id']")
ENV_ID=$(bakery GET "/api/projects/$PROJECT_ID" | json "d['project']['environments'][0]['id']")
APP_ID=$(bakery POST "/api/environments/$ENV_ID/applications" \
	"{\"name\":\"$RUN\",\"build_pack\":\"dockerimage\",\"docker_image\":\"ghcr.io/traefik/whoami:v1.10\",\"port\":80}" | json "d['application']['id']")
APP_SLUG=$(bakery GET "/api/applications/$APP_ID" | json "d['application']['slug']")
APPS+=("$APP_ID $APP_SLUG")
bakery PUT "/api/applications/$APP_ID/environment-variables" '{"environment_variables":[{"name":"SECRET","value":"hunter2hunter2","runtime":true}]}' >/dev/null
DB_ID=$(bakery POST "/api/environments/$ENV_ID/databases" "{\"name\":\"$RUN\",\"type\":\"redis\"}" | json "d['database']['id']")

say "Invitations"
MEMBER=$WORK/member VIEWER=$WORK/viewer
M_TOKEN=$(invite "member-$RUN@example.com" member)
V_TOKEN=$(invite "viewer-$RUN@example.com" viewer)
expect "the link shows who is invited" 200 "$(as "$WORK/anon" GET "/api/invitations/by-token/$M_TOKEN")"
expect "the link names the Role" member "$(body "d['invitation']['role']")"
expect "the link names the guild" "Default" "$(body "d['guild']['name']")"
expect "a new email has no Member yet" False "$(body "d['existing_member']")"
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
NO=$(invite "no-$RUN@example.com" viewer)
expect "the invited person declines without a Session" 204 "$(as "$WORK/anon" POST "/api/invitations/by-token/$NO/decline")"
expect "a declined link is refused" 410 "$(join "$NO" "$WORK/no" No)"
expect "it is no longer listed" 0 "$(bakery GET /api/invitations | json "len([i for i in d['invitations'] if i['email']=='no-$RUN@example.com'])")"
expect "nobody is invited as owner" 422 "$(as "$JAR" POST /api/invitations "{\"email\":\"x-$RUN@example.com\",\"role\":\"owner\"}")"
as "$MEMBER" GET /api/me >/dev/null
expect "the member is a member" member "$(body "d['member']['role']")"
as "$VIEWER" GET /api/me >/dev/null
expect "the viewer is a viewer" viewer "$(body "d['member']['role']")"

say "What a viewer may do"
expect "viewer reads the project" 200 "$(as "$VIEWER" GET "/api/projects/$PROJECT_ID")"
expect "viewer cannot read Environment variables" 403 "$(as "$VIEWER" GET "/api/applications/$APP_ID/environment-variables")"
expect "viewer reads the Database" 200 "$(as "$VIEWER" GET "/api/databases/$DB_ID")"
expect "viewer sees no Database credentials" "None True" "$(body "d['database'].get('credentials'), d['database'].get('secrets_hidden')")"
expect "viewer cannot deploy" 403 "$(as "$VIEWER" POST "/api/applications/$APP_ID/deploy")"
expect "viewer cannot make a project" 403 "$(as "$VIEWER" POST /api/projects '{"name":"nope"}')"

say "What a member may do"
expect "member reads Environment variables" 200 "$(as "$MEMBER" GET "/api/applications/$APP_ID/environment-variables")"
expect "member sees Database credentials" True "$(as "$MEMBER" GET "/api/databases/$DB_ID" >/dev/null; body "d['database'].get('credentials') is not None")"
expect "member deploys" 201 "$(as "$MEMBER" POST "/api/applications/$APP_ID/deploy")"
wait_for 180 "the member's deployment" deployment_done
echo "ok: the deployment finished"
expect "member cannot add a Server" 403 "$(as "$MEMBER" POST /api/servers '{"name":"x","host":"192.0.2.1","user":"x"}')"
expect "member cannot change S3 storages" 403 "$(as "$MEMBER" POST /api/s3-storages '{}')"
expect "member may list S3 storages" 200 "$(as "$MEMBER" GET /api/s3-storages)"
expect "member lists the guild's Members" 200 "$(as "$MEMBER" GET /api/members)"
expect "member cannot list Invitations" 403 "$(as "$MEMBER" GET /api/invitations)"
expect "member cannot change a Role" 403 "$(as "$MEMBER" PUT "/api/members/${MEMBER_IDS[-1]}/roles/$(role_id Admin)")"
expect "the former role change is gone" 410 "$(as "$MEMBER" PATCH "/api/members/${MEMBER_IDS[-1]}" '{"role":"admin"}')"
expect "member cannot read Known hosts" 403 "$(as "$MEMBER" GET /api/known-hosts)"

say "Admins and the Owner"
expect "owner reads Known hosts" 200 "$(as "$JAR" GET /api/known-hosts)"
A_TOKEN=$(invite "admin-$RUN@example.com" admin)
ADMIN=$WORK/admin
expect "admin accepts" 201 "$(join "$A_TOKEN" "$ADMIN" Admin)"
MEMBER_IDS+=("$(body "d['member']['id']")")
OWNER_ID=$(bakery GET /api/me | json "d['member']['id']")
expect "admin cannot change the Owner's Roles" 403 "$(as "$ADMIN" PUT "/api/members/$OWNER_ID/roles/$(role_id Member)")"
expect "admin cannot remove the Owner" 403 "$(as "$ADMIN" DELETE "/api/members/$OWNER_ID")"
expect "admin makes the viewer a member" 200 "$(as "$ADMIN" PUT "/api/members/$VIEWER_ID/roles/$(role_id Member)")"
expect "the promoted viewer may change things at once" 201 "$(as "$VIEWER" POST /api/projects "{\"name\":\"$RUN-2\"}")"
bakery DELETE "/api/projects/$(body "d['project']['id']")" >/dev/null
bakery DELETE "/api/members/$VIEWER_ID/roles/$(role_id Member)" >/dev/null

say "API tokens"
expect "member makes a token the old way" 201 "$(as "$MEMBER" POST /api/api-tokens '{"name":"ci full"}')"
FULL=$(body "d['token']")
expect "an old-style token gets what the member's Role may grant" "deploy,read,read:sensitive,write" "$(body "','.join(d['api_token']['permissions'])")"
expect "member makes a read-only token the old way" 201 "$(as "$MEMBER" POST /api/api-tokens '{"name":"read","read_only":true}')"
READ=$(body "d['token']")
expect "the old read-only token is read" "read True" "$(body "' '.join(d['api_token']['permissions']), d['api_token']['read_only']")"
FULL_ID=$(as "$MEMBER" GET /api/api-tokens >/dev/null; body "[t['id'] for t in d['api_tokens'] if t['name']=='ci full'][0]")
expect "the token lists projects" 200 "$(with "$FULL" GET /api/projects)"
expect "the token deploys" 201 "$(with "$FULL" POST "/api/applications/$APP_ID/deploy")"
wait_for 180 "the token's deployment" deployment_done
echo "ok: the deployment finished"
expect "the read-only token reads" 200 "$(with "$READ" GET "/api/projects/$PROJECT_ID")"
expect "the read-only token cannot change anything" 403 "$(with "$READ" POST /api/projects '{"name":"nope"}')"
expect "the refusal names the Permission" "Missing required permissions: write" "$(body "d['message']")"
expect "the read-only token sees no Secrets" 403 "$(with "$READ" GET "/api/applications/$APP_ID/environment-variables")"
expect "the read-only token sees no Database credentials" "None True" "$(with "$READ" GET "/api/databases/$DB_ID" >/dev/null; body "d['database'].get('credentials'), d['database'].get('secrets_hidden')")"
expect "a token cannot make a token" 403 "$(with "$FULL" POST /api/api-tokens '{"name":"more"}')"
expect "the token was last used just now" True "$(as "$MEMBER" GET /api/api-tokens >/dev/null; body "[t for t in d['api_tokens'] if t['name']=='ci full'][0]['last_used_at'] is not None")"

say "API token Permissions"
expect "member lists what they may grant" "root:False write:True deploy:True read:True read:sensitive:True" "$(as "$MEMBER" GET /api/api-tokens/permissions >/dev/null; body "' '.join(p['name']+':'+str(p['allowed']) for p in d['permissions'])")"
expect "member may not grant root" 422 "$(as "$MEMBER" POST /api/api-tokens '{"name":"root","permissions":["root"]}')"
expect "viewer may not grant write" 422 "$(as "$VIEWER" POST /api/api-tokens '{"name":"write","permissions":["write"]}')"
expect "viewer makes a read token" 201 "$(as "$VIEWER" POST /api/api-tokens '{"name":"viewer read","permissions":["read"]}')"
expect "an unknown expiry is refused" 422 "$(as "$MEMBER" POST /api/api-tokens '{"name":"odd expiry","expires_in_days":3}')"
expect "member makes a read:sensitive token" 201 "$(as "$MEMBER" POST /api/api-tokens '{"name":"sensitive","permissions":["read:sensitive"]}')"
SENSITIVE=$(body "d['token']")
expect "read:sensitive brings read" "read,read:sensitive" "$(body "','.join(d['api_token']['permissions'])")"
expect "the read:sensitive token sees Secrets" 200 "$(with "$SENSITIVE" GET "/api/applications/$APP_ID/environment-variables")"
expect "the read:sensitive token sees Database credentials" True "$(with "$SENSITIVE" GET "/api/databases/$DB_ID" >/dev/null; body "d['database'].get('credentials') is not None")"
expect "the read:sensitive token cannot change anything" 403 "$(with "$SENSITIVE" PATCH "/api/projects/$PROJECT_ID" "{\"name\":\"$RUN\"}")"
expect "member makes a write token" 201 "$(as "$MEMBER" POST /api/api-tokens '{"name":"write","permissions":["write"]}')"
WRITE=$(body "d['token']")
expect "the write token changes a project" 200 "$(with "$WRITE" PATCH "/api/projects/$PROJECT_ID" "{\"name\":\"$RUN\"}")"
expect "the write token cannot deploy" 403 "$(with "$WRITE" POST "/api/applications/$APP_ID/deploy")"
expect "the write token cannot read" 403 "$(with "$WRITE" GET /api/projects)"
expect "member makes a deploy token expiring in a week" 201 "$(as "$MEMBER" POST /api/api-tokens '{"name":"ci deploy","permissions":["deploy"],"expires_in_days":7}')"
DEPLOY=$(body "d['token']")
DEPLOY_ID=$(body "d['api_token']['id']")
expect "it expires 7 days out" True "$(body "__import__('datetime').datetime.fromisoformat(d['api_token']['expires_at'].replace('Z','+00:00')) - __import__('datetime').datetime.now(__import__('datetime').timezone.utc) > __import__('datetime').timedelta(days=6, hours=23)")"
expect "the deploy token is listed" True "$(as "$MEMBER" GET /api/api-tokens >/dev/null; body "any(t['id']==$DEPLOY_ID and t['permissions']==['deploy'] and t['expires_at'] for t in d['api_tokens'])")"
expect "the deploy token deploys" 201 "$(with "$DEPLOY" POST "/api/applications/$APP_ID/deploy")"
wait_for 180 "the deploy token's deployment" deployment_done
echo "ok: the deployment finished"
expect "the deploy token cannot read projects" 403 "$(with "$DEPLOY" GET /api/projects)"
expect "the deploy token cannot change a project" 403 "$(with "$DEPLOY" PATCH "/api/projects/$PROJECT_ID" "{\"name\":\"$RUN\"}")"
expect "the Owner makes a root token" 201 "$(as "$JAR" POST /api/api-tokens "{\"name\":\"root $RUN\",\"permissions\":[\"root\",\"read\"]}")"
ROOT=$(body "d['token']") ROOT_ID=$(body "d['api_token']['id']")
expect "root stands alone" root "$(body "','.join(d['api_token']['permissions'])")"
expect "the root token sees Secrets" 200 "$(with "$ROOT" GET "/api/applications/$APP_ID/environment-variables")"
expect "the root token reads Known hosts" 200 "$(with "$ROOT" GET /api/known-hosts)"
expect "the root token lists Members" 200 "$(with "$ROOT" GET /api/members)"
bakery DELETE "/api/api-tokens/$ROOT_ID" >/dev/null
podman exec bakery-dev-postgres-1 psql -U bakery -d bakery -tAc "UPDATE api_tokens SET expires_at = now() - interval '1 minute' WHERE id = $DEPLOY_ID" >/dev/null
expect "an expired token is refused" 401 "$(with "$DEPLOY" POST "/api/applications/$APP_ID/deploy")"
expect "the expiry is not a Permission refusal" "invalid API token" "$(body "d['message']")"

say "Revoking"
expect "the member revokes the token" 204 "$(as "$MEMBER" DELETE "/api/api-tokens/$FULL_ID")"
expect "a revoked token is refused" 401 "$(with "$FULL" GET /api/projects)"
expect "a made-up token is refused" 401 "$(with "bky_madeup" GET /api/projects)"

say "Removing a Member"
MEMBER_ID=${MEMBER_IDS[0]}
expect "the Owner removes the member" 204 "$(as "$JAR" DELETE "/api/members/$MEMBER_ID")"
expect "the removed member's Session reaches nothing" 403 "$(as "$MEMBER" GET /api/projects)"
expect "because they are in no guild" "you are in no guild" "$(body "d['message']")"
expect "they keep their account" "None []" "$(as "$MEMBER" GET /api/me >/dev/null; body "d['guild'], d['guilds']")"
expect "the removed member's tokens stop working" 401 "$(with "$READ" GET /api/projects)"

say "Inviting someone who already has an account"
expect "inviting a Member of the guild is refused" 422 "$(as "$JAR" POST /api/invitations "{\"email\":\"viewer-$RUN@example.com\",\"role\":\"member\"}")"
BACK=$(invite "member-$RUN@example.com" viewer)
expect "the link knows the email has a Member" True "$(as "$WORK/anon" GET "/api/invitations/by-token/$BACK" >/dev/null; body "d['existing_member']")"
expect "accepting without their Session is refused" 401 "$(join "$BACK" "$WORK/anon" Nobody)"
expect "it says whom to sign in as" "sign in as member-$RUN@example.com to accept" "$(body "d['message']")"
expect "accepting as someone else is refused" 403 "$(join "$BACK" "$VIEWER" Viewer)"
expect "they accept with their Session" 200 "$(as "$MEMBER" POST "/api/invitations/by-token/$BACK/accept" '{}')"
expect "they are back, with the invited Role" "Default viewer" "$(as "$MEMBER" GET /api/me >/dev/null; body "d['guild']['name'], d['role']")"
expect "and list the guild" "['Default']" "$(body "[g['name'] for g in d['guilds']]")"
expect "the link works once" 410 "$(as "$MEMBER" POST "/api/invitations/by-token/$BACK/accept" '{}')"

say "PASS"
