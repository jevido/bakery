import { statusType, type StatusType } from '../../lib/statusColors'
import type { ExecutionStatus } from '../../lib/types'

const labels: Record<ExecutionStatus, string> = { succeeded: 'Success', running: 'In progress', failed: 'Failed' }

/** What Coolify's backup pages show for a Backup execution's status. */
export function executionStatus(status: ExecutionStatus): { label: string; type: StatusType } {
  // A running execution is still in flight, not up: warning, as Coolify draws it.
  return { label: labels[status], type: status === 'running' ? 'warning' : statusType(status) }
}
