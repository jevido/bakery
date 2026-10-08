<script lang="ts">
  // Paperclip's LiveRunWidget (ui/src/components/LiveRunWidget.tsx; MIT, see
  // NOTICE) for an Issue's newest queued or running Run: the Agent, the Run's
  // number, status and start, Stop, and its Transcript live. A queued Run
  // says whose desktop it waits for, since only the Hirer's desktop runs it;
  // its stream is followed already, so the claim shows as it happens.
  import { LoaderCircle, Square } from '@lucide/svelte'
  import AgentIcon from '@bakery/ui/AgentIcon.svelte'
  import RunStatus from '@bakery/ui/RunStatus.svelte'
  import { ago } from './format'
  import { href } from './router.svelte'
  import RunView from './RunView.svelte'
  import type { Run } from './runs'

  let {
    run,
    hirer,
    stopping = false,
    oncancel,
    onstatus,
  }: { run: Run; hirer: string | null; stopping?: boolean; oncancel: () => void; onstatus: (r: Run) => void } = $props()
</script>

<section class="chrome overflow-hidden rounded-xl border border-blue-500/25 bg-background/80" aria-label="Live run" data-testid="live-run" data-run={run.id}>
  <div class="border-b border-border/60 bg-blue-500/[0.04] px-4 py-3">
    <div class="text-xs font-semibold tracking-wider text-blue-700 uppercase dark:text-blue-300">Live Run</div>
  </div>
  <div class="px-4 py-4">
    <div class="mb-3 flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div class="min-w-0">
        <a href={href(`/agents/${run.agent.id}`)} class="inline-flex items-center gap-1.5 text-sm font-medium hover:underline">
          <AgentIcon icon={run.agent.icon} class="size-4" />{run.agent.name}
        </a>
        <div class="mt-2 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
          <span class="inline-flex items-center rounded-full border border-border/70 bg-background/70 px-2 py-1 font-mono">Run #{run.id}</span>
          <RunStatus status={run.status} />
          <span>{run.started_at ? `started ${ago(run.started_at)}` : `queued ${ago(run.created_at)}`}</span>
          {#if run.desktop}<span>on {run.desktop.name}</span>{/if}
        </div>
      </div>
      {#if run.can_cancel}
        <button
          type="button"
          onclick={oncancel}
          disabled={stopping}
          class="inline-flex items-center gap-1 self-start rounded-full border border-destructive/20 bg-destructive/[0.06] px-2.5 py-1 text-xs font-medium text-destructive transition-colors hover:bg-destructive/[0.12] disabled:opacity-50"
        >
          <Square class="size-2.5" fill="currentColor" />{stopping ? 'Cancelling…' : 'Cancel'}
        </button>
      {/if}
    </div>
    {#if run.status === 'queued'}
      <p class="flex items-center gap-2 text-sm text-muted-foreground" data-testid="run-waiting">
        <LoaderCircle class="size-4 animate-spin motion-reduce:animate-none" />
        Waiting for {hirer ? `${hirer}'s` : "the hirer's"} desktop
      </p>
    {/if}
    <!-- Followed while queued too, so the claim shows the moment it happens. -->
    <div class={[run.status === 'queued' && 'hidden']}>
      <RunView {run} {onstatus} class="max-h-80 pr-1" />
    </div>
  </div>
</section>
