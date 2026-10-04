#!/usr/bin/env bash
# End to end: one-click Databases. Every Database type becomes running and answers
# its own client on the Internal URL from a container on the bakery network;
# Postgres keeps its data across stop/start and a Public port change; the
# Public URL answers from the host until the Public port is removed; a
# Project with a Database cannot be deleted; deleting a Database removes its
# Container and volume, or only its Container with delete_volumes=false.
# With RESTART_API=1 it also removes a Database's Container by hand,
# restarts `task dev` (detached) and waits for the API to start the
# Database again. Needs `task dev` running.
set -euo pipefail

KEEP_FORGEJO=1 # this test does not use Forgejo
# shellcheck source=SCRIPTDIR/../lib/e2e.sh
. "$(dirname "$0")/../lib/e2e.sh"

DBS=()
e2e_cleanup_hook() {
	local id
	for id in "${DBS[@]}"; do bakery DELETE "/api/databases/$id" >/dev/null; done
}

sign_in
PROJECT_ID=$(bakery POST /api/projects "{\"name\":\"$RUN\"}" | json "d['project']['id']")
ENV_ID=$(bakery GET "/api/projects/$PROJECT_ID" | json "d['project']['environments'][0]['id']")

database() { bakery GET "/api/databases/$1" | json "d['database']$2"; }
status_is() { [ "$(database "$1" "['status']")" = "$2" ]; }
create() { # create TYPE: prints the new Database's id
	local out
	out=$(bakery POST "/api/environments/$ENV_ID/databases" "{\"name\":\"$RUN-$1\",\"type\":\"$1\"}")
	echo "$out" | json "d['database']['id']" || fail "creating $1: $out"
}
running() { # running ID: waits, failing early on an error
	local until=$((SECONDS + 240)) s
	while :; do
		s=$(database "$1" "['status']")
		[ "$s" = running ] && return
		[ "$s" = exited ] && fail "database $1 exited: $(database "$1" ".get('error')")"
		[ $SECONDS -lt $until ] || fail "database $1 is $s, not running: $(database "$1" ".get('error')")"
		sleep 2
	done
}
# client ID CMD...: runs CMD in a throwaway container of the Database's own
# image on the bakery network, with $URL set to its Internal URL.
client() {
	local id=$1 image url
	shift
	url=$(database "$id" "['internal_url']")
	image=$(podman inspect --format '{{.ImageName}}' "bakery-db-$(database "$id" "['slug']")")
	podman run --rm --network bakery -e URL="$url" "$image" sh -c "$*"
}
update() { # update ID PUBLIC_PORT
	bakery PATCH "/api/databases/$1" "{\"name\":\"$RUN-postgresql\",\"version\":\"18-alpine\",\"public_port\":$2,\"resource_limits\":{\"memory_mb\":null,\"cpus\":null}}" >/dev/null
}

say "Every Database type"
declare -A ID
for type in postgresql mysql mariadb redis valkey mongodb; do
	ID[$type]=$(create "$type")
	DBS+=("${ID[$type]}")
done
for type in postgresql mysql mariadb redis valkey mongodb; do
	running "${ID[$type]}"
	echo "ok: $type running"
done

say "Internal URLs from the bakery network"
q() { # q TYPE EXPECTED CMD
	local got
	got=$(client "${ID[$1]}" "$3" 2>&1 | tr -d '\r' | tail -1)
	[ "$got" = "$2" ] || fail "$1 on its internal URL answered '$got'"
	echo "ok: $1 answers on $(database "${ID[$1]}" "['internal_url']" | sed 's#//[^@]*@#//***@#')"
}
# shellcheck disable=SC2016 # $URL expands in the client container
q postgresql 1 'psql "$URL" -tAc "select 1"'
# mysql://user:password@host:port/database, split for clients without URLs.
# shellcheck disable=SC2016
MYSQL_ARGS='u=${URL#mysql://}; c=${u%%@*}; r=${u#*@}; hp=${r%%/*}; set -- -h "${hp%:*}" -P "${hp#*:}" -u "${c%%:*}" -p"${c#*:}" "${r#*/}"'
q mysql 1 "$MYSQL_ARGS; mysql \"\$@\" -N -e 'select 1' 2>/dev/null"
q mariadb 1 "$MYSQL_ARGS; mariadb \"\$@\" -N -e 'select 1'"
# shellcheck disable=SC2016
q redis PONG 'redis-cli -u "$URL" --no-auth-warning ping'
# shellcheck disable=SC2016
q valkey PONG 'valkey-cli -u "$URL" --no-auth-warning ping'
# shellcheck disable=SC2016
q mongodb 1 'mongosh --quiet "$URL" --eval "db.runCommand({ping: 1}).ok"'

