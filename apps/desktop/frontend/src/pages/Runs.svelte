<script lang="ts">
  // "Runs on this desktop": every Run the Runner executes right now, across
  // every connected Bakery, each with its Transcript growing live. Read
  // straight from the Runner (localRuns): no round trip to a Bakery.
  import { ListChecks } from '@lucide/svelte'
  import AgentIcon from '@bakery/ui/AgentIcon.svelte'
  import RunSource from '@bakery/ui/RunSource.svelte'
  import RunStatus from '@bakery/ui/RunStatus.svelte'
  import RunTranscript from '@bakery/ui/RunTranscript.svelte'
  import { localRuns, onEvent, type LocalRun, type RunsEvent } from '../lib/desktop'

  let runs = $state.raw<LocalRun[] | null>(null)
  let loadError = $state('')

  function load() {
    localRuns().then(
      (r) => (runs = r),
      (e: Error) => (loadError = e.message),
    )
  }
  $effect(load)
  $effect(() => onEvent<RunsEvent>('runs', load))
</script>

<div class="mx-auto max-w-5xl space-y-4">
  {#if loadError}
    <p class="text-sm text-destructive" role="alert">{loadError}</p>
  {:else if runs === null}
    <p class="py-8 text-center text-sm text-muted-foreground">Loading…</p>
  {:else if runs.length === 0}
    <div class="flex flex-col items-center justify-center py-16 text-center" data-testid="no-local-runs">
      <div class="mb-4 rounded-md bg-muted/50 p-4"><ListChecks class="size-10 text-muted-foreground/50" /></div>
      <p class="text-sm text-muted-foreground">No Runs on this desktop right now.</p>
    </div>
  {:else}
    <div class="space-y-3" data-testid="local-runs">
      {#each runs as r (`${r.address} ${r.run_id}`)}
        <div class="rounded-lg border border-border p-4" data-run={r.run_id}>
          <div class="mb-3 flex flex-wrap items-center gap-2 text-sm">
            <span class="flex size-7 items-center justify-center rounded-full bg-accent"><AgentIcon icon={r.agent.icon} class="size-3.5" /></span>
            <span class="font-medium">{r.agent.name}</span>
            <RunStatus status={r.status} />
            <RunSource source={r.invocation_source} reason={r.wake_reason} count={r.wake_count} />
            <span class="text-xs text-muted-foreground">{r.guild.name}</span>
            {#if r.issue}
              <span class="min-w-0 truncate text-xs text-muted-foreground"><span class="font-mono">{r.issue.identifier}</span> {r.issue.title}</span>
            {/if}
          </div>
          <RunTranscript events={r.events ?? []} live class="max-h-96 pr-1" />
        </div>
      {/each}
    </div>
  {/if}
</div>
