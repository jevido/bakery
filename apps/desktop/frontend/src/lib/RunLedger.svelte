<script lang="ts">
  // An Agent's Runs, as apps/web/src/lib/RunLedger.svelte lists them for the
  // dashboard's Agent page: each Run's status, start, run time and tokens,
  // newest first, each unfolding to its Transcript.
  import { ChevronDown, ChevronRight } from '@lucide/svelte'
  import { SvelteSet } from 'svelte/reactivity'
  import RunSource from '@bakery/ui/RunSource.svelte'
  import RunStatus from '@bakery/ui/RunStatus.svelte'
  import { compactCount, runDuration } from '@bakery/ui/runTranscript'
  import RunView from './RunView.svelte'
  import type { Run } from './desktop'

  let { address, guildID, runs }: { address: string; guildID: number; runs: Run[] } = $props()

  const open = new SvelteSet<number>()
  const toggle = (id: number) => (open.has(id) ? open.delete(id) : open.add(id))

  function runTimeMs(r: Pick<Run, 'started_at' | 'finished_at' | 'usage'>, now = Date.now()): number | null {
    if (r.usage.duration_ms > 0) return r.usage.duration_ms
    if (!r.started_at) return null
    return (r.finished_at ? Date.parse(r.finished_at) : now) - Date.parse(r.started_at)
  }

  // As Carbon's diffForHumans says it: "5 minutes ago" (apps/web/src/lib/format.ts's ago).
  function ago(iso: string): string {
    const s = Math.round((Date.now() - Date.parse(iso)) / 1000)
    if (s < 1) return 'just now'
    const units: [number, string][] = [
      [365 * 86400, 'year'],
      [30 * 86400, 'month'],
      [7 * 86400, 'week'],
      [86400, 'day'],
      [3600, 'hour'],
      [60, 'minute'],
      [1, 'second'],
    ]
    for (const [secs, unit] of units) {
      const n = Math.floor(s / secs)
      if (n >= 1) return `${n} ${unit}${n === 1 ? '' : 's'} ago`
    }
    return 'just now'
  }
</script>

{#if runs.length === 0}
  <div class="rounded-md border border-dashed px-3 py-3 text-sm text-muted-foreground">No runs yet.</div>
{:else}
  <div class="space-y-1.5" data-testid="run-ledger">
    {#each runs as r (r.id)}
      {@const ms = runTimeMs(r)}
      {@const unfolded = open.has(r.id)}
      <div class="rounded-lg border border-border/60 text-xs text-muted-foreground" data-run={r.id}>
        <div class="flex flex-wrap items-center gap-2 px-3 py-2">
          <button type="button" class="inline-flex items-center gap-2 hover:text-foreground" aria-expanded={unfolded} onclick={() => toggle(r.id)}>
            {#if unfolded}<ChevronDown class="size-3.5 shrink-0" />{:else}<ChevronRight class="size-3.5 shrink-0" />{/if}
            <span class="font-medium text-foreground">Run #{r.id}</span>
          </button>
          <RunStatus status={r.status} />
          <RunSource source={r.invocation_source} reason={r.wake_reason} count={r.wake_count} />
          {#if r.issue}<span class="truncate text-foreground">{r.issue.identifier} {r.issue.title}</span>{/if}
          <span>{r.started_at ? `started ${ago(r.started_at)}` : `queued ${ago(r.created_at)}`}</span>
          <span class="ml-auto flex shrink-0 items-center gap-3 tabular-nums">
            {#if ms !== null}<span title="Run time">{runDuration(ms)}</span>{/if}
            {#if r.usage.input_tokens + r.usage.output_tokens > 0}
              <span title="Input and output tokens">{compactCount(r.usage.input_tokens + r.usage.cached_input_tokens)} / {compactCount(r.usage.output_tokens)} tokens</span>
            {/if}
          </span>
        </div>
        {#if r.error && r.status !== 'succeeded'}
          <p class="mx-3 mb-2 rounded-md border border-destructive/30 bg-destructive/10 px-2 py-1 break-words text-destructive">{r.error}</p>
        {/if}
        {#if unfolded}
          <div class="border-t border-border/40 px-3 py-3">
            {#if r.status === 'queued'}
              <p class="text-muted-foreground">Not started yet.</p>
            {:else}
              <RunView {address} {guildID} run={r} class="max-h-96 pr-1" />
            {/if}
          </div>
        {/if}
      </div>
    {/each}
  </div>
{/if}
