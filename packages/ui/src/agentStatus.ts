// An Agent's status as both apps show it: the words and Paperclip's hues
// (ui/src/lib/status-colors.ts agentStatusDot; MIT, see NOTICE).
import type { StatusType } from './statusColors'

/** An Agent status; running and error follow its Runs. */
export type AgentStatus = 'pending_approval' | 'idle' | 'running' | 'error' | 'paused' | 'terminated'

/** How an Agent status reads: "Pending approval". */
export const agentStatusLabel = (s: AgentStatus) => s.charAt(0).toUpperCase() + s.slice(1).replace(/_/g, ' ')

/** Paperclip's agent status hues: idle grey, running green, paused amber,
 * error red; a waiting hire is amber too and a terminated Agent red. */
export const agentStatusTones: Record<AgentStatus, StatusType> = {
  idle: 'neutral',
  running: 'success',
  error: 'error',
  paused: 'warning',
  pending_approval: 'warning',
  terminated: 'error',
}

/** Why a paused Agent is paused; any other status has none. */
export type PauseReason = 'manual' | 'budget'

/** How an Agent's status reads, "Paused by budget" when its Budget's Hard stop paused it. */
export const agentStatusText = (a: { status: AgentStatus; pause_reason?: PauseReason | null }) =>
  a.status === 'paused' && a.pause_reason === 'budget' ? 'Paused by budget' : agentStatusLabel(a.status)
