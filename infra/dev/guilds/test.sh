#!/usr/bin/env bash
# End to end: Guilds keep to themselves. The Owner is in the first Guild,
# "Default", and in a second Guild made for the test; what is made in the
# second (a Project with an Application, a Database and a Service) is not
# listed, readable or changeable from the first, with a Session or an API
# token, and the other way round. Needs `task dev` running (API on
# 127.0.0.1:4910) and the dev Postgres (`task db:up`).
set -euo pipefail

# Nothing here uses Forgejo; leave it as it is.
KEEP_FORGEJO=1
# shellcheck source=SCRIPTDIR/../lib/e2e.sh
. "$(dirname "$0")/../lib/e2e.sh"

PSQL=(podman compose -f "$ROOT/infra/dev/compose.yml" exec -T postgres psql -U bakery -d bakery -tAq)
GUILD_B="" B_PROJECT_ID="" B_APP_ID="" B_DB_ID="" B_SERVICE_ID="" TOKEN_IDS=()
e2e_cleanup_hook() {
	[ -z "$B_SERVICE_ID" ] || in_b DELETE "/api/services/$B_SERVICE_ID" >/dev/null
	[ -z "$B_DB_ID" ] || in_b DELETE "/api/databases/$B_DB_ID" >/dev/null
	[ -z "$B_APP_ID" ] || in_b DELETE "/api/applications/$B_APP_ID" >/dev/null
	[ -z "$B_PROJECT_ID" ] || in_b DELETE "/api/projects/$B_PROJECT_ID" >/dev/null
	local id
	for id in "${TOKEN_IDS[@]}"; do curl -s -b "$JAR" -b "bakery_guild=${id%:*}" -X DELETE "$API/api/api-tokens/${id#*:}" >/dev/null; done
	if [ -n "$GUILD_B" ]; then
		"${PSQL[@]}" -c "DELETE FROM memberships WHERE guild_id = $GUILD_B; DELETE FROM guilds WHERE id = $GUILD_B" >/dev/null
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
body() { json "$1" <"$WORK/body"; }
expect() { # expect DESCRIPTION WANT GOT
	[ "$2" = "$3" ] || fail "$1: got $3, want $2 ($(head -c 300 "$WORK/body" 2>/dev/null))"
	echo "ok: $1"
}

sign_in
GUILD_A=$(bakery GET /api/me | json "d['guild']['id']")
OWNER_ID=$(bakery GET /api/me | json "d['member']['id']")

say "A second Guild, with the Owner as its admin (made with SQL until the dashboard can)"
GUILD_B=$("${PSQL[@]}" -c "INSERT INTO guilds (name, description, created_at, updated_at) VALUES ('$RUN', '', now(), now()) RETURNING id" | head -1)
"${PSQL[@]}" -c "INSERT INTO memberships (guild_id, user_id, role, created_at, updated_at) VALUES ($GUILD_B, $OWNER_ID, 'admin', now(), now())" >/dev/null
expect "the Owner acts in the second Guild" "$GUILD_B" "$(in_b GET /api/me | json "d['guild']['id']")"

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

say "Guilds keep to themselves"
