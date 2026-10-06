#!/usr/bin/env bash
# End to end: Guilds keep to themselves. The Owner (the Instance admin) is in
# the first Guild, "Default", and makes a second one through the API, inviting
# a new person as its admin and a viewer of "Default" as its member; Roles are
# per Guild. What is made in the
# second (a Project with an Application, a Database and a Service, a Remote
# server, an S3 storage, a Notification channel and a Known host) is not
# listed, readable or changeable from the first, with a Session or an API
# token, and the other way round. Every Guild sees the Local server, and
# only the Instance admin changes it. The second Guild cannot be deleted while
# it owns anything, and can once it is empty. Needs `task dev` running (API on
# 127.0.0.1:4910) and the dev Postgres (`task db:up`).
set -euo pipefail

# Nothing here uses Forgejo; leave it as it is.
KEEP_FORGEJO=1
# shellcheck source=SCRIPTDIR/../lib/e2e.sh
. "$(dirname "$0")/../lib/e2e.sh"

PSQL=(podman compose -f "$ROOT/infra/dev/compose.yml" exec -T postgres psql -U bakery -d bakery -tAq)
GUILD_B="" B_PROJECT_ID="" B_APP_ID="" B_DB_ID="" B_SERVICE_ID="" TOKEN_IDS=()
B_SERVER_ID="" B_STORAGE_ID="" B_CHANNEL_ID="" A_CHANNEL_ID="" V_PROJECT_ID="" V_APP_ID="" VIEWER_ID=""
e2e_cleanup_hook() {
	[ -z "$V_APP_ID" ] || in_b DELETE "/api/applications/$V_APP_ID" >/dev/null
	[ -z "$V_PROJECT_ID" ] || in_b DELETE "/api/projects/$V_PROJECT_ID" >/dev/null
	[ -z "$B_SERVER_ID" ] || in_b DELETE "/api/servers/$B_SERVER_ID" >/dev/null
	[ -z "$B_STORAGE_ID" ] || in_b DELETE "/api/s3-storages/$B_STORAGE_ID" >/dev/null
	[ -z "$B_CHANNEL_ID" ] || in_b DELETE "/api/notification-channels/$B_CHANNEL_ID" >/dev/null
	[ -z "$A_CHANNEL_ID" ] || bakery DELETE "/api/notification-channels/$A_CHANNEL_ID" >/dev/null
	[ -z "$B_SERVICE_ID" ] || in_b DELETE "/api/services/$B_SERVICE_ID" >/dev/null
	[ -z "$B_DB_ID" ] || in_b DELETE "/api/databases/$B_DB_ID" >/dev/null
	[ -z "$B_APP_ID" ] || in_b DELETE "/api/applications/$B_APP_ID" >/dev/null
	[ -z "$B_PROJECT_ID" ] || in_b DELETE "/api/projects/$B_PROJECT_ID" >/dev/null
	local id
	for id in "${TOKEN_IDS[@]}"; do curl -s -b "$JAR" -b "bakery_guild=${id%:*}" -X DELETE "$API/api/api-tokens/${id#*:}" >/dev/null; done
	[ -z "$VIEWER_ID" ] || bakery DELETE "/api/members/$VIEWER_ID" >/dev/null
	if [ -n "$GUILD_B" ]; then
		"${PSQL[@]}" -c "DELETE FROM known_hosts WHERE guild_id = $GUILD_B; DELETE FROM invitations WHERE guild_id = $GUILD_B; DELETE FROM memberships WHERE guild_id = $GUILD_B; DELETE FROM guilds WHERE id = $GUILD_B" >/dev/null
	fi
}