PG=${ID[postgresql]}
container() { echo "bakery-db-$(database "$1" "['slug']")"; }
PG_CONTAINER=$(container "$PG")
psql_int() { client "$PG" "psql \"\$URL\" -tAc \"$1\""; }

say "Postgres keeps its data"
psql_int "CREATE TABLE kept (v text); INSERT INTO kept VALUES ('still here')" >/dev/null
bakery POST "/api/databases/$PG/stop" >/dev/null
status_is "$PG" stopped || fail "not stopped after stop"
podman container exists "$PG_CONTAINER" && fail "a stopped database still has a container"
bakery POST "/api/databases/$PG/start" >/dev/null
running "$PG"
[ "$(psql_int "SELECT v FROM kept")" = "still here" ] || fail "row lost across stop/start"
echo "ok: row survives stop and start"

say "Public port"
PORT=$((40000 + RANDOM % 20000))
update "$PG" "$PORT"
has_public_url() { [ -n "$(database "$PG" ".get('public_url') or ''")" ]; }
wait_for 60 "the public URL" has_public_url
running "$PG"
PUBLIC=$(database "$PG" "['public_url']")
[[ $PUBLIC == *"@localhost:$PORT/"* ]] || fail "public URL $PUBLIC"
IMAGE=$(podman inspect --format '{{.ImageName}}' "$PG_CONTAINER")
public() { podman run --rm --network host "$IMAGE" psql "$PUBLIC" -tAc "$1" 2>/dev/null; }
answers() { [ "$(public "select 1")" = 1 ]; }
wait_for 30 "the public port" answers
[ "$(public "SELECT v FROM kept")" = "still here" ] || fail "row lost across the public port change"
echo "ok: $PORT answers from the host, row still there"
update "$PG" null
running "$PG"
public "select 1" >/dev/null && fail "the public port still answers after removing it"
echo "ok: no longer published"

say "A Project with Databases cannot be deleted"
code=$(curl -s -o /dev/null -w '%{http_code}' -b "$JAR" -X DELETE "$API/api/projects/$PROJECT_ID")
[ "$code" = 409 ] || fail "deleting the project answered $code"
echo "ok: 409"

if [ -n "${RESTART_API:-}" ]; then
	say "Recovery after an API restart"
	REDIS=${ID[redis]}
	podman rm -f "$(container "$REDIS")" >/dev/null
	status_is "$REDIS" missing || fail "not missing after removing its container"
	(cd "$ROOT" && task down >/dev/null 2>&1 || true)
	# Detached, with no file descriptor of ours, so it outlives this script.
	(cd "$ROOT" && setsid -f task dev </dev/null >/tmp/bakery-task-dev.log 2>&1)
	wait_for 180 "the API" curl -sf "$API/api/health" -o /dev/null
	sign_in
	running "$REDIS"
	echo "ok: started again on boot"
fi

say "Deleting removes Container and volume, or keeps the volume when asked"
# Asked over the rootless socket's libpod API, as The Bakery itself does.
SOCK=${XDG_RUNTIME_DIR:-/run/user/$(id -u)}/podman/podman.sock
volume_exists() { [ "$(curl -s -o /dev/null -w '%{http_code}' --unix-socket "$SOCK" "http://d/v5.0.0/libpod/volumes/$1/exists")" = 204 ]; }
for type in "${!ID[@]}"; do
	id=${ID[$type]}
	name=$(container "$id")
	flags=
	[ "$type" = redis ] && flags='?delete_volumes=false'
	[ "$(curl -s -o /dev/null -w '%{http_code}' -b "$JAR" -X DELETE "$API/api/databases/$id$flags")" = 204 ] || fail "deleting $type"
	podman container exists "$name" && fail "$type container left"
	if [ -n "$flags" ]; then
		volume_exists "bakery-db-$id-data" || fail "$type volume not kept"
		curl -sf -o /dev/null --unix-socket "$SOCK" -X DELETE "http://d/v5.0.0/libpod/volumes/bakery-db-$id-data" || fail "removing kept $type volume"
	else
		volume_exists "bakery-db-$id-data" && fail "$type volume left"
	fi
done
DBS=()
echo "ok: nothing left"
echo
echo "PASS"
