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