# in_b METHOD PATH [JSON]: the Owner's Session acting in the second Guild.
in_b() {
	local args=(-sS -b "$JAR" -b "bakery_guild=$GUILD_B" -X "$1" "$API$2")
	if [ $# -ge 3 ]; then args+=(-H 'Content-Type: application/json' -d "$3"); fi
	curl "${args[@]}"
}
# status_in GUILD METHOD PATH [JSON]: the status code of a request in a Guild
# with the Owner's Session; the body is in $WORK/body.
status_in() {
	local args=(-s --max-time 10 -o "$WORK/body" -w '%{http_code}' -b "$JAR" -b "bakery_guild=$1" -X "$2" "$API$3")
	if [ $# -ge 4 ]; then args+=(-H 'Content-Type: application/json' -d "$4"); fi
	curl "${args[@]}"
}
# with TOKEN METHOD PATH: the status code of a request with an API token.
with() {
	curl -s -o "$WORK/body" -w '%{http_code}' -H "Authorization: Bearer $1" -X "$2" "$API$3"
}
# as JAR METHOD PATH [JSON]: the status code of a request with another
# Member's Session, keeping the cookies it sets; the body is in $WORK/body.
as() {
	local args=(-s --max-time 10 -o "$WORK/body" -w '%{http_code}' -b "$1" -c "$1" -X "$2" "$API$3")
	if [ $# -ge 4 ]; then args+=(-H 'Content-Type: application/json' -d "$4"); fi
	curl "${args[@]}"
}
body() { json "$1" <"$WORK/body"; }
expect() { # expect DESCRIPTION WANT GOT
	[ "$2" = "$3" ] || fail "$1: got $3, want $2 ($(head -c 300 "$WORK/body" 2>/dev/null))"
	echo "ok: $1"
}

sign_in
GUILD_A=$(bakery GET /api/me | json "d['guild']['id']")

say "The Instance admin makes a second Guild"
expect "the Owner creates it" 201 "$(curl -s -o "$WORK/body" -w '%{http_code}' -b "$JAR" -H 'Content-Type: application/json' -X POST "$API/api/guilds" -d "{\"name\":\"$RUN\",\"description\":\"made by the guilds e2e\"}")"
GUILD_B=$(body "d['guild']['id']")
expect "as its admin" admin "$(body "d['guild']['role']")"
expect "the Owner acts in the second Guild" "$GUILD_B" "$(in_b GET /api/me | json "d['guild']['id']")"
expect "both Guilds are listed" True "$(bakery GET /api/guilds | json "{g['id'] for g in d['guilds']} >= {$GUILD_A, $GUILD_B}")"
expect "the second Guild owns nothing yet" "[]" "$(in_b GET /api/guilds/current | json "d['guild']['blocking']")"
expect "it is renamed and described" "$RUN-b described" "$(in_b PATCH /api/guilds/current "{\"name\":\"$RUN-b\",\"description\":\"described\"}" | json "d['guild']['name'] + ' ' + d['guild']['description']")"
# A copy of the Owner's cookies, so the switching leaves $JAR without a
# Current guild cookie (in_b and status_in add their own).
SWITCHER=$WORK/switcher
cp "$JAR" "$SWITCHER"
expect "switching to it" 204 "$(as "$SWITCHER" POST "/api/guilds/$GUILD_B/switch")"
expect "makes it the Session's Current guild" "$GUILD_B" "$(as "$SWITCHER" GET /api/me >/dev/null; body "d['guild']['id']")"
expect "switching back" 204 "$(as "$SWITCHER" POST "/api/guilds/$GUILD_A/switch")"
expect "switching to a Guild that does not exist" 404 "$(as "$SWITCHER" POST /api/guilds/999999999/switch)"

say "A viewer of the first Guild and a new person join the second"
VIEWER=$WORK/viewer NEWCOMER=$WORK/newcomer
V_INVITE=$(bakery POST /api/invitations "{\"email\":\"viewer-$RUN@example.com\",\"role\":\"viewer\"}" | json "d['path'].rsplit('/', 1)[1]")
expect "the viewer joins the first Guild" 201 "$(as "$VIEWER" POST "/api/invitations/by-token/$V_INVITE/accept" '{"name":"Viewer","password":"correct horse battery"}')"
VIEWER_ID=$(as "$VIEWER" GET /api/me >/dev/null; body "d['member']['id']")
expect "a viewer cannot make a Guild's Invitations" 403 "$(as "$VIEWER" GET /api/invitations)"
VB_INVITE=$(in_b POST /api/invitations "{\"email\":\"viewer-$RUN@example.com\",\"role\":\"member\"}" | json "d['path'].rsplit('/', 1)[1]")
expect "the link knows the viewer has an account" True "$(as "$WORK/anon" GET "/api/invitations/by-token/$VB_INVITE" >/dev/null; body "d['existing_member']")"
expect "the viewer accepts with their Session" 200 "$(as "$VIEWER" POST "/api/invitations/by-token/$VB_INVITE/accept" '{}')"
expect "and is in both, with a Role in each" "[('Default', 'viewer'), ('$RUN-b', 'member')]" "$(as "$VIEWER" GET /api/guilds >/dev/null; body "sorted((g['name'], g['role']) for g in d['guilds'])")"
N_INVITE=$(in_b POST /api/invitations "{\"email\":\"newcomer-$RUN@example.com\",\"role\":\"admin\"}" | json "d['path'].rsplit('/', 1)[1]")
expect "a new person accepts" 201 "$(as "$NEWCOMER" POST "/api/invitations/by-token/$N_INVITE/accept" '{"name":"Newcomer","password":"correct horse battery"}')"
expect "and is only in the second Guild, as its admin" "[('$RUN-b', 'admin')]" "$(as "$NEWCOMER" GET /api/guilds >/dev/null; body "[(g['name'], g['role']) for g in d['guilds']]")"
expect "they cannot switch to the first Guild" 404 "$(as "$NEWCOMER" POST "/api/guilds/$GUILD_A/switch")"
expect "the second Guild lists its three Members" 3 "$(in_b GET /api/members | json "len(d['members'])")"

say "A Role is per Guild"
expect "the viewer switches to the second Guild" 204 "$(as "$VIEWER" POST "/api/guilds/$GUILD_B/switch")"
expect "where, as a member, they make a Project" 201 "$(as "$VIEWER" POST /api/projects "{\"name\":\"$RUN-v\"}")"
V_PROJECT_ID=$(body "d['project']['id']")
V_ENV_ID=$(as "$VIEWER" GET "/api/projects/$V_PROJECT_ID" >/dev/null; body "d['project']['environments'][0]['id']")
expect "and an Application" 201 "$(as "$VIEWER" POST "/api/environments/$V_ENV_ID/applications" "{\"name\":\"$RUN-v\",\"build_pack\":\"dockerimage\",\"docker_image\":\"ghcr.io/traefik/whoami:v1.10\",\"port\":80}")"
V_APP_ID=$(body "d['application']['id']")
expect "the viewer switches back to the first Guild" 204 "$(as "$VIEWER" POST "/api/guilds/$GUILD_A/switch")"
expect "where they are still a viewer" viewer "$(as "$VIEWER" GET /api/me >/dev/null; body "d['role']")"
expect "and are refused a write" 403 "$(as "$VIEWER" POST /api/projects "{\"name\":\"$RUN-nope\"}")"
expect "and cannot read their own Project of the second Guild" 404 "$(as "$VIEWER" GET "/api/projects/$V_PROJECT_ID")"

say "The second Guild gets a Project with an Application, a Database and a Service"
B_PROJECT_ID=$(in_b POST /api/projects "{\"name\":\"$RUN\"}" | json "d['project']['id']")
B_ENV_ID=$(in_b GET "/api/projects/$B_PROJECT_ID" | json "d['project']['environments'][0]['id']")
B_APP_ID=$(in_b POST "/api/environments/$B_ENV_ID/applications" \
	"{\"name\":\"$RUN\",\"build_pack\":\"dockerimage\",\"docker_image\":\"ghcr.io/traefik/whoami:v1.10\",\"port\":80}" | json "d['application']['id']")
B_DB_ID=$(in_b POST "/api/environments/$B_ENV_ID/databases" "{\"name\":\"$RUN\",\"type\":\"postgresql\"}" | json "d['database']['id']")
B_SCHEDULE_ID=$(in_b GET "/api/databases/$B_DB_ID/scheduled-backups" | json "d['scheduled_backups'][0]['id'] if d['scheduled_backups'] else ''")
B_SERVICE_ID=$(in_b POST "/api/environments/$B_ENV_ID/services" "{\"template\":\"whoami\",\"name\":\"$RUN-whoami\"}" | json "d['service']['id']")
expect "the second Guild lists its Project" True "$(in_b GET /api/projects | json "any(p['id']==$B_PROJECT_ID for p in d['projects'])")"

say "The first Guild does not see the second Guild's Project"
expect "absent from the first Guild's Projects" False "$(bakery GET /api/projects | json "any(p['id']==$B_PROJECT_ID for p in d['projects'])")"
for route in \
	"GET /api/projects/$B_PROJECT_ID" \
	"PATCH /api/projects/$B_PROJECT_ID" \
	"DELETE /api/projects/$B_PROJECT_ID" \
	"GET /api/projects/$B_PROJECT_ID/variables" \
	"POST /api/projects/$B_PROJECT_ID/environments" \
	"GET /api/projects/$B_PROJECT_ID/databases" \
	"GET /api/projects/$B_PROJECT_ID/services" \
	"GET /api/environments/$B_ENV_ID" \
	"DELETE /api/environments/$B_ENV_ID" \
	"POST /api/environments/$B_ENV_ID/applications" \
	"POST /api/environments/$B_ENV_ID/databases" \
	"POST /api/environments/$B_ENV_ID/services" \
	"GET /api/applications/$B_APP_ID" \
	"PATCH /api/applications/$B_APP_ID" \
	"GET /api/applications/$B_APP_ID/environment-variables" \
	"GET /api/applications/$B_APP_ID/deployments" \
	"GET /api/applications/$B_APP_ID/status" \
	"GET /api/applications/$B_APP_ID/previews" \
	"GET /api/applications/$B_APP_ID/webhook" \
	"GET /api/applications/$B_APP_ID/routing" \
	"GET /api/applications/$B_APP_ID/logs" \
	"POST /api/applications/$B_APP_ID/deploy" \
	"DELETE /api/applications/$B_APP_ID" \
	"GET /api/databases/$B_DB_ID" \
	"GET /api/databases/$B_DB_ID/backup-executions" \
	"GET /api/databases/$B_DB_ID/scheduled-backups" \
	"GET /api/databases/$B_DB_ID/logs" \
	"POST /api/databases/$B_DB_ID/stop" \
	"DELETE /api/databases/$B_DB_ID" \
	"GET /api/services/$B_SERVICE_ID" \
	"POST /api/services/$B_SERVICE_ID/restart" \
	"GET /api/services/$B_SERVICE_ID/components/whoami/logs" \
	"DELETE /api/services/$B_SERVICE_ID"; do
	expect "$route from the first Guild" 404 "$(status_in "$GUILD_A" "${route% *}" "${route#* }" '{}')"
done
if [ -n "$B_SCHEDULE_ID" ]; then
	expect "the Scheduled backup from the first Guild" 404 "$(status_in "$GUILD_A" GET "/api/scheduled-backups/$B_SCHEDULE_ID")"
fi
expect "the second Guild still reads its Application" 200 "$(status_in "$GUILD_B" GET "/api/applications/$B_APP_ID")"

say "The second Guild does not see the first Guild's Projects"
A_PROJECT_ID=$(bakery POST /api/projects "{\"name\":\"$RUN-a\"}" | json "d['project']['id']")
PROJECT_ID=$A_PROJECT_ID
expect "absent from the second Guild's Projects" False "$(in_b GET /api/projects | json "any(p['id']==$A_PROJECT_ID for p in d['projects'])")"
expect "the first Guild's Project from the second" 404 "$(status_in "$GUILD_B" GET "/api/projects/$A_PROJECT_ID")"

say "An API token works only in the Guild it was made in"
A_TOKEN=$(bakery POST /api/api-tokens "{\"name\":\"$RUN\",\"permissions\":[\"read\"]}" | json "d['token']")
TOKEN_IDS+=("$GUILD_A:$(bakery GET /api/api-tokens | json "[t['id'] for t in d['api_tokens'] if t['name']=='$RUN'][0]")")
B_TOKEN=$(in_b POST /api/api-tokens "{\"name\":\"$RUN\",\"permissions\":[\"read\"]}" | json "d['token']")
TOKEN_IDS+=("$GUILD_B:$(in_b GET /api/api-tokens | json "[t['id'] for t in d['api_tokens'] if t['name']=='$RUN'][0]")")
expect "the first Guild's token cannot read the second Guild's Project" 404 "$(with "$A_TOKEN" GET "/api/projects/$B_PROJECT_ID")"
expect "the first Guild's token cannot read the second Guild's Application" 404 "$(with "$A_TOKEN" GET "/api/applications/$B_APP_ID")"
expect "the first Guild's token does not list it" False "$(with "$A_TOKEN" GET /api/projects >/dev/null; body "any(p['id']==$B_PROJECT_ID for p in d['projects'])")"
expect "the second Guild's token reads its Project" 200 "$(with "$B_TOKEN" GET "/api/projects/$B_PROJECT_ID")"
expect "the second Guild's token cannot read the first Guild's Project" 404 "$(with "$B_TOKEN" GET "/api/projects/$A_PROJECT_ID")"

say "The second Guild gets a Remote server, an S3 storage, a Notification channel and a Known host"
B_SERVER_ID=$(in_b POST /api/servers "{\"name\":\"$RUN\",\"host\":\"203.0.113.7\",\"user\":\"bakery\"}" | json "d['server']['id']")
B_STORAGE_ID=$(in_b POST /api/s3-storages "{\"name\":\"$RUN\",\"endpoint\":\"http://127.0.0.1:4960\",\"bucket\":\"bakery-backups\",\"access_key\":\"a\",\"secret_key\":\"s\"}" | json "d['s3_storage']['id']")
B_CHANNEL_ID=$(in_b POST /api/notification-channels "{\"name\":\"$RUN\",\"kind\":\"webhook\",\"settings\":{\"url\":\"http://127.0.0.1:4988/hook\"}}" | json "d['channel']['id']")
B_HOST_ID=$("${PSQL[@]}" -c "INSERT INTO known_hosts (guild_id, host, keys, created_at, updated_at) VALUES ($GUILD_B, '$RUN.example', '$RUN.example ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl', now(), now()) RETURNING id" | head -1)
LOCAL_ID=$(in_b GET /api/servers | json "[s['id'] for s in d['servers'] if s['kind']=='local'][0]")
expect "the second Guild lists the Local server and its own" "['local', 'remote']" "$(in_b GET /api/servers | json "sorted(s['kind'] for s in d['servers'] if s['kind']=='local' or s['id']==$B_SERVER_ID)")"
expect "the second Guild lists its S3 storage" True "$(in_b GET /api/s3-storages | json "any(s['id']==$B_STORAGE_ID for s in d['s3_storages'])")"
expect "the second Guild lists its channel" True "$(in_b GET /api/notification-channels | json "any(c['id']==$B_CHANNEL_ID for c in d['channels'])")"
expect "the second Guild lists its Known host" True "$(in_b GET /api/known-hosts | json "any(h['id']==$B_HOST_ID for h in d['known_hosts'])")"

say "The first Guild does not see them"
expect "the Remote server is absent from the first Guild's Servers" False "$(bakery GET /api/servers | json "any(s['id']==$B_SERVER_ID for s in d['servers'])")"
expect "the first Guild still lists the Local server" True "$(bakery GET /api/servers | json "any(s['id']==$LOCAL_ID for s in d['servers'])")"
expect "the S3 storage is absent from the first Guild's" False "$(bakery GET /api/s3-storages | json "any(s['id']==$B_STORAGE_ID for s in d['s3_storages'])")"
expect "the channel is absent from the first Guild's" False "$(bakery GET /api/notification-channels | json "any(c['id']==$B_CHANNEL_ID for c in d['channels'])")"
expect "the Known host is absent from the first Guild's" False "$(bakery GET /api/known-hosts | json "any(h['id']==$B_HOST_ID for h in d['known_hosts'])")"
for route in \
	"GET /api/servers/$B_SERVER_ID" \
	"GET /api/servers/$B_SERVER_ID/metrics" \
	"GET /api/servers/$B_SERVER_ID/details" \
	"PATCH /api/servers/$B_SERVER_ID" \
	"POST /api/servers/$B_SERVER_ID/validate" \
	"POST /api/servers/$B_SERVER_ID/cleanup" \
	"DELETE /api/servers/$B_SERVER_ID/host-key" \
	"DELETE /api/servers/$B_SERVER_ID" \
	"PATCH /api/s3-storages/$B_STORAGE_ID" \
	"DELETE /api/s3-storages/$B_STORAGE_ID" \
	"GET /api/notification-channels/$B_CHANNEL_ID" \
	"PATCH /api/notification-channels/$B_CHANNEL_ID" \
	"POST /api/notification-channels/$B_CHANNEL_ID/test" \
	"GET /api/notification-channels/$B_CHANNEL_ID/deliveries" \
	"DELETE /api/notification-channels/$B_CHANNEL_ID" \
	"DELETE /api/known-hosts/$B_HOST_ID"; do
	expect "$route from the first Guild" 404 "$(status_in "$GUILD_A" "${route% *}" "${route#* }" '{}')"
done
expect "testing the second Guild's S3 storage from the first" 404 "$(status_in "$GUILD_A" POST /api/s3-storages/check "{\"id\":$B_STORAGE_ID,\"name\":\"x\",\"endpoint\":\"http://127.0.0.1:4960\",\"bucket\":\"bakery-backups\",\"access_key\":\"a\"}")"
A_PROJECT_ENV=$(bakery GET "/api/projects/$A_PROJECT_ID" | json "d['project']['environments'][0]['id']")
expect "an Application of the first Guild cannot target the second Guild's Server" 422 "$(status_in "$GUILD_A" POST "/api/environments/$A_PROJECT_ENV/applications" "{\"name\":\"$RUN-x\",\"build_pack\":\"dockerimage\",\"docker_image\":\"ghcr.io/traefik/whoami:v1.10\",\"port\":80,\"server_id\":$B_SERVER_ID}")"
expect "a Scheduled backup of the second Guild cannot use the first Guild's S3 storage" 422 "$(
	A_STORAGE_ID=$(bakery POST /api/s3-storages "{\"name\":\"$RUN-a\",\"endpoint\":\"http://127.0.0.1:4960\",\"bucket\":\"bakery-backups\",\"access_key\":\"a\",\"secret_key\":\"s\"}" | json "d['s3_storage']['id']")
	status_in "$GUILD_B" POST "/api/databases/$B_DB_ID/scheduled-backups" "{\"cron\":\"0 3 * * *\",\"retention\":7,\"s3_storage_id\":$A_STORAGE_ID}"
	bakery DELETE "/api/s3-storages/$A_STORAGE_ID" >/dev/null
)"
expect "the same names are free in the first Guild" 201 "$(status_in "$GUILD_A" POST /api/servers "{\"name\":\"$RUN\",\"host\":\"203.0.113.7\",\"user\":\"bakery\"}")"
bakery DELETE "/api/servers/$(body "d['server']['id']")" >/dev/null
expect "the second Guild still reads its Server" 200 "$(status_in "$GUILD_B" GET "/api/servers/$B_SERVER_ID")"

say "A guild's events reach only its own channels"
A_CHANNEL_ID=$(bakery POST /api/notification-channels "{\"name\":\"$RUN-a\",\"kind\":\"webhook\",\"settings\":{\"url\":\"http://127.0.0.1:4988/hook\"},\"event_kinds\":[\"backup_success\",\"backup_failure\"]}" | json "d['channel']['id']")
in_b PATCH "/api/notification-channels/$B_CHANNEL_ID" "{\"name\":\"$RUN\",\"kind\":\"webhook\",\"settings\":{\"url\":\"http://127.0.0.1:4988/hook\"},\"event_kinds\":[\"backup_success\",\"backup_failure\"]}" >/dev/null
wait_for 60 "the second Guild's Database runs" sh -c "curl -s -b '$JAR' -b 'bakery_guild=$GUILD_B' '$API/api/databases/$B_DB_ID' | grep -q '\"status\":\"running\"'"
in_b POST "/api/databases/$B_DB_ID/backup-executions" '{}' >/dev/null
wait_for 60 "the second Guild's channel hears of the Backup execution" sh -c "curl -s -b '$JAR' -b 'bakery_guild=$GUILD_B' '$API/api/notification-channels/$B_CHANNEL_ID/deliveries' | grep -q 'Backup of'"
expect "the first Guild's channel did not" 0 "$(bakery GET "/api/notification-channels/$A_CHANNEL_ID/deliveries" | json "len(d['deliveries'])")"

say "Every Guild uses the Local server; only the Instance admin changes it"
LOCAL_NAME=$(in_b GET "/api/servers/$LOCAL_ID" | json "d['server']['name']")
expect "the Instance admin renames it from the second Guild" 200 "$(status_in "$GUILD_B" PATCH "/api/servers/$LOCAL_ID" "{\"name\":\"$LOCAL_NAME\",\"description\":\"\"}")"
as_admin() { as "$NEWCOMER" "$1" "$2" "${3:-{\}}"; }
expect "another admin of the second Guild reads the Local server" 200 "$(as_admin GET "/api/servers/$LOCAL_ID")"
expect "but may not change it" 403 "$(as_admin PATCH "/api/servers/$LOCAL_ID" "{\"name\":\"$LOCAL_NAME\"}")"
expect "with the reason" "the local server is the instance's" "$(body "d['message']")"
expect "nor clean it up" 403 "$(as_admin POST "/api/servers/$LOCAL_ID/cleanup")"
expect "and sees only the second Guild's Containers on it" True "$(as_admin GET "/api/servers/$LOCAL_ID/metrics" >/dev/null; body "all(c['owner'] in ('proxy',) or (c['owner']=='database' and c['owner_id']=='$B_DB_ID') or (c['owner']=='service' and c['owner_id']=='$B_SERVICE_ID') or (c['owner']=='application' and c['owner_id']=='$B_APP_ID') for c in d['containers'])")"
expect "they still change the second Guild's own Server" 200 "$(as_admin PATCH "/api/servers/$B_SERVER_ID" "{\"name\":\"$RUN\",\"host\":\"203.0.113.8\",\"user\":\"bakery\"}")"

say "A Guild is deleted only once it owns nothing"
expect "deleting the second Guild is refused" 409 "$(status_in "$GUILD_B" DELETE /api/guilds/current)"
expect "with what it still owns" True "$(body "'projects' in d['blocking']")"
expect "a member of the second Guild may not delete it" 403 "$(as "$VIEWER" POST "/api/guilds/$GUILD_B/switch" >/dev/null; as "$VIEWER" DELETE /api/guilds/current)"
for path in "/api/applications/$V_APP_ID" "/api/projects/$V_PROJECT_ID" "/api/services/$B_SERVICE_ID" "/api/databases/$B_DB_ID" "/api/applications/$B_APP_ID" "/api/projects/$B_PROJECT_ID" \
	"/api/servers/$B_SERVER_ID" "/api/s3-storages/$B_STORAGE_ID" "/api/notification-channels/$B_CHANNEL_ID"; do
	expect "DELETE $path in the second Guild" 204 "$(status_in "$GUILD_B" DELETE "$path")"
done
V_APP_ID="" V_PROJECT_ID="" B_SERVICE_ID="" B_DB_ID="" B_APP_ID="" B_PROJECT_ID="" B_SERVER_ID="" B_STORAGE_ID="" B_CHANNEL_ID=""
expect "an empty Guild owns nothing" "[]" "$(in_b GET /api/guilds/current | json "d['guild']['blocking']")"
expect "and is deleted" 204 "$(status_in "$GUILD_B" DELETE /api/guilds/current)"
TOKEN_IDS=("$GUILD_A:${TOKEN_IDS[0]#*:}")
GUILD_B_GONE=$GUILD_B GUILD_B=""
expect "its token stops working" 401 "$(with "$B_TOKEN" GET /api/projects)"
expect "the viewer is left in the first Guild only" "['Default']" "$(as "$VIEWER" GET /api/guilds >/dev/null; body "[g['name'] for g in d['guilds']]")"
expect "and acts there again" viewer "$(as "$VIEWER" GET /api/me >/dev/null; body "d['role']")"
expect "the new person is left in no Guild" "None []" "$(as "$NEWCOMER" GET /api/me >/dev/null; body "d['guild'], d['guilds']")"
expect "the Owner no longer lists it" False "$(bakery GET /api/guilds | json "any(g['id']==$GUILD_B_GONE for g in d['guilds'])")"

say "Guilds keep to themselves"
