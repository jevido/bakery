#!/usr/bin/env bash
# Ralph loop: runs fresh headless Claude sessions against PROMPT.md until
# Bakery reaches Coolify parity, a task is blocked, or progress stalls.
# Each session plans the next phase or works the current one (see PROMPT.md).
#
#   .claude/ralph/ralph.sh                 # run until done
#   RALPH_MAX=5 .claude/ralph/ralph.sh     # at most 5 iterations
#
# Progress streams live: logs/<stamp>-<n>.log is a readable transcript (what
# Claude says and every tool it calls), logs/<stamp>-<n>.jsonl the raw
# stream-json events, and logs/loop.log the loop's own lines.
#   tail -f .claude/ralph/logs/$(ls -t .claude/ralph/logs | grep '\.log$' | grep -v loop | head -1)
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

say() { echo "$*" | tee -a "$logs/loop.log"; }
heads() { echo "$(git -C "$repo" rev-parse HEAD) $(git -C "$planning" rev-parse HEAD)"; }

# Turns stream-json events into one readable line each.
readable='
def clip(n): tostring | gsub("\n"; " ⏎ ") | if length > n then .[0:n] + "…" else . end;
def ts: (now | strflocaltime("%H:%M:%S"));
if .type == "system" and .subtype == "init" then
  "\(ts) [start] model \(.model), cwd \(.cwd)"
elif .type == "assistant" then
  .message.content[]? |
  if .type == "text" then "\(ts) [claude] \(.text | clip(2000))"
  elif .type == "tool_use" then
    "\(ts) [\(.name)] " + (
      .input.description // .input.command // .input.file_path // .input.skill
      // .input.pattern // .input.prompt // (.input | tostring) | clip(300))
  else empty end
elif .type == "user" then
  .message.content[]? | select(type == "object" and .type == "tool_result" and .is_error == true) |
  "\(ts)   ! error: \(.content | if type == "array" then (map(.text? // "") | join(" ")) else . end | clip(400))"
elif .type == "result" then
  "\(ts) [done] \(.subtype), \(.num_turns) turns, \((.duration_ms / 60000 * 10 | floor) / 10) min, $\(.total_cost_usd // 0)"
else empty end
'

stalled=0
for ((i = 1; i <= max; i++)); do
  stamp=$(date +%Y%m%d-%H%M%S)
  log="$logs/$stamp-$i.log"
  raw="$logs/$stamp-$i.jsonl"
  before=$(heads)
  say "== ralph iteration $i ($stamp) -> $log"

  args=(-p "$(cat "$here/PROMPT.md")" --permission-mode "$mode" --add-dir "$planning"
    --output-format stream-json --verbose)
  [[ -n "${RALPH_MODEL:-}" ]] && args+=(--model "$RALPH_MODEL")
  [[ -n "${RALPH_BUDGET_USD:-}" ]] && args+=(--max-budget-usd "$RALPH_BUDGET_USD")

  (cd "$repo" && claude "${args[@]}") 2>&1 | tee "$raw" | jq -Rr --unbuffered "fromjson? // {type: \"raw\"} | $readable" | tee "$log"
  status=${PIPESTATUS[0]}

  # Whatever the session did, no dev servers stay up between iterations.
  (cd "$repo" && task down >/dev/null 2>&1)

  # The verdict is the last RALPH line of the session's final answer, not of
  # anything it said along the way.
  final=$(jq -r 'select(.type == "result") | .result // empty' "$raw" 2>/dev/null | tail -c 4000)
  verdict=$(grep -Eo 'RALPH: (DONE|BLOCKED.*|CONTINUE)' <<<"$final" | tail -1)
  case "$verdict" in
    "RALPH: DONE")
      say "== ralph: done after $i iteration(s)"
      exit 0 ;;
    "RALPH: BLOCKED"*)
      say "== ralph: $verdict"
      exit 2 ;;
  esac

  if [[ "$(heads)" == "$before" ]]; then
    stalled=$((stalled + 1))
    say "== ralph: no new commit this iteration ($stalled/$stall_limit, claude exit $status)"
    if ((stalled >= stall_limit)); then
      say "== ralph: stopping, no progress in $stall_limit iterations"
      exit 3
    fi
    sleep 30
  else
    stalled=0
  fi
done
say "== ralph: reached RALPH_MAX=$max"
