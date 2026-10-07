#!/usr/bin/env bash
# End to end: Roles and the hierarchy in "Default". The Owner invites Al as
# an admin and Bo as a member. Al creates "Deployer" (view resources and
# deploy), drags it just below Admin and gives it to Bo, with a second Role
# that has manage_roles and manage_members. Bo still cannot edit or assign
# Admin or remove Al, and Al cannot remove the Guild Master. An Invitation
# with role_ids gives exactly those Roles. On a Project, a deny of deploy
# for Deployer stops Bo there only, until Al allows it to Bo himself, and a
# deny of view resources hides another Project from Bo but not from Al.
# Needs `task dev` running (API on 127.0.0.1:4910).
set -euo pipefail

# Nothing here uses Forgejo; leave it as it is.
KEEP_FORGEJO=1
# shellcheck source=SCRIPTDIR/../lib/e2e.sh
. "$(dirname "$0")/../lib/e2e.sh"

MEMBER_IDS=() ROLE_IDS=() P2_ID="" P2_APP=""
e2e_cleanup_hook() {
	local id
	if [ -n "$P2_APP" ]; then bakery DELETE "/api/applications/$P2_APP" >/dev/null; fi
	if [ -n "$P2_ID" ]; then bakery DELETE "/api/projects/$P2_ID" >/dev/null; fi
	for id in "${MEMBER_IDS[@]}"; do bakery DELETE "/api/members/$id" >/dev/null; done
	for id in "${ROLE_IDS[@]}"; do bakery DELETE "/api/roles/$id" >/dev/null; done
	for id in $(bakery GET /api/invitations 2>/dev/null | json "' '.join(str(i['id']) for i in d['invitations'] if '$RUN' in i['email'])"); do
		bakery DELETE "/api/invitations/$id" >/dev/null
	done
}

# as JAR METHOD PATH [JSON]: the status code of a request with a Member's
# Session; the body is in $WORK/body.
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
# invite JSON: the token of a new Invitation's link, made by the Owner.
invite() {
	bakery POST /api/invitations "$1" | json "d['path'].rsplit('/', 1)[1]"
}
# join TOKEN JAR NAME: accepts an Invitation as a new Member, signing in
# with JAR; prints their id.
join() {
	as "$2" POST "/api/invitations/by-token/$1/accept" "{\"name\":\"$3\",\"password\":\"correct horse battery\"}" >/dev/null
	body "d['member']['id']"
}
# roles_of MEMBER: the names of the Roles the Member holds, top first.
roles_of() { bakery GET /api/members | json "[[r['name'] for r in m['roles']] for m in d['members'] if m['id'] == $1][0]"; }
# order: the Role names of the Current guild, top first, @everyone last.
order() { bakery GET /api/roles | json "[r['name'] for r in d['roles']]"; }

sign_in
OWNER_ID=$(bakery GET /api/me | json "d['member']['id']")
DEPLOYER="Deployer $RUN" KEEPER="Keeper $RUN"

