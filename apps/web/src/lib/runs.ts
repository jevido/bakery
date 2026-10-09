// The agents context's Runs: an Agent's work on an Issue, executed by
// `claude` on its Hirer's desktop app, and each Run's Transcript. Every
// call answers in the Current guild.
import { api } from './api'
import type { InvocationSource, RunStatus, WakeReason } from '@bakery/ui/runStatus'
import type { RunEvent } from '@bakery/ui/runTranscript'

export type { InvocationSource, RunStatus, WakeReason } from '@bakery/ui/runStatus'
export type { RunEvent } from '@bakery/ui/runTranscript'

/** What a Run used, as the CLI reported it; the cost is an equivalent only. */
export type RunUsage = {
  input_tokens: number
  cached_input_tokens: number
  output_tokens: number
  turns: number
  cost_equivalent_usd: number
  duration_ms: number
}

export type Run = {
  id: number
  agent: { id: number; name: string; icon: string }
  issue: { id: number; identifier: string; title: string } | null
  invocation_source: InvocationSource
  wake_reason: WakeReason
  /** How many Wakes this Run took: more than one when later ones joined it. */
  wake_count: number
  status: RunStatus
  requested_by: { id: number; name: string } | null
  desktop: { id: number; name: string } | null
  retry_of_run_id: number | null
  usage: RunUsage
  exit_code: number | null
  error: string
  /** When the Hirer's Subscription limit resets, on a limited Run. */
  limit_resets_at: string | null
  /** While a queued Run waits on its Hirer's Desktop limit: when it resets. */
  subscription_limit_resets_at: string | null
  created_at: string
  started_at: string | null
  finished_at: string | null
  /** Whether the asker may cancel it: not final, and they may manage its Agent. */
  can_cancel: boolean
}

/** Which Runs to list; each filter is optional, newest first. */
export type RunFilter = { issue?: number; agent?: number; status?: RunStatus; limit?: number }

export const listRuns = (f: RunFilter) => {
  const q = new URLSearchParams()
  for (const [k, v] of Object.entries(f)) if (v !== undefined) q.set(k, String(v))
  return api<{ runs: Run[] }>('GET', `/runs?${q}`).then((r) => r.runs)
}
export const startRun = (agentId: number, issueId: number) =>
  api<{ run: Run }>('POST', `/agents/${agentId}/runs`, { issue_id: issueId }).then((r) => r.run)
/** Wakes the Agent on demand without an Issue; joins its queued Run if it has one. */
export const runHeartbeat = (agentId: number) =>
  api<{ run: Run }>('POST', `/agents/${agentId}/heartbeat`).then((r) => r.run)
export const cancelRun = (id: number) => api<{ run: Run }>('POST', `/runs/${id}/cancel`).then((r) => r.run)
export const runEvents = (id: number, after = 0) =>
  api<{ events: RunEvent[] }>('GET', `/runs/${id}/events?after=${after}`).then((r) => r.events)

/**
 * Follows a Run's Transcript live: every stored event, then each new one as
 * the Desktop reports it, and the Run whenever its status or usage changes.
 * The stream ends with the Run; the returned function stops it earlier.
 */
export function followRun(id: number, onEvent: (e: RunEvent) => void, onStatus: (r: Run) => void): () => void {
  const es = new EventSource(`/api/runs/${id}/stream`)
  es.addEventListener('event', (e) => onEvent(JSON.parse((e as MessageEvent).data)))
  es.addEventListener('status', (e) => onStatus(JSON.parse((e as MessageEvent).data).run))
  // After `end` EventSource must not reconnect.
  es.addEventListener('end', () => es.close())
  return () => es.close()
}

/** How long a Run ran, or has been running: "4.2 s". */
export function runTime(r: Pick<Run, 'started_at' | 'finished_at' | 'usage'>, now = Date.now()): number | null {
  if (r.usage.duration_ms > 0) return r.usage.duration_ms
  if (!r.started_at) return null
  return (r.finished_at ? Date.parse(r.finished_at) : now) - Date.parse(r.started_at)
}
