<script lang="ts">
  // What started a Run, as Paperclip's run rows badge it (AgentDetail's
  // source badge and its hues; MIT, see NOTICE), with the Wake reason as its
  // title and "×N" when more Wakes joined the same Run.
  import {
    invocationSourceLabels,
    wakeReasonLabels,
    type InvocationSource,
    type WakeReason,
  } from './runStatus'

  let {
    source,
    reason,
    count = 1,
  }: { source: InvocationSource; reason: WakeReason; count?: number } = $props()

  const hues: Record<InvocationSource, string> = {
    timer: 'bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300',
    assignment: 'bg-violet-100 text-violet-700 dark:bg-violet-900/50 dark:text-violet-300',
    on_demand: 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/50 dark:text-cyan-300',
    automation: 'bg-muted text-muted-foreground',
  }
  const why = $derived(wakeReasonLabels[reason] ?? reason)
</script>

<span class="inline-flex items-center gap-1" data-run-source={source}>
  <span class="rounded-md px-1.5 py-0.5 text-[10px] font-medium {hues[source] ?? hues.automation}" title={why}>
    {invocationSourceLabels[source] ?? source}
  </span>
  <span class="text-[10px] text-muted-foreground">{why}</span>
  {#if count > 1}<span class="text-[10px] font-medium text-muted-foreground tabular-nums" title={`${count} wakes joined this run`} data-wake-count={count}>×{count}</span>{/if}
</span>
