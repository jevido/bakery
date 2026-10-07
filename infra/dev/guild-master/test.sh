#!/usr/bin/env bash
# End to end: the Guild Master. Ann, a viewer of "Default", makes a Guild
# and is its Guild Master; she invites Bea as a member and Cy as an admin.
# Nobody, not even an admin, re-roles or removes Ann. Ann offers the Guild
# Master to Bea: nothing changes until Bea accepts; Bea declines, Ann
# offers again and withdraws, offers a third time and Bea accepts. Then Bea
# is the Guild Master, both keep their Roles, Bea can remove Ann and nobody
# can remove Bea. An offer past its 7 days cannot be accepted. Needs
# `task dev` running (API on 127.0.0.1:4910) and the dev Postgres.
set -euo pipefail

# Nothing here uses Forgejo; leave it as it is.
KEEP_FORGEJO=1
# shellcheck source=SCRIPTDIR/../lib/e2e.sh
. "$(dirname "$0")/../lib/e2e.sh"

PSQL=(podman compose -f "$ROOT/infra/dev/compose.yml" exec -T postgres psql -U bakery -d bakery -tAq)
GUILD="" ANN_ID=""
e2e_cleanup_hook() {
	[ -z "$ANN_ID" ] || bakery DELETE "/api/members/$ANN_ID" >/dev/null
	if [ -n "$GUILD" ]; then
		"${PSQL[@]}" -c "DELETE FROM invitations WHERE guild_id = $GUILD; DELETE FROM memberships WHERE guild_id = $GUILD; DELETE FROM guilds WHERE id = $GUILD" >/dev/null
	fi
	for id in $(bakery GET /api/invitations 2>/dev/null | json "' '.join(str(i['id']) for i in d['invitations'] if '$RUN' in i['email'])"); do
		bakery DELETE "/api/invitations/$id" >/dev/null
	done
}

# as JAR METHOD PATH [JSON]: the status code of a request with a Member's
# Session, keeping the cookies it sets; the body is in $WORK/body.
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
# invite JAR EMAIL ROLE: the token of a new Invitation's link, made by JAR's
# Member in their Current guild.
invite() {
	as "$1" POST /api/invitations "{\"email\":\"$2\",\"role\":\"$3\"}" >/dev/null
	body "d['path'].rsplit('/', 1)[1]"
}
# join TOKEN JAR NAME: accepts an Invitation as a new Member, signing in
# with JAR; prints their id.
join() {
	as "$2" POST "/api/invitations/by-token/$1/accept" "{\"name\":\"$3\",\"password\":\"correct horse battery\"}" >/dev/null
	body "d['member']['id']"
}
# held MEMBER: the names of the Roles the Member holds in the Guild.
held() {
	"${PSQL[@]}" -c "SELECT string_agg(r.name, ',' ORDER BY r.name) FROM memberships m JOIN membership_roles mr ON mr.membership_id = m.id JOIN roles r ON r.id = mr.role_id WHERE m.guild_id = $GUILD AND m.user_id = $1"
}
# as_role_id JAR NAME: the id of the Role with this name in JAR's Current
# guild.
as_role_id() { as "$1" GET /api/roles >/dev/null; body "[r['id'] for r in d['roles'] if r['name'] == '$2'][0]"; }
master() { as "$1" GET /api/guilds/current >/dev/null; body "d['guild']['guild_master']['id']"; }
offer() { as "$1" GET /api/guilds/current >/dev/null; body "(d['guild']['offer'] or {}).get('id')"; }

sign_in
ANN=$WORK/ann BEA=$WORK/bea CY=$WORK/cy OWNER_ID=$(bakery GET /api/me | json "d['member']['id']")

say "Ann makes a Guild and is its Guild Master"
ANN_ID=$(join "$(invite "$JAR" "ann-$RUN@example.com" viewer)" "$ANN" Ann)
expect "Ann creates a Guild" 201 "$(as "$ANN" POST /api/guilds "{\"name\":\"$RUN\"}")"
GUILD=$(body "d['guild']['id']")
expect "Ann is its Guild Master" "$ANN_ID" "$(master "$ANN")"
expect "/api/me says so" True "$(as "$ANN" GET /api/me >/dev/null; body "d['guild_master']")"
expect "Ann holds Admin too" Admin "$(held "$ANN_ID")"
BEA_ID=$(join "$(invite "$ANN" "bea-$RUN@example.com" member)" "$BEA" Bea)
CY_ID=$(join "$(invite "$ANN" "cy-$RUN@example.com" admin)" "$CY" Cy)
expect "Bea is in Ann's Guild" "$GUILD" "$(as "$BEA" GET /api/me >/dev/null; body "d['guild']['id']")"
expect "the Members list Ann first, marked" "$ANN_ID True" "$(as "$CY" GET /api/members >/dev/null; body "d['members'][0]['id'], d['members'][0]['guild_master']")"
expect "nobody else is marked" 1 "$(body "sum(m['guild_master'] for m in d['members'])")"

