#!/usr/bin/env bash
# End to end: Notifications. One channel of every kind (email to Mailpit,
# the rest to a local receiver standing in for Discord, Slack, Telegram,
# ntfy and a webhook) passes its Test; a failed Deployment, a succeeded one
# for the channel that asked, a failed Backup and, with the Remote server
# stand-in running, a Server going down and coming back each reach the
# channels subscribed to them; a channel that cannot be reached fails after
# three attempts; an Invitation is emailed with a link that works.
#
# Needs `task dev` and `task mail:up`. The API is restarted with the
# Telegram API pointed at the receiver and a 10 s Server probe, and
# restarted as it was at the end. `task remote:up` adds the Server probe.
set -euo pipefail

KEEP_FORGEJO=1 # this test does not use Forgejo
# shellcheck source=SCRIPTDIR/../lib/e2e.sh
. "$(dirname "$0")/../lib/e2e.sh"

MAILPIT=http://127.0.0.1:4985
RECEIVER=http://127.0.0.1:4988
LOG="$WORK/received.jsonl"
STAND_IN=bakery-dev-remote-1
CHANNELS=() DB_ID="" STORAGE_ID="" SERVER_ID="" TOKEN_ID="" RECEIVER_PID="" RESTARTED="" STOPPED=""

# restart_api [VAR=VALUE...]: restarts `task dev` (detached, so it outlives
# this script) with the given environment, and signs in again.
restart_api() {
	(cd "$ROOT" && task down >/dev/null 2>&1 || true)
	(cd "$ROOT" && env "$@" setsid -f task dev </dev/null >/tmp/bakery-task-dev.log 2>&1)
	wait_for 180 "the API" curl -sf "$API/api/health" -o /dev/null
	sign_in >/dev/null
}

e2e_cleanup_hook() {
	local id
	for id in "${CHANNELS[@]}"; do bakery DELETE "/api/notification-channels/$id" >/dev/null; done
	[ -z "$DB_ID" ] || bakery DELETE "/api/databases/$DB_ID" >/dev/null
	[ -z "$STORAGE_ID" ] || bakery DELETE "/api/s3-storages/$STORAGE_ID" >/dev/null
	[ -z "$SERVER_ID" ] || bakery DELETE "/api/servers/$SERVER_ID" >/dev/null
	[ -z "$TOKEN_ID" ] || bakery DELETE "/api/api-tokens/$TOKEN_ID" >/dev/null
	for id in $(bakery GET /api/invitations 2>/dev/null | json "' '.join(str(i['id']) for i in d['invitations'] if '$RUN' in i['email'])"); do
		bakery DELETE "/api/invitations/$id" >/dev/null
	done
	curl -s "$MAILPIT/api/v1/messages?limit=500" | json "' '.join(m['ID'] for m in d['messages'] if any('$RUN' in t['Address'] for t in m['To']))" |
		python3 -c "import json,sys; ids=sys.stdin.read().split(); print(json.dumps({'IDs': ids}))" |
		curl -s -X DELETE -H 'Content-Type: application/json' -d @- "$MAILPIT/api/v1/messages" >/dev/null
	[ -z "$STOPPED" ] || podman start "$STAND_IN" >/dev/null 2>&1
	[ -z "$RECEIVER_PID" ] || kill "$RECEIVER_PID" 2>/dev/null
	if [ -n "$RESTARTED" ]; then
		say "Restarting the API as it was"
		restart_api
	fi
}

# received N FILTER: waits until N requests matching the Python FILTER
# (over r, one logged request) reached the receiver.
count() { python3 -c "import json,sys; print(sum(1 for l in open('$LOG') for r in [json.loads(l)] if $1))"; }
at_least() { [ "$(count "$2")" -ge "$1" ]; }
received() { wait_for 60 "$1 request(s) where $2" at_least "$1" "$2"; }
# one DESCRIPTION FILTER: exactly one request matched FILTER.
one() { [ "$(count "$2")" = 1 ] || fail "$1: $(count "$2") requests, want 1"; }
# mails N TO SUBJECT_PART: waits until Mailpit has N messages to TO whose
# subject contains SUBJECT_PART.
mail_count() {
	curl -s "$MAILPIT/api/v1/messages?limit=500" |
		json "sum(1 for m in d['messages'] if any(t['Address']=='$1' for t in m['To']) and '$2' in m['Subject'])"
}
mailed() { [ "$(mail_count "$2" "$3")" -ge "$1" ]; }
mails() { wait_for 60 "$1 email(s) to $2 about $3" mailed "$1" "$2" "$3"; }
channel() { # channel JSON: adds a channel, its id in $CHANNEL and remembered for cleanup
	local out
	out=$(bakery POST /api/notification-channels "$1")
	CHANNEL=$(json "d['channel']['id']" <<<"$out") || fail "adding a channel: $out"
	CHANNELS+=("$CHANNEL")
}

