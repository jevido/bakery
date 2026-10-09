// The Bakery's one status palette, in the shape of Paperclip's statusBadge map
// (ui/src/lib/status-colors.ts, MIT, see NOTICE): every state a Deployment,
// Backup execution, Database, Service, Server, Container, notification
// delivery, Goal, Routine or Routine run can be in maps to a tone, and each tone to the tinted pill classes
// built from theme.css's success, warning, error and muted tokens.

export type StatusType = 'neutral' | 'success' | 'warning' | 'error'

export const statusTypes: Record<string, StatusType> = {
  // Done and up.
  running: 'success',
  healthy: 'success',
  finished: 'success',
  succeeded: 'success',
  reachable: 'success',
  sent: 'success',
  // A Goal under way or reached.
  active: 'success',
  achieved: 'success',
  // An Approval decided for.
  approved: 'success',
  // A Routine run whose Execution Issue is done.
  completed: 'success',
  // Waiting or in flight.
  queued: 'warning',
  pending: 'warning',
  cloning: 'warning',
  building: 'warning',
  starting: 'warning',
  restarting: 'warning',
  deploying: 'warning',
  in_progress: 'warning',
  revision_requested: 'warning',
  degraded: 'warning',
  // A Routine whose automatic triggers are off.
  paused: 'warning',
  unvalidated: 'warning',
  // Down or broken.
  failed: 'error',
  exited: 'error',
  stopped: 'error',
  unhealthy: 'error',
  unreachable: 'error',
  rejected: 'error',
  // Neither.
  cancelled: 'neutral',
  missing: 'neutral',
  planned: 'neutral',
}

/** The tone of a raw state such as `running` or `running:healthy` (its part before the colon). */
export function statusType(state: string): StatusType {
  return statusTypes[state.toLowerCase().split(':')[0].trim()] ?? 'neutral'
}

export const statusBadgeClasses: Record<StatusType, string> = {
  neutral: 'bg-muted text-muted-foreground',
  success: 'bg-success/15 text-success',
  warning: 'bg-warning/15 text-warning-700 dark:text-warning',
  error: 'bg-destructive/15 text-destructive',
}

/** The small dot beside a status, as Paperclip's run rows draw it. */
export const statusDotClasses: Record<StatusType, string> = {
  neutral: 'bg-muted-foreground/60',
  success: 'bg-success',
  warning: 'bg-warning',
  error: 'bg-destructive',
}
