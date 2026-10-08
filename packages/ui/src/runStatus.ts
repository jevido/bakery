// A Run's status as both apps show it: the words and Paperclip's hues
// (ui/src/lib/status-colors.ts for heartbeat runs; MIT, see NOTICE).
import type { StatusType } from './statusColors'

/** A Run status, as the agents document names them. */
export type RunStatus = 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled' | 'lost'

/** The statuses a Run never leaves. */
export const finalRunStatuses: readonly RunStatus[] = ['succeeded', 'failed', 'cancelled', 'lost']

/** Whether a Run in this status is over. */
export const runIsFinal = (s: RunStatus) => finalRunStatuses.includes(s)

/** How a Run status reads: "Succeeded". */
export const runStatusLabel = (s: RunStatus) => s.charAt(0).toUpperCase() + s.slice(1)

/** Paperclip's run hues: waiting amber, running and succeeded green, failed
 * red, cancelled grey; a lost Run waits again as a new one, so it is amber. */
export const runStatusTones: Record<RunStatus, StatusType> = {
  queued: 'warning',
  running: 'success',
  succeeded: 'success',
  failed: 'error',
  cancelled: 'neutral',
  lost: 'warning',
}

/** What started a Run, as the agents document names them. */
export type InvocationSource = 'timer' | 'assignment' | 'on_demand' | 'automation'

/** Why a Run was woken, as the agents document names them. */
export type WakeReason = 'manual' | 'heartbeat_invoked' | 'issue_assigned' | 'issue_commented' | 'heartbeat_timer'

/** Paperclip's labels for the Invocation sources (AgentDetail's sourceLabels). */
export const invocationSourceLabels: Record<InvocationSource, string> = {
  timer: 'Timer',
  assignment: 'Assignment',
  on_demand: 'On-demand',
  automation: 'Automation',
}

/** How a Wake reason reads beside the source. */
export const wakeReasonLabels: Record<WakeReason, string> = {
  manual: 'Run',
  heartbeat_invoked: 'Run heartbeat',
  issue_assigned: 'Assigned',
  issue_commented: 'New comment',
  heartbeat_timer: 'Interval',
}
