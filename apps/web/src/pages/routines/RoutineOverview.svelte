<script lang="ts">
  // Paperclip's RoutineOverview (ui/src/components/RoutineOverview.tsx; MIT,
  // see NOTICE), the Overview section read: four facts (State, Triggers,
  // Next run, Last run), the default Agent, the description as Markdown,
  // then the five latest Routine runs with their Execution Issues. The
  // Bakery adds the facts Paperclip keeps in its edit form: Project, Goal,
  // parent Issue, Priority and the two policies, each Schedule as a line
  // in words, and the Routine variables in a compact list.
  import { CalendarClock, Clock3, Play, Repeat } from '@lucide/svelte'
  import AgentIcon from '@bakery/ui/AgentIcon.svelte'
  import Markdown from '@bakery/ui/Markdown.svelte'
  import StatusBadge from '@bakery/ui/StatusBadge.svelte'
  import type { Snippet } from 'svelte'
  import PriorityIcon from '../../lib/PriorityIcon.svelte'
  import { href } from '../../lib/router.svelte'
  import { policyHelp, routineRunStatusLabel, routineState, summarizeSchedule, type RoutineDetail } from '../../lib/routines'
  import { describeSchedule } from '../../lib/schedule'
  import { workLabel } from '../../lib/work'

  let { routine }: { routine: RoutineDetail } = $props()

  const schedule = $derived(summarizeSchedule(routine.triggers))
  const lastRun = $derived(routine.recent_runs[0] ?? null)
  const recent = $derived(routine.recent_runs.slice(0, 5))
  const schedules = $derived(routine.triggers.filter((t) => t.kind === 'schedule'))
  const when = (at: string) => new Date(at).toLocaleString()
</script>

