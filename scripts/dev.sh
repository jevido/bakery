#!/usr/bin/env bash
set -euo pipefail

COMPOSE_FILE="docker-compose.dev.yml"
if [ ! -f "$COMPOSE_FILE" ]; then
  echo "Missing $COMPOSE_FILE. Please create it before running dev." >&2
  exit 1
fi

if command -v docker-compose >/dev/null 2>&1; then
  COMPOSE_CMD=(docker-compose -f "$COMPOSE_FILE")
else
  COMPOSE_CMD=(docker compose -f "$COMPOSE_FILE")
fi

echo "Bringing up development database..."
"${COMPOSE_CMD[@]}" up -d

echo -n "Waiting for PostgreSQL to accept connections"
until "${COMPOSE_CMD[@]}" exec -T postgres pg_isready -U bakery -d bakery >/dev/null 2>&1; do
  printf '.'
  sleep 2
done
echo " done"

echo "Running database migrations..."
bun backend/lib/migrate.js
echo "Migrations complete. Starting backend (Ctrl+C to stop)."
echo
echo "Tip: run 'cd app && bun run dev' in another terminal for the SvelteKit UI."
echo "When you're done, stop Postgres with 'docker compose -f $COMPOSE_FILE down'."

exec bun backend/server.js
