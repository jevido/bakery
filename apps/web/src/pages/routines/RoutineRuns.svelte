<script lang="ts">
  // Paperclip's RunsSection (ui/src/components/routine-sections/
  // operate-sections.tsx; MIT, see NOTICE): the Routine's runs, newest
  // first, each with its status, source, trigger and time, and the
  // Execution Issue it made with that Issue's status, as the Recent Runs tab,
  // and the Routine revision it ran.
  import StatusBadge from '@bakery/ui/StatusBadge.svelte'
  import { ago } from '../../lib/format'
  import PageSkeleton from '../../lib/PageSkeleton.svelte'
  import { href } from '../../lib/router.svelte'
  import { routineRevisions, routineRunStatusLabel, routineRuns, type RoutineRun } from '../../lib/routines'
  import Empty from '../../lib/ui/Empty.svelte'
  import { workLabel } from '../../lib/work'

  // `version` changes after a Run, so the list asks again.
  let { id, version }: { id: number; version: number } = $props()

  let runs = $state.raw<RoutineRun[] | null>(null)
  let loadError = $state('')
  /** Each Routine revision's number by its id. */
  let numbers = $state.raw(new Map<number, number>())

  $effect(() => {
    void version
    routineRuns(id)
      .then((rs) => (runs = rs))
      .catch((e) => (loadError = e.message))
    routineRevisions(id)
      .then((rs) => (numbers = new Map(rs.map((r) => [r.id, r.revision_number]))))
      .catch(() => {})
  })
</script>

{#if loadError}
  <p class="text-sm text-destructive">{loadError}</p>
{:else if runs === null}
  <PageSkeleton />
{:else if runs.length === 0}
  <div class="py-12"><Empty title="No runs yet. Run the routine now or wait for its schedule." icon="routines" /></div>
{:else}
  <div class="rounded-lg border" role="list" aria-label="Routine runs">
    {#each runs as run (run.id)}
      <div role="listitem" class="flex flex-col gap-1 border-b px-3 py-2.5 text-sm last:border-b-0 sm:flex-row sm:items-center sm:gap-4" data-routine-run={run.id}>
        <span class="w-32"><StatusBadge status={run.status} label={routineRunStatusLabel(run.status)} /></span>
        <span class="w-20 text-xs text-muted-foreground">{workLabel(run.source)}</span>
        <span class="w-32 truncate text-xs text-muted-foreground">{run.trigger ? run.trigger.label || workLabel(run.trigger.kind) : 'Run button'}</span>
        <span class="min-w-0 flex-1 truncate text-xs">
          {#if run.issue}
            <a href={href(`/issues/${run.issue.identifier}`)} class="hover:underline"><span class="font-mono text-muted-foreground">{run.issue.identifier}</span> {run.issue.title}</a>
            <span class="ml-1 text-muted-foreground">· {workLabel(run.issue.status)}</span>
          {:else}
            <span class="text-muted-foreground">{run.failure_reason ?? 'No issue'}</span>
          {/if}
        </span>
        <span class="w-12 text-xs text-muted-foreground" data-run-revision>{run.routine_revision_id && numbers.has(run.routine_revision_id) ? `rev ${numbers.get(run.routine_revision_id)}` : ''}</span>
        <span class="w-28 text-right text-xs text-muted-foreground" title={new Date(run.triggered_at).toLocaleString()}>{ago(run.triggered_at)}</span>
      </div>
    {/each}
  </div>
{/if}
