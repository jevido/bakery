#!/usr/bin/env bash
# End to end: a Member's Profile and Two-factor authentication. Invited
# Members (never the dev Owner, whom every other test signs in as) rename
# themselves, switch two-factor on, sign in in two steps with Authenticator
# codes and Recovery codes, change their password, and are reset by an
# admin and by the artisan command. Needs `task dev` running (API on
# 127.0.0.1:4910).
set -euo pipefail

# Nothing here uses Forgejo; leave it as it is.
KEEP_FORGEJO=1
# shellcheck source=SCRIPTDIR/../lib/e2e.sh
. "$(dirname "$0")/../lib/e2e.sh"

MEMBER_IDS=()
e2e_cleanup_hook() {
	local id
	for id in "${MEMBER_IDS[@]}"; do bakery DELETE "/api/members/$id" >/dev/null; done
	for id in $(bakery GET /api/invitations 2>/dev/null | json "' '.join(str(i['id']) for i in d['invitations'] if '$RUN' in i['email'])"); do
		bakery DELETE "/api/invitations/$id" >/dev/null
	done
}

# as JAR METHOD PATH [JSON]: the status code of a request with a Member's
# cookies; the body is in $WORK/body.
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
# join EMAIL ROLE JAR: invites EMAIL and accepts it, signed in with JAR.
join() {
	local token
	token=$(bakery POST /api/invitations "{\"email\":\"$1\",\"role\":\"$2\"}" | json "d['path'].rsplit('/', 1)[1]")
	expect "$1 joins" 201 "$(as "$3" POST "/api/invitations/by-token/$token/accept" '{"name":"Someone","password":"correct horse battery"}')"
	MEMBER_IDS+=("$(body "d['member']['id']")")
}
# login JAR EMAIL PASSWORD: the status of the password step, in a fresh jar.
login() {
	rm -f "$1"
	as "$1" POST /api/login "{\"email\":\"$2\",\"password\":\"$3\"}"
}

# totp SECRET STEP: the Authenticator code of one time step.
totp() {
	python3 - "$1" "$2" <<'PY'
import base64, hashlib, hmac, struct, sys
s, step = sys.argv[1], int(sys.argv[2])
key = base64.b32decode(s + "=" * (-len(s) % 8))
h = hmac.new(key, struct.pack(">Q", step), hashlib.sha1).digest()
o = h[-1] & 15
print("%06d" % ((struct.unpack(">I", h[o:o + 4])[0] & 0x7FFFFFFF) % 1000000))
PY
}
# next_code SECRET sets CODE to an Authenticator code Bakery has not seen
# yet, for a step after LAST_STEP (Bakery accepts one step either side of
# now), waiting for the clock when every usable step is used. Not called in
# $(...), so that LAST_STEP survives.
LAST_STEP=0 CODE=""
next_code() {
	local now step
	while :; do
		now=$(($(date +%s) / 30))
		step=$((LAST_STEP + 1 > now - 1 ? LAST_STEP + 1 : now - 1))
		[ "$step" -gt $((now + 1)) ] || break
		sleep 2
	done
	LAST_STEP=$step
	CODE=$(totp "$1" "$step")
}

sign_in

M_EMAIL="member-$RUN@example.com" A_EMAIL="admin-$RUN@example.com" V_EMAIL="viewer-$RUN@example.com"
M="$WORK/member" M2="$WORK/member2" A="$WORK/admin" V="$WORK/viewer"
PASSWORD="correct horse battery"

say "Invited Members"
join "$M_EMAIL" member "$M"
MEMBER_ID=${MEMBER_IDS[0]}
join "$A_EMAIL" admin "$A"
join "$V_EMAIL" viewer "$V"
OWNER_ID=$(bakery GET /api/members | json "[m['id'] for m in d['members'] if m['role']=='owner'][0]")

say "Profile"
expect "a member renames themselves" 200 "$(as "$M" PATCH /api/me '{"name":"Mia Member"}')"
expect "the new name is theirs" "Mia Member" "$(body "d['member']['name']")"
expect "an empty name is refused" 422 "$(as "$M" PATCH /api/me '{"name":"  "}')"
expect "a viewer changes their own Profile too" 200 "$(as "$V" PATCH /api/me '{"name":"Vic Viewer"}')"
expect "the member makes an API token before two-factor" 201 "$(as "$M" POST /api/api-tokens '{"name":"ci token"}')"
TOKEN=$(body "d['token']")

