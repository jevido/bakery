#!/usr/bin/env bash
# Ralph loop: runs fresh headless Claude sessions against PROMPT.md until
# Bakery reaches Coolify parity, a task is blocked, or progress stalls.
# Each session plans the next phase or works the current one (see PROMPT.md).
#
#   .claude/ralph/ralph.sh                 # run until done
#   RALPH_MAX=5 .claude/ralph/ralph.sh     # at most 5 iterations
#
# Env: RALPH_MAX (default 200), RALPH_STALL (iterations without a new commit
# before stopping, default 3), RALPH_PERMISSION_MODE (default auto),
# RALPH_MODEL (default: the CLI default), RALPH_BUDGET_USD (per iteration).
set -uo pipefail

here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../.." && pwd)
planning=/home/jevido/Projects/planning
logs="$here/logs"
mkdir -p "$logs" "$here/state"

max=${RALPH_MAX:-200}
stall_limit=${RALPH_STALL:-3}
mode=${RALPH_PERMISSION_MODE:-auto}

heads() { echo "$(git -C "$repo" rev-parse HEAD) $(git -C "$planning" rev-parse HEAD)"; }

stalled=0
for ((i = 1; i <= max; i++)); do
  stamp=$(date +%Y%m%d-%H%M%S)
  log="$logs/$stamp-$i.log"
  before=$(heads)
  echo "== ralph iteration $i ($stamp) -> $log"

  args=(-p "$(cat "$here/PROMPT.md")" --permission-mode "$mode" --add-dir "$planning" --output-format text)
  [[ -n "${RALPH_MODEL:-}" ]] && args+=(--model "$RALPH_MODEL")
  [[ -n "${RALPH_BUDGET_USD:-}" ]] && args+=(--max-budget-usd "$RALPH_BUDGET_USD")

  (cd "$repo" && claude "${args[@]}") 2>&1 | tee "$log"
  status=${PIPESTATUS[0]}

  # Whatever the session did, no dev servers stay up between iterations.
  (cd "$repo" && task down >/dev/null 2>&1)

  verdict=$(grep -Eo 'RALPH: (DONE|BLOCKED.*|CONTINUE)' "$log" | tail -1)
  case "$verdict" in
    "RALPH: DONE")
      echo "== ralph: done after $i iteration(s)"
      exit 0 ;;
    "RALPH: BLOCKED"*)
      echo "== ralph: $verdict"
      exit 2 ;;
  esac

  if [[ "$(heads)" == "$before" ]]; then
    stalled=$((stalled + 1))
    echo "== ralph: no new commit this iteration ($stalled/$stall_limit, claude exit $status)"
    if ((stalled >= stall_limit)); then
      echo "== ralph: stopping, no progress in $stall_limit iterations"
      exit 3
    fi
    sleep 30
  else
    stalled=0
  fi
done
echo "== ralph: reached RALPH_MAX=$max"