sign_in
curl -sf "$MAILPIT/api/v1/messages" -o /dev/null || fail "Mailpit is not running on $MAILPIT (start task mail:up)"

say "Receiver on $RECEIVER, API with the Telegram API there and a 10 s Server probe"
: >"$LOG"
bun "$ROOT/infra/dev/notifications/receiver.ts" "$LOG" &
RECEIVER_PID=$!
wait_for 20 "the receiver" curl -s -o /dev/null "$RECEIVER/ready"
RESTARTED=1
restart_api BAKERY_TELEGRAM_API_URL="$RECEIVER/telegram" BAKERY_SERVER_PROBE_INTERVAL=10s

say "One channel of every kind"
OPS="ops-$RUN@example.com"
channel "{\"name\":\"$RUN-email\",\"kind\":\"email\",\"settings\":{\"host\":\"127.0.0.1\",\"port\":4980,\"security\":\"none\",\"username\":\"bakery\",\"password\":\"pw-$RUN\",\"from\":\"bakery@example.com\",\"to\":[\"$OPS\"]}}"
channel "{\"name\":\"$RUN-discord\",\"kind\":\"discord\",\"settings\":{\"url\":\"$RECEIVER/discord/tok-$RUN\"}}"
channel "{\"name\":\"$RUN-slack\",\"kind\":\"slack\",\"settings\":{\"url\":\"$RECEIVER/slack/tok-$RUN\"}}"
channel "{\"name\":\"$RUN-telegram\",\"kind\":\"telegram\",\"settings\":{\"bot_token\":\"123:bot-$RUN\",\"chat_id\":\"-100\"}}"
channel "{\"name\":\"$RUN-ntfy\",\"kind\":\"ntfy\",\"settings\":{\"url\":\"$RECEIVER/ntfy\",\"topic\":\"bakery\",\"token\":\"tk-$RUN\"}}"
channel "{\"name\":\"$RUN-hook\",\"kind\":\"webhook\",\"settings\":{\"url\":\"$RECEIVER/hook/tok-$RUN\",\"secret\":\"secret-$RUN\"},\"event_kinds\":[\"deployment_failed\",\"deployment_succeeded\",\"backup_failed\",\"server_unreachable\",\"server_reachable\"]}"
HOOK=$CHANNEL
for secret in "pw-$RUN" "tok-$RUN" "bot-$RUN" "tk-$RUN" "secret-$RUN"; do
	! grep -q -- "$secret" <<<"$(bakery GET /api/notification-channels)" || fail "a secret ($secret) is in the channel list"
done
echo "ok: six channels, no secret shown"

OUT=$(bakery POST /api/api-tokens "{\"name\":\"$RUN\",\"read_only\":true}")
TOKEN_ID=$(json "d['api_token']['id']" <<<"$OUT")
READ_ONLY=$(json "d['token']" <<<"$OUT")
[ "$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $READ_ONLY" "$API/api/notification-channels")" = 403 ] ||
	fail "a viewer reads the channels"
echo "ok: a viewer is refused"

say "Test each channel"
for id in "${CHANNELS[@]}"; do
	[ "$(bakery POST "/api/notification-channels/$id/test" | json "d['ok']")" = True ] || fail "the test of channel $id failed"
done
mails 1 "$OPS" "Test notification from Bakery"
received 1 "r['path']=='/discord/tok-$RUN' and json.loads(r['body'])['content'].startswith('**Test notification from Bakery**')"
received 1 "r['path']=='/slack/tok-$RUN' and json.loads(r['body'])['text'].startswith('*Test notification from Bakery*')"
received 1 "r['path']=='/telegram/bot123:bot-$RUN/sendMessage' and json.loads(r['body'])['chat_id']=='-100'"
received 1 "r['path']=='/ntfy/bakery' and r['headers'].get('title')=='Test notification from Bakery' and r['headers'].get('authorization')=='Bearer tk-$RUN'"
received 1 "r['path']=='/hook/tok-$RUN'"
python3 - "$LOG" "secret-$RUN" "/hook/tok-$RUN" <<'PY' || fail "the webhook signature does not verify"
import hashlib, hmac, json, sys
log, secret, path = sys.argv[1:]
r = [json.loads(l) for l in open(log) if json.loads(l)['path'] == path][-1]
want = 'sha256=' + hmac.new(secret.encode(), r['body'].encode(), hashlib.sha256).hexdigest()
sys.exit(0 if r['headers'].get('x-bakery-signature') == want else 1)
PY
echo "ok: every kind delivered its test, the webhook is signed"