say "Switching two-factor on"
expect "two-factor starts off" off "$(as "$M" GET /api/me/two-factor >/dev/null && body "d['state']")"
expect "setup starts" 200 "$(as "$M" POST /api/me/two-factor)"
SECRET=$(body "d['secret']")
grep -q "^otpauth://totp/The%20Bakery:" <<<"$(body "d['otpauth_uri']")" || fail "otpauth URI: $(body "d['otpauth_uri']")"
expect "it is pending" pending "$(as "$M" GET /api/me/two-factor >/dev/null && body "d['state']")"
expect "a wrong code is refused" 422 "$(as "$M" POST /api/me/two-factor/confirm '{"code":"000000"}')"
next_code "$SECRET"
expect "the right code switches it on" 200 "$(as "$M" POST /api/me/two-factor/confirm "{\"code\":\"$CODE\"}")"
expect "10 Recovery codes, once" 10 "$(body "len(d['recovery_codes'])")"
RECOVERY=$(body "d['recovery_codes'][0]")
expect "me says two-factor is on" True "$(as "$M" GET /api/me >/dev/null && body "d['member']['two_factor']")"
STORED=$(podman exec bakery-dev-postgres-1 psql -U bakery -d bakery -tAc "SELECT two_factor_secret_encrypted FROM users WHERE id = $MEMBER_ID")
[ -n "$STORED" ] || fail "no secret stored"
grep -q "$SECRET" <<<"$STORED" && fail "the secret is stored in plain text"
echo "ok: the secret is stored encrypted"

say "Signing in in two steps"
expect "the password step answers" 200 "$(login "$M2" "$M_EMAIL" "$PASSWORD")"
expect "it asks for a code" True "$(body "d.get('two_factor_required')")"
grep -q bakery_session "$M2" && fail "a Session was issued before the code"
expect "no Session before the code" 401 "$(as "$M2" GET /api/me)"
next_code "$SECRET"
USED=$CODE
expect "the right code signs in" 200 "$(as "$M2" POST /api/login/two-factor "{\"code\":\"$USED\"}")"
expect "the Session works" 200 "$(as "$M2" GET /api/me)"
login "$WORK/replay" "$M_EMAIL" "$PASSWORD" >/dev/null
expect "the same code again is refused" 422 "$(as "$WORK/replay" POST /api/login/two-factor "{\"code\":\"$USED\"}")"
grep -q "already used" <<<"$(body "d['message']")" || fail "replay message: $(body "d['message']")"
for i in 1 2 3; do
	expect "wrong code $i of 5" 422 "$(as "$WORK/replay" POST /api/login/two-factor '{"code":"000000"}')"
done
expect "four tries so far, one left" 1 "$(body "d['attempts_left']")"
expect "the fifth wrong code ends the challenge" 401 "$(as "$WORK/replay" POST /api/login/two-factor '{"code":"000000"}')"
expect "even a right code needs the password again" 401 "$(as "$WORK/replay" POST /api/login/two-factor "{\"code\":\"$(totp "$SECRET" $((LAST_STEP + 1)))\"}")"
expect "no challenge, no second step" 401 "$(as "$WORK/none" POST /api/login/two-factor '{"code":"123456"}')"
login "$WORK/rc" "$M_EMAIL" "$PASSWORD" >/dev/null
expect "a Recovery code signs in" 200 "$(as "$WORK/rc" POST /api/login/two-factor "{\"recovery_code\":\"$RECOVERY\"}")"
login "$WORK/rc2" "$M_EMAIL" "$PASSWORD" >/dev/null
expect "the same Recovery code never again" 422 "$(as "$WORK/rc2" POST /api/login/two-factor "{\"recovery_code\":\"$RECOVERY\"}")"
expect "9 Recovery codes left" 9 "$(as "$M" GET /api/me/two-factor >/dev/null && body "d['recovery_codes_left']")"
expect "the API token needs no code" 200 "$(with "$TOKEN" GET /api/projects)"

