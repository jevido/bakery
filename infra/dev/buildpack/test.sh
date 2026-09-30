#!/usr/bin/env bash
# End to end against the Forgejo stand-in: every Build pack. A static site
# and a Node app without a Dockerfile (Nixpacks) from a repository, a public
# image, and a private image in Forgejo's container registry, with a moved
# tag and a rollback to the image pulled first. Needs `task dev` running (API on 127.0.0.1:4910, proxy
# on 4943). Starts Forgejo (compose profile git) and removes it and
# everything the test created when done.
set -euo pipefail

# shellcheck source=SCRIPTDIR/../lib/e2e.sh
. "$(dirname "$0")/../lib/e2e.sh"

REGISTRY_IMAGE="127.0.0.1:4950/$FORGEJO_USER/$RUN"
V2_IMAGE="localhost/bakery-e2e-v2:$RUN"
e2e_cleanup_hook() {
	podman rmi -f "$REGISTRY_IMAGE:1" "$V2_IMAGE" >/dev/null 2>&1
}

sign_in
start_forgejo
create_repo

say "Static site"
mkdir -p "$REPO/public"
echo "static marker $RUN" >"$REPO/public/index.html"
echo "not published" >"$REPO/secret.txt"
push "A static site"
create_app 80 ',"build_pack":"static","publish_directory":"public","health_check":{"enabled":true,"path":"/","interval":1,"timeout":2,"retries":5}'
bakery POST "/api/applications/$APP_ID/deploy" >/dev/null
wait_for 300 "the static deployment" deployment_done
grep -q "Healthy after" <<<"$(bakery GET "/api/deployments/$(latest "['id']")/log")" || fail "the static site was not health checked"
[ "$(fetch /)" = "static marker $RUN" ] || fail "static site answers: $(fetch / || true)"
if fetch /secret.txt >/dev/null; then fail "a file outside the publish directory is served"; fi

say "Nixpacks: a Node app without a Dockerfile"
rm -rf "${REPO:?}/public" "$REPO/secret.txt"
cat >"$REPO/package.json" <<'JSON'
{"name":"hello","version":"1.0.0","scripts":{"start":"node index.js"}}
JSON
cat >"$REPO/index.js" <<'JS'
require("http").createServer((q, r) => r.end("nixpacks " + process.env.GREETING + " " + process.env.NODE_ENV)).listen(3000)
JS
push "A Node app"
new_app "{\"name\":\"$RUN-node\",\"git_url\":\"ssh://git@127.0.0.1:4952/$FORGEJO_USER/$RUN.git\",\"port\":3000,\"build_pack\":\"nixpacks\"}"
bakery PUT "/api/applications/$APP_ID/env" '{"env":[{"name":"GREETING","value":"hello","build":true,"runtime":true}]}' >/dev/null
bakery POST "/api/applications/$APP_ID/deploy" >/dev/null
wait_for 900 "the nixpacks deployment" deployment_done
[ "$(fetch /)" = "nixpacks hello production" ] || fail "nixpacks app answers: $(fetch / || true)"

# Not Docker Hub: it rate limits pulls, and every image deploy pulls.
say "Image: a public image"
new_app "{\"name\":\"$RUN-public\",\"build_pack\":\"image\",\"image_reference\":\"ghcr.io/traefik/whoami:v1.10\",\"port\":80}"
bakery POST "/api/applications/$APP_ID/deploy" >/dev/null
wait_for 300 "the public image deployment" deployment_done
grep -q '^Hostname:' <<<"$(fetch / || true)" || fail "whoami answers: $(fetch / || true)"
[[ $(latest "['source_image']") == ghcr.io/traefik/whoami@sha256:* ]] || fail "source image: $(latest "['source_image']")"

say "Image: a private image in Forgejo's registry"
# Private user, so its packages need credentials.
forgejo PATCH "/admin/users/$FORGEJO_USER" "{\"login_name\":\"$FORGEJO_USER\",\"source_id\":0,\"visibility\":\"private\"}" >/dev/null
podman pull -q ghcr.io/traefik/whoami:v1.10 >/dev/null
podman tag ghcr.io/traefik/whoami:v1.10 "$REGISTRY_IMAGE:1"
podman push -q --tls-verify=false --creds "$FORGEJO_USER:$TOKEN" "$REGISTRY_IMAGE:1"
new_app "{\"name\":\"$RUN-private\",\"build_pack\":\"image\",\"image_reference\":\"$REGISTRY_IMAGE:1\",\"port\":80}"
bakery POST "/api/applications/$APP_ID/deploy" >/dev/null
failed() { [ "$(latest "['status']")" = failed ]; }
wait_for 120 "the deployment without credentials to fail" failed
grep -qiE 'unauthori[sz]ed|authentication' <<<"$(latest "['error']")" || fail "error without credentials: $(latest "['error']")"
bakery PATCH "/api/applications/$APP_ID" "{\"name\":\"$RUN-private\",\"image_reference\":\"$REGISTRY_IMAGE:1\",\"port\":80,\"registry_credentials\":{\"username\":\"$FORGEJO_USER\",\"password\":\"$TOKEN\"}}" >/dev/null
FIRST=$(bakery POST "/api/applications/$APP_ID/deploy" | json "d['deployment']['id']")
wait_for 120 "the deployment with credentials" deployment_done
grep -q '^Hostname:' <<<"$(fetch / || true)" || fail "private whoami answers: $(fetch / || true)"
if grep -qF "$TOKEN" <<<"$(bakery GET "/api/deployments/$FIRST/log")"; then fail "the registry token is in the deployment log"; fi

say "Image: move the tag, redeploy, roll back"
cat >"$WORK/Containerfile" <<'CONTAINERFILE'
FROM docker.io/library/busybox:stable
RUN mkdir /www && echo "version 2" > /www/index.html
CMD ["httpd", "-f", "-p", "80", "-h", "/www"]
CONTAINERFILE
podman build -q -t "$V2_IMAGE" -f "$WORK/Containerfile" "$WORK" >/dev/null
podman tag "$V2_IMAGE" "$REGISTRY_IMAGE:1"
podman push -q --tls-verify=false --creds "$FORGEJO_USER:$TOKEN" "$REGISTRY_IMAGE:1"
bakery POST "/api/applications/$APP_ID/deploy" >/dev/null
wait_for 120 "the deployment of the moved tag" deployment_done
serves_v2() { [ "$(fetch /)" = "version 2" ]; }
wait_for 30 "version 2" serves_v2
bakery POST "/api/deployments/$FIRST/rollback" >/dev/null
wait_for 120 "the rollback" deployment_done
serves_v1() { grep -q '^Hostname:' <<<"$(fetch / || true)"; }
wait_for 30 "the first image again" serves_v1

say "PASS"
