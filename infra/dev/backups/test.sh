#!/usr/bin/env bash
# End to end: Backups of Databases. For PostgreSQL, MySQL, MariaDB and
# MongoDB a row is backed up to local disk and the Garage stand-in, deleted
# and restored; a Backup whose local file is gone restores from Garage;
# Retention 2 keeps two files and two objects; a schedule of every minute
# backs up by itself; Redis refuses a Backup; deleting a Database removes
# its local Backups. Needs `task dev` and `task s3:up`.
set -euo pipefail

KEEP_FORGEJO=1 # this test does not use Forgejo
# shellcheck source=SCRIPTDIR/../lib/e2e.sh
. "$(dirname "$0")/../lib/e2e.sh"

[ -f "$STATE/garage.env" ] || fail "no $STATE/garage.env: run task s3:up"
# shellcheck disable=SC1091
. "$STATE/garage.env"
BACKUPS_DIR="$ROOT/services/api/storage/backups"

DBS=() STORAGE_ID=""
e2e_cleanup_hook() {
	local id
	for id in "${DBS[@]}"; do bakery DELETE "/api/databases/$id" >/dev/null; done
	if [ -n "$STORAGE_ID" ]; then bakery DELETE "/api/s3-storages/$STORAGE_ID" >/dev/null; fi
	for key in $(objects "$RUN/"); do s3 -X DELETE "$GARAGE_ENDPOINT/$GARAGE_BUCKET/$key" >/dev/null; done
}

# s3 ARGS...: curl signed for the Garage stand-in.
s3() { curl -sS --aws-sigv4 "aws:amz:$GARAGE_REGION:s3" --user "$GARAGE_ACCESS_KEY:$GARAGE_SECRET_KEY" "$@"; }
# objects PREFIX: the object keys below PREFIX, one per line.
objects() {
	s3 "$GARAGE_ENDPOINT/$GARAGE_BUCKET?list-type=2&prefix=$1" |
		python3 -c 'import re,sys; print("\n".join(re.findall(r"<Key>([^<]*)</Key>", sys.stdin.read())))'
}

sign_in
PROJECT_ID=$(bakery POST /api/projects "{\"name\":\"$RUN\"}" | json "d['project']['id']")
ENV_ID=$(bakery GET "/api/projects/$PROJECT_ID" | json "d['project']['environments'][0]['id']")

database() { bakery GET "/api/databases/$1" | json "d['database']$2"; }
create() { # create ENGINE: prints the new Database's id
	local out
	out=$(bakery POST "/api/environments/$ENV_ID/databases" "{\"name\":\"$RUN-$1\",\"engine\":\"$1\"}")
	echo "$out" | json "d['database']['id']" || fail "creating $1: $out"
}
running() { # running ID
	local until=$((SECONDS + 240)) s
	while :; do
		s=$(database "$1" "['status']")
		[ "$s" = running ] && return
		[ $SECONDS -lt $until ] || fail "database $1 is $s, not running: $(database "$1" ".get('error')")"
		sleep 2
	done
}
container() { echo "bakery-db-$(database "$1" "['slug']")"; }