say "Al, an admin, and Bo, a member, join"
AL=$WORK/al BO=$WORK/bo CY=$WORK/cy
AL_ID=$(join "$(invite "{\"email\":\"al-$RUN@example.com\",\"role\":\"admin\"}")" "$AL" Al)
MEMBER_IDS+=("$AL_ID")
BO_ID=$(join "$(invite "{\"email\":\"bo-$RUN@example.com\",\"role_ids\":[$(role_id Member)]}")" "$BO" Bo)
MEMBER_IDS+=("$BO_ID")
expect "Bo holds Member" "['Member']" "$(roles_of "$BO_ID")"
expect "every Member lists the Permissions" 13 "$(as "$BO" GET /api/permissions >/dev/null; body "len(d['permissions'])")"
expect "four of them can be overridden" "['view_resources', 'see_secrets', 'deploy', 'manage_applications']" "$(body "[p['key'] for p in d['permissions'] if p['overridable']]")"
expect "every Member lists the Roles" 200 "$(as "$BO" GET /api/roles)"
expect "a member cannot create a Role" 403 "$(as "$BO" POST /api/roles '{"name":"x"}')"

say "Al creates Deployer and drags it just below Admin"
expect "Al creates Deployer" 201 "$(as "$AL" POST /api/roles "{\"name\":\"$DEPLOYER\",\"color\":\"#E67E22\",\"permissions\":[\"view_resources\",\"deploy\"]}")"
DEPLOYER_ID=$(body "d['role']['id']")
ROLE_IDS+=("$DEPLOYER_ID")
expect "it sits just above @everyone" "1 #e67e22" "$(body "d['role']['position'], d['role']['color']" | tr -d "(),'")"
expect "Al creates Keeper" 201 "$(as "$AL" POST /api/roles "{\"name\":\"$KEEPER\",\"permissions\":[\"manage_roles\",\"manage_members\"]}")"
KEEPER_ID=$(body "d['role']['id']")
ROLE_IDS+=("$KEEPER_ID")
NEW_ORDER=$(bakery GET /api/roles | json "(lambda ids: ids[:1] + [$DEPLOYER_ID] + ids[1:])([r['id'] for r in d['roles'] if not r['base'] and r['id'] != $DEPLOYER_ID])")
expect "Al drags Deployer below Admin" 200 "$(as "$AL" PUT /api/roles/order "{\"role_ids\":$NEW_ORDER}")"
expect "the Roles are in the new order" True "$(body "[r['name'] for r in d['roles']][:2] == ['Admin', '$DEPLOYER'] and d['roles'][-1]['name'] == '@everyone'")"
expect "the Positions are contiguous" True "$(body "[r['position'] for r in d['roles']] == list(range(len(d['roles']) - 1, -1, -1))")"
expect "Al gives Bo Deployer" 200 "$(as "$AL" PUT "/api/members/$BO_ID/roles/$DEPLOYER_ID")"
expect "Al gives Bo Keeper" 200 "$(as "$AL" PUT "/api/members/$BO_ID/roles/$KEEPER_ID")"
expect "Bo holds all three, top first" "['$DEPLOYER', 'Member', '$KEEPER']" "$(roles_of "$BO_ID")"
expect "Deployer counts its Member" 1 "$(bakery GET /api/roles | json "[r['members'] for r in d['roles'] if r['id'] == $DEPLOYER_ID][0]")"

say "Bo, with manage_roles and manage_members, still cannot touch Admin or Al"
CY_ID=$(join "$(invite "{\"email\":\"cy-$RUN@example.com\",\"role_ids\":[]}")" "$CY" Cy)
MEMBER_IDS+=("$CY_ID")
expect "Cy holds only @everyone" "[]" "$(roles_of "$CY_ID")"
ADMIN_ID=$(role_id Admin)
expect "Bo cannot edit Admin" 403 "$(as "$BO" PATCH "/api/roles/$ADMIN_ID" '{"name":"Boss"}')"
expect "Bo cannot assign Admin" 403 "$(as "$BO" PUT "/api/members/$CY_ID/roles/$ADMIN_ID")"
expect "Bo cannot remove Al" 403 "$(as "$BO" DELETE "/api/members/$AL_ID")"
expect "Bo cannot edit Deployer, his own highest" 403 "$(as "$BO" PATCH "/api/roles/$DEPLOYER_ID" '{"color":"#000000"}')"
expect "Bo cannot grant manage_servers, which he lacks" 403 "$(as "$BO" PATCH "/api/roles/$KEEPER_ID" '{"permissions":["manage_roles","manage_members","manage_servers"]}')"
expect "Bo assigns Member to Cy, below his own" 200 "$(as "$BO" PUT "/api/members/$CY_ID/roles/$(role_id Member)")"
expect "Bo cannot move Deployer above Admin" 403 "$(as "$BO" PUT /api/roles/order "{\"role_ids\":$(bakery GET /api/roles | json "(lambda ids: [ids[1], ids[0]] + ids[2:])([r['id'] for r in d['roles'] if not r['base']])")}")"
expect "Bo cannot rename @everyone" 403 "$(as "$BO" PATCH "/api/roles/$(role_id @everyone)" '{"name":"all"}')"
expect "another Guild's Role is not found" 404 "$(as "$BO" PATCH /api/roles/999999999 '{"color":"#000000"}')"
expect "the former role change is gone" 410 "$(as "$AL" PATCH "/api/members/$CY_ID" '{"role":"member"}')"
expect "Al cannot remove the Guild Master" 403 "$(as "$AL" DELETE "/api/members/$OWNER_ID")"
expect "Al removes Cy" 204 "$(as "$AL" DELETE "/api/members/$CY_ID")"

say "An Invitation with role_ids gives exactly those Roles"
expect "Al invites with Deployer and Keeper" 201 "$(as "$AL" POST /api/invitations "{\"email\":\"di-$RUN@example.com\",\"role_ids\":[$DEPLOYER_ID,$KEEPER_ID]}")"
expect "the Invitation names them" "['$DEPLOYER', '$KEEPER']" "$(body "[r['name'] for r in d['invitation']['roles']]")"
DI_TOKEN=$(body "d['path'].rsplit('/', 1)[1]")
expect "Bo cannot invite with Admin" 403 "$(as "$BO" POST /api/invitations "{\"email\":\"x-$RUN@example.com\",\"role_ids\":[$ADMIN_ID]}")"
DI_ID=$(join "$DI_TOKEN" "$WORK/di" Di)
MEMBER_IDS+=("$DI_ID")
expect "Di holds exactly those" "['$DEPLOYER', '$KEEPER']" "$(roles_of "$DI_ID")"

say "Permission overrides on a Project"
# An image that does not exist: a deploy is accepted, then fails at once.
IMAGE_APP="\"build_pack\":\"dockerimage\",\"docker_image\":\"localhost/bakery-e2e-none:$RUN\",\"port\":80"
new_app "{\"name\":\"p1-$RUN\",$IMAGE_APP}"
P1_ID=$PROJECT_ID P1_APP=$APP_ID
P2_ID=$(bakery POST /api/projects "{\"name\":\"p2-$RUN\"}" | json "d['project']['id']")
P2_ENV=$(bakery GET "/api/projects/$P2_ID" | json "d['project']['environments'][0]['id']")
P2_APP=$(bakery POST "/api/environments/$P2_ENV/applications" "{\"name\":\"p2-$RUN\",$IMAGE_APP}" | json "d['application']['id']")
expect "Bo cannot override Deployer, his own highest" 403 "$(as "$BO" PUT "/api/projects/$P1_ID/permissions/roles/$DEPLOYER_ID" '{"deny":["deploy"]}')"
expect "manage_servers cannot be overridden" 422 "$(as "$AL" PUT "/api/projects/$P1_ID/permissions/roles/$DEPLOYER_ID" '{"deny":["manage_servers"]}')"
expect "Al denies deploy to Deployer on P1" 200 "$(as "$AL" PUT "/api/projects/$P1_ID/permissions/roles/$DEPLOYER_ID" '{"deny":["deploy"]}')"
expect "the override reads back" "[None, ['deploy']]" "$(as "$AL" GET "/api/projects/$P1_ID/permissions" >/dev/null; body "[[o['member_id'], o['deny']] for o in d['overrides'] if o['role_id'] == $DEPLOYER_ID][0]")"
expect "Bo's Permissions on P1 lack deploy" False "$(as "$BO" GET "/api/projects/$P1_ID" >/dev/null; body "'deploy' in d['project']['permissions']")"
expect "Bo cannot deploy in P1" 403 "$(as "$BO" POST "/api/applications/$P1_APP/deploy")"
expect "Bo deploys in P2" 201 "$(as "$BO" POST "/api/applications/$P2_APP/deploy")"
expect "Al, an admin, keeps deploy on P1" True "$(as "$AL" GET "/api/projects/$P1_ID" >/dev/null; body "'deploy' in d['project']['permissions']")"
expect "Al allows deploy to Bo himself on P1" 200 "$(as "$AL" PUT "/api/projects/$P1_ID/permissions/members/$BO_ID" '{"allow":["deploy"]}')"
expect "Bo deploys in P1 now" 201 "$(as "$BO" POST "/api/applications/$P1_APP/deploy")"
expect "Al denies view resources to Deployer on P2" 200 "$(as "$AL" PUT "/api/projects/$P2_ID/permissions/roles/$DEPLOYER_ID" '{"deny":["view_resources"]}')"
expect "Bo's Projects lack P2" "True False" "$(as "$BO" GET /api/projects >/dev/null; body "' '.join(str(any(p['id'] == i for p in d['projects'])) for i in ($P1_ID, $P2_ID))")"
expect "P2's Application is not found for Bo" 404 "$(as "$BO" GET "/api/applications/$P2_APP")"
expect "nor are its Deployments" 404 "$(as "$BO" GET "/api/applications/$P2_APP/deployments")"
expect "Al still sees P2" True "$(as "$AL" GET /api/projects >/dev/null; body "any(p['id'] == $P2_ID for p in d['projects'])")"
expect "Al clears Bo's own override" 204 "$(as "$AL" DELETE "/api/projects/$P1_ID/permissions/members/$BO_ID")"
expect "Bo cannot deploy in P1 again" 403 "$(as "$BO" POST "/api/applications/$P1_APP/deploy")"

say "Deleting a Role"
expect "Al deletes Deployer" 204 "$(as "$AL" DELETE "/api/roles/$DEPLOYER_ID")"
expect "Bo no longer holds it" "['Member', '$KEEPER']" "$(roles_of "$BO_ID")"
expect "@everyone cannot be deleted" 403 "$(as "$AL" DELETE "/api/roles/$(role_id @everyone)")"
expect "the Positions are contiguous again" True "$(bakery GET /api/roles | json "[r['position'] for r in d['roles']] == list(range(len(d['roles']) - 1, -1, -1))")"
echo "PASS"