say "A failed Deployment reaches every channel; a succeeded one only the channel that asked"
PROJECT_ID=$(bakery POST /api/projects "{\"name\":\"$RUN\"}" | json "d['project']['id']")
ENV_ID=$(bakery GET "/api/projects/$PROJECT_ID" | json "d['project']['environments'][0]['id']")
APP_ID=$(bakery POST "/api/environments/$ENV_ID/applications" \
	"{\"name\":\"$RUN\",\"build_pack\":\"image\",\"image_reference\":\"ghcr.io/jevido/bakery-e2e-does-not-exist:v0\",\"port\":80}" | json "d['application']['id']")
APP_SLUG=$(bakery GET "/api/applications/$APP_ID" | json "d['application']['slug']")
APPS+=("$APP_ID $APP_SLUG")
bakery POST "/api/applications/$APP_ID/deploy" >/dev/null
FAILED="Deployment of $APP_SLUG failed"
mails 1 "$OPS" "$FAILED"
for path in "/discord/tok-$RUN" "/slack/tok-$RUN" "/ntfy/bakery" "/hook/tok-$RUN" "/telegram/bot123:bot-$RUN/sendMessage"; do
	received 2 "r['path']=='$path'"
done
one "the webhook's failed deployment" "r['path']=='/hook/tok-$RUN' and json.loads(r['body'])['event']=='deployment_failed' and '$FAILED' == json.loads(r['body'])['title'] and json.loads(r['body'])['link'].endswith('/#/applications/$APP_ID')"
one "ntfy marks a failure" "r['path']=='/ntfy/bakery' and r['headers'].get('title')=='$FAILED' and r['headers'].get('tags')=='warning'"

bakery PATCH "/api/applications/$APP_ID" "{\"name\":\"$RUN\",\"image_reference\":\"ghcr.io/traefik/whoami:v1.10\",\"port\":80}" >/dev/null
bakery POST "/api/applications/$APP_ID/deploy" >/dev/null
wait_for 180 "the deployment" deployment_done
received 1 "r['path']=='/hook/tok-$RUN' and json.loads(r['body'])['event']=='deployment_succeeded'"
sleep 3
[ "$(count "'Deployment of $APP_SLUG succeeded' in r['body']")" = 1 ] || fail "a channel not subscribed heard of a succeeded deployment"
echo "ok: failures to all, success only to the webhook"

say "A channel that cannot be reached fails after three attempts, the others are sent"
channel "{\"name\":\"$RUN-closed\",\"kind\":\"webhook\",\"settings\":{\"url\":\"http://127.0.0.1:4989/closed\"},\"event_kinds\":[\"deployment_failed\"]}"
CLOSED=$CHANNEL
bakery PATCH "/api/applications/$APP_ID" "{\"name\":\"$RUN\",\"image_reference\":\"ghcr.io/jevido/bakery-e2e-does-not-exist:v0\",\"port\":80}" >/dev/null
bakery POST "/api/applications/$APP_ID/deploy" >/dev/null
closed_failed() { [ "$(bakery GET "/api/notification-channels/$CLOSED/deliveries" | json "d['deliveries'][0]['status'] if d['deliveries'] else ''")" = failed ]; }
wait_for 150 "the delivery to the closed port to fail" closed_failed
[ "$(bakery GET "/api/notification-channels/$CLOSED/deliveries" | json "d['deliveries'][0]['attempts']")" = 3 ] || fail "not three attempts"
grep -q "connection refused" <<<"$(bakery GET "/api/notification-channels/$CLOSED/deliveries" | json "d['deliveries'][0]['last_error']")" ||
	fail "no reason on the failed delivery"
[ "$(bakery GET "/api/notification-channels/$HOOK/deliveries" | json "all(x['status']=='sent' for x in d['deliveries'])")" = True ] ||
	fail "the webhook has unsent deliveries"