# sql ENGINE ID STATEMENT: runs STATEMENT with the Engine's own client in
# the Database's Container, printing the result without headers.
# shellcheck disable=SC2016 # the variables expand in the container
sql() {
	local c
	c=$(container "$2")
	case $1 in
	postgresql) podman exec "$c" sh -c 'PGPASSWORD="$POSTGRES_PASSWORD" psql -h 127.0.0.1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tAc "$0"' "$3" ;;
	mysql) podman exec "$c" sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -h 127.0.0.1 -uroot -N "$MYSQL_DATABASE" -e "$0"' "$3" ;;
	mariadb) podman exec "$c" sh -c 'MYSQL_PWD="$MARIADB_ROOT_PASSWORD" mariadb -h 127.0.0.1 -uroot -N "$MARIADB_DATABASE" -e "$0"' "$3" ;;
	mongodb) podman exec "$c" sh -c 'mongosh --quiet --host 127.0.0.1 -u "$MONGO_INITDB_ROOT_USERNAME" -p "$MONGO_INITDB_ROOT_PASSWORD" --authenticationDatabase admin "$MONGO_INITDB_DATABASE" --eval "$0"' "$3" ;;
	esac
}
write_row() {
	case $1 in
	mongodb) sql "$1" "$2" 'db.notes.insertOne({body: "kept"})' >/dev/null ;;
	*) sql "$1" "$2" "CREATE TABLE notes (body varchar(20)); INSERT INTO notes VALUES ('kept')" >/dev/null ;;
	esac
}
drop_row() {
	case $1 in
	mongodb) sql "$1" "$2" 'db.notes.drop()' >/dev/null ;;
	*) sql "$1" "$2" "DROP TABLE notes" >/dev/null ;;
	esac
}
read_row() {
	case $1 in
	mongodb) sql "$1" "$2" 'db.notes.findOne().body' ;;
	*) sql "$1" "$2" "SELECT body FROM notes" ;;
	esac
}

backup_field() { bakery GET "/api/databases/$1/backups" | json "[b for b in d['backups'] if b['id']==$2][0]$3"; }
back_up() { # back_up ID: prints the new Backup's id once it succeeded
	local out b until=$((SECONDS + 120)) s
	out=$(bakery POST "/api/databases/$1/backups")
	b=$(echo "$out" | json "d['backup']['id']") || fail "backing up $1: $out"
	while :; do
		s=$(backup_field "$1" "$b" "['status']")
		[ "$s" = succeeded ] && break
		[ "$s" = failed ] && fail "backup $b failed: $(backup_field "$1" "$b" ".get('error')")"
		[ $SECONDS -lt $until ] || fail "backup $b still $s"
		sleep 1
	done
	echo "$b"
}
restore() { # restore ID BACKUP
	local out until=$((SECONDS + 120))
	out=$(bakery POST "/api/backups/$2/restore")
	[[ $out == *restoring* ]] || fail "restoring $2: $out"
	while [ "$(database "$1" "['restoring']")" = True ]; do
		[ $SECONDS -lt $until ] || fail "restore of $2 did not finish"
		sleep 1
	done
	local err
	err=$(database "$1" "['last_restore'].get('error') or ''")
	[ -z "$err" ] || fail "restore of $2 failed: $err"
}
schedule() { # schedule ID CRON RETENTION
	local out
	out=$(bakery PUT "/api/databases/$1/backup-schedule" "{\"enabled\":true,\"cron\":\"$2\",\"retention\":$3,\"s3_storage_id\":$STORAGE_ID}")
	[[ $out == *'"database"'* ]] || fail "setting the schedule of $1: $out"
}

