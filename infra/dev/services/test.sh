#!/usr/bin/env bash
# End to end: Services. A Service from the whoami template answers through
# the Proxy; a two-Component compose Service reaches its Postgres by name
# and lists its volume by its Podman name, and keeps its data and generated
# password across Redeploy; Stop and Start take its Domain away and back;
# build: is refused with its line; Domains are one namespace with
# Applications; a Project with a Service cannot be deleted; deleting
# Services leaves no bakery-svc-* container, network or volume, except the
# volume of the one deleted with delete_volumes=false. Needs
# `task dev`.
set -euo pipefail

KEEP_FORGEJO=1 # this test does not use Forgejo
# shellcheck source=SCRIPTDIR/../lib/e2e.sh
. "$(dirname "$0")/../lib/e2e.sh"

SVCS=()
e2e_cleanup_hook() {
	local id
	for id in "${SVCS[@]}"; do bakery DELETE "/api/services/$id" >/dev/null; done
}

sign_in
PROJECT_ID=$(bakery POST /api/projects "{\"name\":\"$RUN\"}" | json "d['project']['id']")
ENV_ID=$(bakery GET "/api/projects/$PROJECT_ID" | json "d['project']['environments'][0]['id']")

service() { bakery GET "/api/services/$1" | json "d['service']$2"; }
code() { curl -s -o /dev/null -w '%{http_code}' -b "$JAR" "$@"; }
# create JSON: prints the new Service's id.
create() {
	local out
	out=$(bakery POST "/api/environments/$ENV_ID/services" "$1")
	echo "$out" | json "d['service']['id']" || fail "creating a service: $out"
}
running() { # running ID: waits, failing early on an error
	local until=$((SECONDS + 300)) s
	while :; do
		s=$(service "$1" "['status']")
		[ "$s" = running ] && return
		[ "$s" = failed ] && fail "service $1 failed: $(service "$1" ".get('last_error')")"
		[ $SECONDS -lt $until ] || fail "service $1 is $s, not running"
		sleep 2
	done
}
domain() { service "$1" "['components'][$2]['domains'][0]"; }
# answers DOMAIN: the HTTP code the Proxy answers on the Domain with.
answers() { curl -sk -o /dev/null -w '%{http_code}' --resolve "$1:4943:127.0.0.1" "https://$1:4943/" || true; }
compose_json() { COMPOSE=$2 NAME=$1 python3 -c 'import json,os; print(json.dumps({"name":os.environ["NAME"],"compose":os.environ["COMPOSE"]}))'; }

say "whoami from the catalog"
[[ $(bakery GET /api/service-templates) == *'"key":"whoami"'* ]] || fail "no whoami template"
WHOAMI=$(create "{\"template\":\"whoami\",\"name\":\"$RUN-whoami\"}")
SVCS+=("$WHOAMI")
running "$WHOAMI"
WHOAMI_DOMAIN=$(domain "$WHOAMI" 0)
wait_for 30 "whoami on $WHOAMI_DOMAIN" test "$(answers "$WHOAMI_DOMAIN")" = 200
grep -q '^Hostname:' <<<"$(curl -sk --resolve "$WHOAMI_DOMAIN:4943:127.0.0.1" "https://$WHOAMI_DOMAIN:4943/")" || fail "whoami did not answer as whoami"
echo "ok: $WHOAMI_DOMAIN answers"

say "Two Components from a compose file"
# shellcheck disable=SC2016 # ${...} is for Bakery to fill in
STACK_COMPOSE='services:
  web:
    image: docker.io/traefik/whoami:v1.10
    environment:
      - SERVICE_FQDN_WEB_80
    depends_on: [db]
  db:
    image: docker.io/library/postgres:18-alpine
    environment:
      POSTGRES_PASSWORD: ${SERVICE_PASSWORD_DB}
      POSTGRES_DB: ${DB_NAME:-app}
    volumes:
      - pgdata:/var/lib/postgresql
