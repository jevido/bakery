#!/usr/bin/env bash
# Prepares the Garage stand-in started by `task s3:up`: one node with a
# layout, the bucket bakery-backups and the key bakery-dev allowed on it.
# Writes the endpoint and key to infra/dev/state/garage.env (gitignored)
# for tests. Safe to run again.
set -euo pipefail

cd "$(dirname "$0")/../../.."
CTR=bakery-dev-garage-1
STATE=infra/dev/state
garage() { podman exec "$CTR" /garage "$@" 2>/dev/null; }

until=$((SECONDS + 60))
until garage status >/dev/null; do
	[ $SECONDS -lt $until ] || { echo "garage did not start" >&2; exit 1; }
	sleep 1
done

status=$(garage status)
if grep -q 'NO ROLE ASSIGNED' <<<"$status"; then
	node=$(awk '/HEALTHY NODES/ { getline; getline; print $1; exit }' <<<"$status")
	garage layout assign -z dc1 -c 1G "$node" >/dev/null
	garage layout apply --version 1 >/dev/null
fi

garage bucket info bakery-backups >/dev/null || garage bucket create bakery-backups >/dev/null
if ! garage key info bakery-dev >/dev/null; then
	garage key create bakery-dev >/dev/null
fi
garage bucket allow --read --write --owner bakery-backups --key bakery-dev >/dev/null

info=$(garage key info --show-secret bakery-dev)
access=$(awk '/^Key ID:/ { print $3 }' <<<"$info")
secret=$(awk '/^Secret key:/ { print $3 }' <<<"$info")
[ -n "$access" ] && [ -n "$secret" ] || { echo "could not read the bakery-dev key" >&2; exit 1; }

mkdir -p "$STATE"
umask 077
cat >"$STATE/garage.env" <<ENV
GARAGE_ENDPOINT=http://127.0.0.1:4960
GARAGE_REGION=garage
GARAGE_BUCKET=bakery-backups
GARAGE_ACCESS_KEY=$access
GARAGE_SECRET_KEY=$secret
ENV
echo "garage: bucket bakery-backups ready, key in $STATE/garage.env"
