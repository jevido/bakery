<script lang="ts">
  // A Run's status pill, with Paperclip's live dot while it runs
  // (IssueRunLedger's running chip; MIT, see NOTICE).
  import StatusBadge from './StatusBadge.svelte'
  import { runStatusLabel, runStatusTones, type RunStatus } from './runStatus'

  let { status, class: className = '' }: { status: RunStatus; class?: string } = $props()
</script>

<StatusBadge type={runStatusTones[status]} class={className}>
  {#if status === 'running'}
    <span class="relative flex size-1.5" aria-hidden="true">
      <span class="absolute inline-flex size-full animate-ping rounded-full bg-current opacity-70 motion-reduce:animate-none"></span>
      <span class="relative inline-flex size-1.5 rounded-full bg-current"></span>
    </span>
  {/if}
  <span data-run-status={status}>{runStatusLabel(status)}</span>
</StatusBadge>
