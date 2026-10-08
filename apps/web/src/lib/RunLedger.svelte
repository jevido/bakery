<script lang="ts">
  // Paperclip's IssueRunLedger (ui/src/components/IssueRunLedger.tsx; MIT,
  // see NOTICE) trimmed to what a Run here has: each Run's status, Agent
  // (or Issue, on an Agent's page), start, run time and tokens, newest
  // first, each unfolding to its Transcript. Its liveness, child-work and
  // stop-reason summaries are left out: they come from Heartbeats.
  import { ChevronDown, ChevronRight } from '@lucide/svelte'
  import { SvelteSet } from 'svelte/reactivity'
  import AgentIcon from '@bakery/ui/AgentIcon.svelte'
  import RunStatus from '@bakery/ui/RunStatus.svelte'
  import { compactCount, runDuration } from '@bakery/ui/runTranscript'
  import { ago } from './format'
  import { href } from './router.svelte'
  import RunView from './RunView.svelte'
  import { runTime, type Run } from './runs'

  let {
    runs,
    show = 'agent',
    onstatus,
  }: { runs: Run[]; show?: 'agent' | 'issue'; onstatus?: (r: Run) => void } = $props()

  const open = new SvelteSet<number>()
  const toggle = (id: number) => (open.has(id) ? open.delete(id) : open.add(id))
</script>

{#if runs.length === 0}
  <div class="rounded-md border border-dashed px-3 py-3 text-sm text-muted-foreground">No runs yet.</div>
{:else}
  <div class="chrome space-y-1.5" data-testid="run-ledger">
    {#each runs as r (r.id)}
      {@const ms = runTime(r)}
      {@const unfolded = open.has(r.id)}
      <div class="rounded-lg border border-border/60 text-xs text-muted-foreground" data-run={r.id}>
        <div class="flex flex-wrap items-center gap-2 px-3 py-2">
          <button type="button" class="inline-flex items-center gap-2 hover:text-foreground" aria-expanded={unfolded} onclick={() => toggle(r.id)}>
            {#if unfolded}<ChevronDown class="size-3.5 shrink-0" />{:else}<ChevronRight class="size-3.5 shrink-0" />{/if}
            <span class="font-medium text-foreground">Run #{r.id}</span>
          </button>
          <RunStatus status={r.status} />
          {#if show === 'agent'}
            <span class="inline-flex items-center gap-1 text-foreground"><AgentIcon icon={r.agent.icon} class="size-3.5" />{r.agent.name}</span>
          {:else if r.issue}
            <a href={href(`/issues/${r.issue.identifier}`)} class="min-w-0 truncate text-foreground hover:underline">
              <span class="font-mono text-muted-foreground">{r.issue.identifier}</span> {r.issue.title}
            </a>
          {/if}
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
              <RunView run={r} {onstatus} class="max-h-96 pr-1" />
            {/if}
          </div>
        {/if}
      </div>
    {/each}
  </div>
{/if}