say "Nobody re-roles or removes the Guild Master"
expect "an admin cannot re-role Ann" 403 "$(as "$CY" PUT "/api/members/$ANN_ID/roles/$(as_role_id "$CY" Viewer)")"
expect "an admin cannot remove Ann" 403 "$(as "$CY" DELETE "/api/members/$ANN_ID")"
expect "Cy, the other admin, can be demoted: Ann still holds everything" 200 "$(as "$ANN" DELETE "/api/members/$CY_ID/roles/$(as_role_id "$ANN" Admin)")"
as "$ANN" PUT "/api/members/$CY_ID/roles/$(as_role_id "$ANN" Admin)" >/dev/null

say "Offers"
expect "only the Guild Master offers" 403 "$(as "$CY" POST /api/guilds/current/guild-master-offer "{\"member_id\":$BEA_ID}")"
expect "not to someone outside the Guild" 422 "$(as "$ANN" POST /api/guilds/current/guild-master-offer "{\"member_id\":$OWNER_ID}")"
expect "not to herself" 422 "$(as "$ANN" POST /api/guilds/current/guild-master-offer "{\"member_id\":$ANN_ID}")"
expect "Ann offers it to Bea" 201 "$(as "$ANN" POST /api/guilds/current/guild-master-offer "{\"member_id\":$BEA_ID}")"
OFFER=$(body "d['offer']['id']")
expect "nothing changed" "$ANN_ID" "$(master "$BEA")"
expect "the Guild shows the offer" "$OFFER" "$(offer "$CY")"
expect "Bea sees it on /api/me with its Guild" "$OFFER $RUN" "$(as "$BEA" GET /api/me >/dev/null; body "d['offers'][0]['id'], d['offers'][0]['guild']['name']")"
expect "one open offer at a time" 409 "$(as "$ANN" POST /api/guilds/current/guild-master-offer "{\"member_id\":$CY_ID}")"
expect "Cy cannot accept Bea's offer" 404 "$(as "$CY" POST "/api/guild-master-offers/$OFFER/accept")"
expect "nor Ann" 404 "$(as "$ANN" POST "/api/guild-master-offers/$OFFER/accept")"
expect "Bea declines" 204 "$(as "$BEA" POST "/api/guild-master-offers/$OFFER/decline")"
expect "the offer is gone" None "$(offer "$ANN")"
expect "a declined offer cannot be accepted" 409 "$(as "$BEA" POST "/api/guild-master-offers/$OFFER/accept")"
expect "Ann offers again" 201 "$(as "$ANN" POST /api/guilds/current/guild-master-offer "{\"member_id\":$BEA_ID}")"
expect "Bea cannot withdraw it" 403 "$(as "$BEA" DELETE /api/guilds/current/guild-master-offer)"
expect "Ann withdraws it" 204 "$(as "$ANN" DELETE /api/guilds/current/guild-master-offer)"
expect "nothing left to withdraw" 404 "$(as "$ANN" DELETE /api/guilds/current/guild-master-offer)"
expect "Bea has no offers" 0 "$(as "$BEA" GET /api/me >/dev/null; body "len(d['offers'])")"

say "Bea accepts"
expect "Ann offers a third time" 201 "$(as "$ANN" POST /api/guilds/current/guild-master-offer "{\"member_id\":$BEA_ID}")"
OFFER=$(body "d['offer']['id']")
expect "Bea accepts" 204 "$(as "$BEA" POST "/api/guild-master-offers/$OFFER/accept")"
expect "Bea is the Guild Master" "$BEA_ID" "$(master "$ANN")"
expect "Ann is not" False "$(as "$ANN" GET /api/me >/dev/null; body "d['guild_master']")"
expect "Ann keeps Admin" Admin "$(held "$ANN_ID")"
expect "Bea keeps Member" Member "$(held "$BEA_ID")"
expect "Bea now holds every Permission" True "$(as "$BEA" GET /api/me >/dev/null; body "'administrator' in d['permissions']")"
expect "Ann cannot remove Bea" 403 "$(as "$ANN" DELETE "/api/members/$BEA_ID")"
expect "Bea removes Ann" 204 "$(as "$BEA" DELETE "/api/members/$ANN_ID")"

say "Expiry"
expect "Bea offers it to Cy" 201 "$(as "$BEA" POST /api/guilds/current/guild-master-offer "{\"member_id\":$CY_ID}")"
OFFER=$(body "d['offer']['id']")
"${PSQL[@]}" -c "UPDATE guild_master_offers SET expires_at = now() - interval '1 minute' WHERE id = $OFFER" >/dev/null
expect "an expired offer cannot be accepted" 409 "$(as "$CY" POST "/api/guild-master-offers/$OFFER/accept")"
expect "with its reason" "this offer has expired" "$(body "d['message']")"
expect "and is written expired" expired "$("${PSQL[@]}" -c "SELECT status FROM guild_master_offers WHERE id = $OFFER")"
expect "Bea is still the Guild Master" "$BEA_ID" "$(master "$CY")"
expect "Bea can offer again" 201 "$(as "$BEA" POST /api/guilds/current/guild-master-offer "{\"member_id\":$CY_ID}")"

say "PASS"
