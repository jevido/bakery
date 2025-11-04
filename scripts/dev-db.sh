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

echo "Starting development database (Ctrl+C to stop)..."
echo "Run 'bun run migrate' in another terminal once Postgres is up."
echo
exec "${COMPOSE_CMD[@]}" up