say "Session-only routes"
expect "an API token cannot read two-factor" 403 "$(with "$TOKEN" GET /api/me/two-factor)"
expect "an API token cannot switch it off" 403 "$(with "$TOKEN" DELETE /api/me/two-factor "{\"password\":\"$PASSWORD\",\"recovery_code\":\"x\"}")"
expect "an API token cannot change the password" 403 "$(with "$TOKEN" POST /api/me/password '{"current_password":"x","new_password":"y"}')"
expect "an API token cannot rename" 403 "$(with "$TOKEN" PATCH /api/me '{"name":"x"}')"

say "Changing the password"
sleep 1 # Sessions carry whole seconds; M2's must be from an earlier one.
expect "a wrong current password is refused" 422 "$(as "$M" POST /api/me/password '{"current_password":"not the password","new_password":"a brand new password"}')"
expect "a short new password is refused" 422 "$(as "$M" POST /api/me/password "{\"current_password\":\"$PASSWORD\",\"new_password\":\"short\"}")"
expect "the password changes" 200 "$(as "$M" POST /api/me/password "{\"current_password\":\"$PASSWORD\",\"new_password\":\"a brand new password\"}")"
expect "this browser stays signed in" 200 "$(as "$M" GET /api/me)"
expect "the other browser is signed out" 401 "$(as "$M2" GET /api/me)"
expect "the old password is refused" 401 "$(login "$M2" "$M_EMAIL" "$PASSWORD")"
expect "the new password asks for a code" True "$(login "$M2" "$M_EMAIL" "a brand new password" >/dev/null && body "d.get('two_factor_required')")"
next_code "$SECRET"
expect "and signs in with one" 200 "$(as "$M2" POST /api/login/two-factor "{\"code\":\"$CODE\"}")"
sleep 1
expect "sign out everywhere else" 200 "$(as "$M" POST /api/me/sign-out-others)"
expect "this browser stays signed in" 200 "$(as "$M" GET /api/me)"
expect "the other browser is signed out" 401 "$(as "$M2" GET /api/me)"

say "Resetting someone's two-factor"
expect "the admin sees it on" True "$(as "$A" GET /api/members >/dev/null && body "[m['two_factor'] for m in d['members'] if m['id']==$MEMBER_ID][0]")"
expect "a member cannot reset anyone" 403 "$(as "$M" DELETE "/api/members/${MEMBER_IDS[1]}/two-factor")"
expect "nobody resets the Owner's in the dashboard" 403 "$(as "$A" DELETE "/api/members/$OWNER_ID/two-factor")"
expect "nor their own" 403 "$(as "$A" DELETE "/api/members/${MEMBER_IDS[1]}/two-factor")"
sleep 1
expect "the admin resets the member's" 204 "$(as "$A" DELETE "/api/members/$MEMBER_ID/two-factor")"
expect "the member is signed out" 401 "$(as "$M" GET /api/me)"
expect "and signs in with the password alone" 200 "$(login "$M" "$M_EMAIL" "a brand new password")"
expect "straight into a Session" "Mia Member" "$(body "d['member']['name']")"
expect "resetting it again is a conflict" 409 "$(as "$A" DELETE "/api/members/$MEMBER_ID/two-factor")"

say "identity:reset-two-factor on the server"
expect "the admin sets up two-factor" 200 "$(as "$A" POST /api/me/two-factor)"
A_SECRET=$(body "d['secret']")
expect "and switches it on" 200 "$(as "$A" POST /api/me/two-factor/confirm "{\"code\":\"$(totp "$A_SECRET" $(($(date +%s) / 30)))\"}")"
grep -q "two-factor switched off" <<<"$(cd "$ROOT/services/api" && go run . artisan identity:reset-two-factor "$A_EMAIL")" || fail "the artisan command did not switch it off"
expect "off after the command" False "$(login "$A" "$A_EMAIL" "$PASSWORD" >/dev/null && body "d['member']['two_factor']")"
if (cd "$ROOT/services/api" && go run . artisan identity:reset-two-factor "nobody-$RUN@example.com") >/dev/null 2>&1; then
	fail "the command succeeded for an unknown email"
fi
echo "ok: the command refuses an unknown email"

say "PASS"