say "S3 storage (Garage)"
out=$(bakery POST /api/s3-storages "{\"name\":\"$RUN\",\"endpoint\":\"$GARAGE_ENDPOINT\",\"region\":\"$GARAGE_REGION\",\"bucket\":\"$GARAGE_BUCKET\",\"prefix\":\"$RUN\",\"access_key\":\"$GARAGE_ACCESS_KEY\",\"secret_key\":\"$GARAGE_SECRET_KEY\"}")
STORAGE_ID=$(echo "$out" | json "d['s3_storage']['id']") || fail "creating the S3 storage: $out"
[[ $out != *"$GARAGE_SECRET_KEY"* ]] || fail "the secret key came back"
[ "$(bakery POST /api/s3-storages/check "{\"id\":$STORAGE_ID,\"name\":\"x\",\"endpoint\":\"$GARAGE_ENDPOINT\",\"region\":\"$GARAGE_REGION\",\"bucket\":\"$GARAGE_BUCKET\",\"access_key\":\"$GARAGE_ACCESS_KEY\"}" | json "d['ok']")" = True ] || fail "connection test"
echo "ok: storage $STORAGE_ID reaches the bucket"

declare -A ID
for engine in postgresql mysql mariadb mongodb redis; do
	ID[$engine]=$(create "$engine")
	DBS+=("${ID[$engine]}")
done

for engine in postgresql mysql mariadb mongodb; do
	say "$engine: back up, drop, restore"
	id=${ID[$engine]}
	running "$id"
	schedule "$id" "0 3 * * *" 2
	write_row "$engine" "$id"
	b=$(back_up "$id")
	[ "$(backup_field "$id" "$b" "['local']")/$(backup_field "$id" "$b" "['s3']")" = True/True ] || fail "$engine backup not local and in S3"
	file=$(backup_field "$id" "$b" "['file_name']")
	[ -s "$BACKUPS_DIR/$id/$file" ] || fail "$engine: no file $BACKUPS_DIR/$id/$file"
	grep -q "/$file$" <<<"$(objects "$RUN/")" || fail "$engine: $file not in the bucket"
	drop_row "$engine" "$id"
	restore "$id" "$b"
	[ "$(read_row "$engine" "$id" | tr -d '\r')" = kept ] || fail "$engine: row not back after restore"
	echo "ok: $engine row back from $file"
done

PG=${ID[postgresql]}
say "Restore from S3 when the local file is gone"
b=$(back_up "$PG")
rm "$BACKUPS_DIR/$PG/$(backup_field "$PG" "$b" "['file_name']")"
drop_row postgresql "$PG"
restore "$PG" "$b"
[ "$(read_row postgresql "$PG")" = kept ] || fail "row not back from S3"
echo "ok: restored from Garage"

say "Retention 2"
back_up "$PG" >/dev/null
back_up "$PG" >/dev/null
n=$(bakery GET "/api/databases/$PG/backups" | json "len(d['backups'])")
[ "$n" = 2 ] || fail "$n backups listed, want 2"
files=$(find "$BACKUPS_DIR/$PG" -type f | wc -l)
[ "$files" = 2 ] || fail "$files files in $BACKUPS_DIR/$PG, want 2"
slug=$(database "$PG" "['slug']")
objs=$(objects "$RUN/$slug-$PG/" | grep -c .)
[ "$objs" = 2 ] || fail "$objs objects in the bucket, want 2"
echo "ok: two backups, two files, two objects"

say "Scheduled every minute"
MY=${ID[mysql]}
schedule "$MY" "* * * * *" 5
scheduled() { [ "$(bakery GET "/api/databases/$MY/backups" | json "sum(1 for b in d['backups'] if b['trigger']=='scheduled' and b['status']=='succeeded')")" -ge 1 ]; }
wait_for 150 "a scheduled backup" scheduled
echo "ok: a scheduled backup succeeded"

say "Redis has no backups"
out=$(curl -s -o /dev/null -w '%{http_code}' -b "$JAR" -X POST "$API/api/databases/${ID[redis]}/backups")
[ "$out" = 422 ] || fail "backing up redis answered $out"
echo "ok: 422"

say "Deleting a Database removes its local backups"
[ "$(curl -s -o /dev/null -w '%{http_code}' -b "$JAR" -X DELETE "$API/api/s3-storages/$STORAGE_ID")" = 409 ] || fail "deleting a storage in use"
for id in "${DBS[@]}"; do
	[ "$(curl -s -o /dev/null -w '%{http_code}' -b "$JAR" -X DELETE "$API/api/databases/$id")" = 204 ] || fail "deleting $id"
	[ ! -e "$BACKUPS_DIR/$id" ] || fail "$BACKUPS_DIR/$id left"
done
DBS=()
[ "$(objects "$RUN/$slug-$PG/" | grep -c .)" = 2 ] || fail "S3 copies did not stay"
echo "ok: local backups gone, S3 copies stay"
echo
echo "PASS"
