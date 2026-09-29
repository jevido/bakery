<script lang="ts">
  import type { DeploymentStatus } from './types'

  let { status }: { status: DeploymentStatus } = $props()
</script>

<span class={['badge', status]}>{status}</span>

<style>
  .badge {
    display: inline-block;
    padding: 0.05rem 0.5rem;
    border-radius: 999px;
    font-size: 0.75rem;
    font-weight: 600;
    border: 1px solid currentColor;
    color: var(--warn);
    white-space: nowrap;
  }
  .finished {
    color: var(--ok);
  }
  .failed {
    color: var(--danger);
  }
</style>