echo "ok: failed after 3 attempts with the reason; the rest sent"

say "A failed Backup"
DB_ID=$(bakery POST "/api/environments/$ENV_ID/databases" "{\"name\":\"$RUN\",\"engine\":\"postgresql\"}" | json "d['database']['id']")
DB_NAME=$(bakery GET "/api/databases/$DB_ID" | json "d['database']['name']")
db_running() { [ "$(bakery GET "/api/databases/$DB_ID" | json "d['database']['status']")" = running ]; }
wait_for 120 "the database" db_running
# An S3 storage nobody answers on: the dump succeeds, the upload fails.
STORAGE_ID=$(bakery POST /api/s3-storages "{\"name\":\"$RUN\",\"endpoint\":\"http://127.0.0.1:4989\",\"bucket\":\"nope\",\"access_key\":\"a\",\"secret_key\":\"b\"}" | json "d['s3_storage']['id']")
bakery PUT "/api/databases/$DB_ID/backup-schedule" "{\"enabled\":false,\"cron\":\"0 3 * * *\",\"retention\":2,\"s3_storage_id\":$STORAGE_ID}" >/dev/null
bakery POST "/api/databases/$DB_ID/backups" >/dev/null
received 1 "r['path']=='/hook/tok-$RUN' and json.loads(r['body'])['event']=='backup_failed' and json.loads(r['body'])['title']=='Backup of $DB_NAME failed'"
mails 1 "$OPS" "Backup of $DB_NAME failed"
echo "ok: the failed backup was told"

if podman container exists "$STAND_IN" && [ "$(podman inspect -f '{{.State.Running}}' "$STAND_IN")" = true ]; then
	say "Server probe: the Remote server stand-in goes down and comes back"
	SERVER_ID=$(bakery POST /api/servers "{\"name\":\"$RUN\",\"host\":\"127.0.0.1\",\"port\":4972,\"user\":\"podman\"}" | json "d['server']['id']")
	KEY=$(bakery GET "/api/servers/$SERVER_ID" | json "d['server']['public_key']")
	podman exec "$STAND_IN" sh -c "echo '$KEY' >> /home/podman/.ssh/authorized_keys"
	[ "$(bakery POST "/api/servers/$SERVER_ID/validate" | json "d['server']['status']")" = reachable ] || fail "the stand-in is not reachable"
	STOPPED=1
	podman stop -t 2 "$STAND_IN" >/dev/null
	received 1 "r['path']=='/hook/tok-$RUN' and json.loads(r['body'])['title']=='Server $RUN is unreachable'"
	podman start "$STAND_IN" >/dev/null 2>&1
	STOPPED=""
	received 1 "r['path']=='/hook/tok-$RUN' and json.loads(r['body'])['title']=='Server $RUN is reachable again'"
	sleep 12
	one "unreachable told once" "r['path']=='/hook/tok-$RUN' and json.loads(r['body'])['title']=='Server $RUN is unreachable'"
	bakery DELETE "/api/servers/$SERVER_ID" >/dev/null
	SERVER_ID=""
	echo "ok: unreachable and reachable again, once each"
else
	say "Server probe skipped: the Remote server stand-in is not running (task remote:up)"
fi

say "An Invitation is emailed with a link that works"
INVITEE="dev-$RUN@example.com"
OUT=$(bakery POST /api/invitations "{\"email\":\"$INVITEE\",\"role\":\"member\"}")
[ "$(json "d['emailed']" <<<"$OUT")" = True ] || fail "not emailed: $OUT"
mails 1 "$INVITEE" "invited you to Bakery"
MESSAGE=$(curl -s "$MAILPIT/api/v1/messages?limit=500" | json "[m['ID'] for m in d['messages'] if any(t['Address']=='$INVITEE' for t in m['To'])][0]")
TOKEN=$(curl -s "$MAILPIT/api/v1/message/$MESSAGE" | json "__import__('re').search(r'#/invite/([A-Za-z0-9]+)', d['Text']).group(1)")
[ "$(curl -s -o /dev/null -w '%{http_code}' "$API/api/invitations/by-token/$TOKEN")" = 200 ] || fail "the emailed link does not open"
[ "$(mail_count "$OPS" "invited you")" = 0 ] ||
	fail "the invitation went to the channel's recipients too"
echo "ok: emailed to the invited person only, the link opens"

say "PASS"
