<script lang="ts">
  import type { ExecutionStatus, DatabaseStatus, DeploymentStatus, ServerStatus, ServiceStatus } from './types'

  let { status }: { status: DeploymentStatus | DatabaseStatus | ExecutionStatus | ServiceStatus | ServerStatus } = $props()
</script>

<span class={['status-badge', status]}>{status}</span>

<style>
  .status-badge {
    display: inline-block;
    padding: 0.05rem 0.5rem;
    border-radius: 999px;
    font-size: 0.75rem;
    font-weight: 600;
    border: 1px solid currentColor;
    color: var(--warn);
    white-space: nowrap;
  }
  .finished,
  .succeeded,
  .running,
  .reachable {
    color: var(--ok);
  }
  .failed,
  .exited,
  .degraded,
  .unreachable {
    color: var(--danger);
  }
  .cancelled,
  .stopped,
  .missing {
    color: var(--muted, gray);
  }
</style>
