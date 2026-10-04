import type { StatusType } from '../../lib/ui/StatusBadge.svelte'
import type { ExecutionStatus } from '../../lib/types'

/** What Coolify's backup pages show for a Backup execution's status. */
export function executionStatus(status: ExecutionStatus): { label: string; type: StatusType } {
  switch (status) {
    case 'succeeded':
      return { label: 'Success', type: 'success' }
    case 'running':
      return { label: 'In progress', type: 'warning' }
    case 'failed':
      return { label: 'Failed', type: 'error' }
  }
}