{#snippet fact(Icon: typeof Repeat, label: string, value: Snippet, detail: string)}
  <div class="flex min-w-0 flex-col gap-1 rounded-lg border border-border p-3" data-routine-fact={label}>
    <div class="flex items-center gap-1.5 text-xs text-muted-foreground">
      <Icon class="size-3.5" aria-hidden="true" />
      <span>{label}</span>
    </div>
    <div class="min-w-0 text-sm font-medium text-foreground">{@render value()}</div>
    <div class="min-w-0 truncate text-xs text-muted-foreground">{detail}</div>
  </div>
{/snippet}

{#snippet property(label: string, value: Snippet)}
  <div class="flex min-w-0 items-center gap-3 py-1" data-property-row={label}>
    <span class="w-32 shrink-0 text-xs text-muted-foreground">{label}</span>
    <span class="min-w-0 truncate text-sm">{@render value()}</span>
  </div>
{/snippet}

<div class="flex flex-col gap-6" data-routine-overview-mode="read">
  <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
    {#snippet state()}<StatusBadge status={routineState(routine)} />{/snippet}
    {@render fact(Repeat, 'State', state, lastRun?.status === 'issue_created' ? 'A run is active now' : 'No active run')}
    {#snippet triggers()}{schedule.label}{/snippet}
    {@render fact(CalendarClock, 'Triggers', triggers, schedule.detail)}
    {#snippet next()}{schedule.nextRunAt ? when(schedule.nextRunAt) : 'Not scheduled'}{/snippet}
    {@render fact(Clock3, 'Next run', next, schedule.nextRunAt ? 'Scheduled' : 'Add or enable a schedule')}
    {#snippet last()}
      {#if lastRun}<StatusBadge status={lastRun.status} label={routineRunStatusLabel(lastRun.status)} />{:else}No runs yet{/if}
    {/snippet}
    {@render fact(Play, 'Last run', last, lastRun ? when(lastRun.triggered_at) : 'Run manually or wait for a trigger')}
  </div>

  <section class="flex flex-col gap-2" aria-labelledby="routine-agent-heading">
    <h2 id="routine-agent-heading" class="text-sm font-semibold">Default agent</h2>
    {#if routine.assignee_agent}
      <a href={href(`/agents/${routine.assignee_agent.id}`)} class="flex w-fit items-center gap-2 text-sm font-medium hover:underline">
        <AgentIcon icon={routine.assignee_agent.icon} class="size-4 text-muted-foreground" />
        {routine.assignee_agent.name}
      </a>
    {:else}
      <p class="text-sm text-muted-foreground">No default agent. Automatic triggers remain paused.</p>
    {/if}
  </section>

  <section class="flex flex-col gap-2" aria-labelledby="routine-description-heading">
    <h2 id="routine-description-heading" class="text-sm font-semibold">Description</h2>
    {#if routine.description.trim()}
      <Markdown source={routine.description} class="text-sm" />
    {:else}
      <p class="text-sm text-muted-foreground">No description yet.</p>
    {/if}
  </section>

  <section class="flex flex-col gap-1" aria-labelledby="routine-properties-heading">
    <h2 id="routine-properties-heading" class="mb-1 text-sm font-semibold">Properties</h2>
    {#snippet project()}
      {#if routine.project}<a href={href(`/project/${routine.project.id}`)} class="hover:underline">{routine.project.name}</a>{:else}<span class="text-muted-foreground">No project</span>{/if}
    {/snippet}
    {@render property('Project', project)}
    {#snippet goal()}
      {#if routine.goal}<a href={href(`/goals/${routine.goal.id}`)} class="hover:underline">{routine.goal.title}</a>{:else}<span class="text-muted-foreground">No goal</span>{/if}
    {/snippet}
    {@render property('Goal', goal)}
    {#snippet parent()}
      {#if routine.parent_issue}
        <a href={href(`/issues/${routine.parent_issue.identifier}`)} class="hover:underline">{routine.parent_issue.identifier} {routine.parent_issue.title}</a>
      {:else}<span class="text-muted-foreground">No parent</span>{/if}
    {/snippet}
    {@render property('Parent issue', parent)}
    {#snippet priority()}<span class="inline-flex items-center gap-1.5"><PriorityIcon priority={routine.priority} />{workLabel(routine.priority)}</span>{/snippet}
    {@render property('Priority', priority)}
    {#snippet concurrency()}<span title={policyHelp[routine.concurrency_policy]}>{workLabel(routine.concurrency_policy)}</span>{/snippet}
    {@render property('Concurrency', concurrency)}
    {#snippet catchUp()}<span title={policyHelp[routine.catch_up_policy]}>{workLabel(routine.catch_up_policy)}</span>{/snippet}
    {@render property('Catch-up', catchUp)}
    {#each schedules as t (t.id)}
      {#snippet line()}
        <span class={!t.enabled ? 'text-muted-foreground line-through' : undefined}>{describeSchedule(t.cron_expression ?? '')} ({t.timezone})</span>
        {#if t.enabled && t.next_run_at}<span class="text-muted-foreground"> · Next run {when(t.next_run_at)}</span>{/if}
      {/snippet}
      {@render property(t.label || 'Schedule', line)}
    {/each}
  </section>

  {#if routine.variables.length > 0}
    <section class="flex flex-col gap-2" aria-labelledby="routine-variables-heading">
      <h2 id="routine-variables-heading" class="text-sm font-semibold">Variables</h2>
      <ul class="flex flex-col gap-1 text-sm">
        {#each routine.variables as v (v.name)}
          <li class="flex min-w-0 items-center gap-2" data-routine-variable={v.name}>
            <code class="shrink-0 rounded bg-muted px-1 py-0.5 font-mono text-xs">{`{{${v.name}}}`}</code>
            <span class="min-w-0 truncate">{v.label || v.name}</span>
            <span class="shrink-0 text-xs text-muted-foreground">
              {v.type}{v.type === 'select' ? ` (${v.options.join(', ')})` : ''}{v.default_value != null ? ` · default ${v.default_value}` : ''}{v.required ? ' · required' : ''}
            </span>
          </li>
        {/each}
      </ul>
    </section>
  {/if}

  <section class="flex flex-col gap-2" aria-labelledby="routine-recent-runs-heading">
    <div class="flex items-center justify-between gap-3">
      <h2 id="routine-recent-runs-heading" class="text-sm font-semibold">Recent runs</h2>
      <a href={href(`/routines/${routine.id}/runs`)} class="text-sm text-muted-foreground hover:text-foreground">View all runs</a>
    </div>
    {#if recent.length === 0}
      <p class="py-6 text-center text-sm text-muted-foreground">No runs yet. Run the routine now or wait for its schedule.</p>
    {:else}
      <div class="flex flex-col gap-0.5">
        {#each recent as run (run.id)}
          <div class="flex min-w-0 items-center gap-2 rounded-lg px-2 py-2 text-sm">
            <StatusBadge status={run.status} label={routineRunStatusLabel(run.status)} />
            {#if run.issue}
              <a href={href(`/issues/${run.issue.identifier}`)} class="min-w-0 flex-1 truncate hover:underline">
                <span class="font-mono text-muted-foreground">{run.issue.identifier}</span> {run.issue.title}
              </a>
            {:else}
              <span class="min-w-0 flex-1 truncate">{run.trigger?.label || run.failure_reason || 'Routine run'}</span>
            {/if}
            <span class="shrink-0 font-mono text-xs text-muted-foreground">{when(run.triggered_at)}</span>
          </div>
        {/each}
      </div>
    {/if}
    <a href={href(`/routines/${routine.id}/activity`)} class="w-fit text-sm text-primary hover:underline">View routine activity</a>
  </section>
</div>
