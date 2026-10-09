<script lang="ts">
  // One Routine: its title, Agent, Project and Schedules, with its latest
  // runs. The Routine page proper (Paperclip's RoutineDetail.tsx, with its
  // Overview, Triggers, Runs and Activity) comes with phase 45's task 07.
  import { untrack } from 'svelte'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import { ago } from '../../lib/format'
  import PageSkeleton from '../../lib/PageSkeleton.svelte'
  import { href, type RoutineSection } from '../../lib/router.svelte'
  import { getRoutine, routineRunStatusLabel, type RoutineDetail } from '../../lib/routines'
  import { describeSchedule } from '../../lib/schedule'

  let { id }: { id: number; section: RoutineSection } = $props()

  let routine = $state.raw<RoutineDetail | null>(null)
  let loadError = $state('')

  untrack(() => getRoutine(id))
    .then((r) => (routine = r))
    .catch((e) => (loadError = e.message))

  $effect(() => breadcrumb.set({ label: 'Routines', href: href('/routines') }, { label: routine?.title ?? 'Routine' }))
</script>

{#if loadError}
  <p class="text-sm text-destructive">{loadError}</p>
{:else if !routine}
  <PageSkeleton />
{:else}
  <div class="space-y-4">
    <h1 class="text-xl font-bold">{routine.title}</h1>
    <p class="text-sm text-muted-foreground">
      {routine.assignee_agent?.name ?? 'No default agent'} · {routine.project?.name ?? 'No project'} · {routine.status}
    </p>
    <ul class="space-y-1 text-sm" aria-label="Triggers">
      {#each routine.triggers as t (t.id)}
        <li>
          {t.kind === 'schedule' ? `${describeSchedule(t.cron_expression ?? '')} (${t.timezone})` : t.label || 'API'}
          {#if t.next_run_at}<span class="text-muted-foreground"> · next run {new Date(t.next_run_at).toLocaleString()}</span>{/if}
        </li>
      {/each}
    </ul>
    <ul class="space-y-1 text-sm" aria-label="Runs">
      {#each routine.recent_runs as run (run.id)}
        <li>
          {routineRunStatusLabel(run.status)} · {ago(run.triggered_at)}
          {#if run.issue}· <a class="hover:underline" href={href(`/issues/${run.issue.identifier}`)}>{run.issue.identifier}</a>{/if}
        </li>
      {/each}
    </ul>
  </div>
{/if}
