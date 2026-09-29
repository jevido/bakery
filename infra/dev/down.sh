#!/usr/bin/env bash
# Stops every local dev server this repo runs, found by the ports they listen
# on (passed as arguments). The whole process group behind each listener is
# stopped, so `task` and the processes it spawned go down together.
set -u

self=$(ps -o pgid= -p $$ | tr -d ' ')
stopped=0

listeners() {
  if command -v lsof >/dev/null 2>&1; then
    lsof -tiTCP:"$1" -sTCP:LISTEN 2>/dev/null
  else
    ss -ltnpH "sport = :$1" 2>/dev/null | grep -o 'pid=[0-9]*' | cut -d= -f2
  fi
}

for port in "$@"; do
  for pid in $(listeners "$port" | sort -u); do
    pgid=$(ps -o pgid= -p "$pid" 2>/dev/null | tr -d ' ')
    # Never signal our own group or init's.
    if [ -n "$pgid" ] && [ "$pgid" -gt 1 ] && [ "$pgid" != "$self" ]; then
      kill -TERM -- "-$pgid" 2>/dev/null || kill -TERM "$pid" 2>/dev/null
    else
      kill -TERM "$pid" 2>/dev/null
    fi
    echo "stopped :$port ($(ps -o comm= -p "$pid" 2>/dev/null || echo "pid $pid"))"
    stopped=1
  done
done

[ "$stopped" = 1 ] || echo "nothing running on: ${*:-(no ports configured)}"