'
STACK=$(create "$(compose_json "$RUN-stack" "$STACK_COMPOSE")")
SVCS+=("$STACK")
running "$STACK"
[ "$(service "$STACK" "['components'][1]['public']")" = False ] || fail "db is public"
PASSWORD=$(bakery GET "/api/services/$STACK" | json "[v['value'] for v in d['service']['variables'] if v['name']=='SERVICE_PASSWORD_DB'][0]")
[ ${#PASSWORD} -eq 32 ] || fail "generated password is ${#PASSWORD} characters"
DB="bakery-svc-$STACK-db"
psql_db() { podman exec "$DB" psql -U postgres -d app -tAc "$1"; }
db_ready() { psql_db 'select 1' >/dev/null 2>&1; }
wait_for 60 "postgres in $DB" db_ready
psql_db "create table notes (body text); insert into notes values ('kept')" >/dev/null
out=$(podman run --rm --network "bakery-svc-$STACK" docker.io/library/busybox sh -c 'ping -c1 -W1 db 2>&1 | head -1')
[[ $out == "PING db ("* ]] || fail "db does not resolve on the Service network: $out"
STACK_DOMAIN=$(domain "$STACK" 0)
wait_for 30 "web on $STACK_DOMAIN" test "$(answers "$STACK_DOMAIN")" = 200
VOLUME=$(service "$STACK" "['components'][1]['volumes'][0]['name']")
[ "$VOLUME" = "bakery-svc-$STACK-pgdata" ] || fail "db's volume is listed as $VOLUME"
[ "$(service "$STACK" "['components'][1]['volumes'][0]['path']")" = /var/lib/postgresql ] || fail "db's volume path"
[ "$(service "$STACK" "['components'][0]['volumes']")" = "[]" ] || fail "web lists volumes"
podman volume exists "$VOLUME" || fail "$VOLUME is not in Podman"
echo "ok: web answers on $STACK_DOMAIN, db resolves by name, its volume is listed"

say "Redeploy keeps data and the generated password"
[ "$(code -X POST "$API/api/services/$STACK/redeploy")" = 202 ] || fail "redeploy"
[ "$(code -X POST "$API/api/services/$STACK/redeploy")" = 409 ] || fail "a second redeploy while busy was not refused"
running "$STACK"
wait_for 60 "postgres after redeploy" db_ready
[ "$(psql_db 'select body from notes')" = kept ] || fail "the row is gone after redeploy"
[ "$(bakery GET "/api/services/$STACK" | json "[v['value'] for v in d['service']['variables'] if v['name']=='SERVICE_PASSWORD_DB'][0]")" = "$PASSWORD" ] || fail "the password changed"
echo "ok: row and password survive"

say "Stop and Start"
[ "$(code -X POST "$API/api/services/$WHOAMI/stop")" = 202 ] || fail "stop"
[ "$(service "$WHOAMI" "['status']")" = stopped ] || fail "not stopped"
[ "$(answers "$WHOAMI_DOMAIN")" != 200 ] || fail "a stopped service still answers"
[ "$(code -X POST "$API/api/services/$WHOAMI/start")" = 202 ] || fail "start"
running "$WHOAMI"
wait_for 30 "whoami after start" test "$(answers "$WHOAMI_DOMAIN")" = 200
echo "ok: stopped, started, answering"

say "Refusals"
out=$(bakery POST "/api/environments/$ENV_ID/services" "$(compose_json "$RUN-bad" $'services:\n  app:\n    build: .\n')")
[[ $out == *'line 3: services.app.build'* ]] || fail "build: not refused with its line: $out"
[ "$(code -X POST -H 'Content-Type: application/json' -d "$(compose_json "$RUN-bad" $'services:\n  app:\n    build: .\n')" "$API/api/environments/$ENV_ID/services")" = 422 ] || fail "build: is not 422"
app=$(bakery POST "/api/environments/$ENV_ID/applications" "{\"name\":\"$RUN-clash\",\"git_url\":\"https://example.com/r.git\",\"port\":80,\"domains\":[\"$WHOAMI_DOMAIN\"]}")
[[ $app == *'already used by another application or service'* ]] || fail "an Application took a Service's domain: $app"
APP=$(bakery POST "/api/environments/$ENV_ID/applications" "{\"name\":\"$RUN-app\",\"git_url\":\"https://example.com/r.git\",\"port\":80}")
APP_ID=$(echo "$APP" | json "d['application']['id']")
APPS+=("$APP_ID $(echo "$APP" | json "d['application']['slug']")")
APP_DOMAIN=$(echo "$APP" | json "d['application']['domains'][0]")
[ "$(code -X PATCH -H 'Content-Type: application/json' -d "{\"domains\":{\"whoami\":[\"$APP_DOMAIN\"]}}" "$API/api/services/$WHOAMI")" = 422 ] || fail "a Service took an Application's domain"
[ "$(code -X DELETE "$API/api/projects/$PROJECT_ID")" = 409 ] || fail "a project with services was deleted"
echo "ok: 422 for build:, domains are shared, 409 for the project"

say "Deleting Services leaves nothing, or only the volumes when asked"
# Asked over the rootless socket's libpod API, as The Bakery itself does.
SOCK=${XDG_RUNTIME_DIR:-/run/user/$(id -u)}/podman/podman.sock
volume_exists() { [ "$(curl -s -o /dev/null -w '%{http_code}' --unix-socket "$SOCK" "http://d/v5.0.0/libpod/volumes/$1/exists")" = 204 ]; }
for id in "${SVCS[@]}"; do
	flags=
	[ "$id" = "$STACK" ] && flags='?delete_volumes=false'
	[ "$(code -X DELETE "$API/api/services/$id$flags")" = 204 ] || fail "deleting service $id"
	[ -z "$(podman ps -a --filter "label=bakery.service=$id" -q)" ] || fail "containers of service $id left"
	[ -z "$(podman network ls --filter "label=bakery.service=$id" -q)" ] || fail "network of service $id left"
	if [ -n "$flags" ]; then
		volume_exists "$VOLUME" || fail "$VOLUME not kept"
		curl -sf -o /dev/null --unix-socket "$SOCK" -X DELETE "http://d/v5.0.0/libpod/volumes/$VOLUME" || fail "removing kept $VOLUME"
	fi
	[ -z "$(podman volume ls --filter "label=bakery.service=$id" -q)" ] || fail "volumes of service $id left"
done
SVCS=()
[ "$(answers "$WHOAMI_DOMAIN")" != 200 ] || fail "a deleted service still answers"
echo "ok: no containers, networks or volumes left; a kept volume stays until removed"
echo
echo "PASS"
