<script lang="ts">
  // Paperclip's IssueProperties with its PropertyRow
  // (ui/src/components/issue-properties/IssueProperties.tsx and
  // primitives.tsx; MIT, see NOTICE): the Issue's status, priority,
  // Assignee, Project, Goal and parent, each a picker, then who created it
  // and when it started, completed, was cancelled and last changed. Every
  // pick is one PATCH, sent through `onsave`. Without `editable` the values
  // are only shown, a Project, Goal or parent as a link. Left out with what
  // the Issue does not have yet: labels, blockers, reviewers and approvers,
  // the execution workspace and the agent's model.
  import type { Snippet } from 'svelte'
  import { Separator } from '$lib/components/ui/separator'
  import { formatDate } from './format'
  import Identity from './Identity.svelte'
  import OptionPopover from './OptionPopover.svelte'
  import PriorityIcon from './PriorityIcon.svelte'
  import { href } from './router.svelte'
  import type { Member } from './session.svelte'
  import StatusIcon from './StatusIcon.svelte'
  import type { Goal, Issue, IssueDetail, IssueInput } from './work'

  let {
    issue,
    members,
    projects,
    goals,
    issues,
    editable,
    onsave,
  }: {
    issue: IssueDetail
    members: Member[]
    projects: { id: number; name: string }[]
    goals: Goal[]
    /** The Guild's Issues, to pick a parent from. */
    issues: Issue[]
    editable: boolean
    onsave: (patch: IssueInput) => unknown
  } = $props()

  // An Issue cannot move under itself or one of its own Sub-issues.
  const below = $derived.by(() => {
    const out = new Set<number>([issue.id])
    let grew = true
    while (grew) {
      grew = false
      for (const i of issues) {
        if (i.parent && out.has(i.parent.id) && !out.has(i.id)) {
          out.add(i.id)
          grew = true
        }
      }
    }
    return out
  })
  const none = { value: null, label: 'None' }
  const memberOptions = $derived([{ value: null, label: 'No assignee' }, ...members.map((m) => ({ value: m.id as number | null, label: m.name }))])
  const projectOptions = $derived([{ value: null, label: 'No project' }, ...projects.map((p) => ({ value: p.id as number | null, label: p.name }))])
  const goalOptions = $derived([{ value: null, label: 'No goal' }, ...goals.map((g) => ({ value: g.id as number | null, label: g.title }))])
  const parentOptions = $derived([
    { ...none, label: 'No parent' },
    ...issues.filter((i) => !below.has(i.id)).map((i) => ({ value: i.id as number | null, label: `${i.identifier} ${i.title}` })),
  ])
</script>

{#snippet row(label: string, value: Snippet)}
  <div class="flex w-full min-w-0 items-center gap-3 py-1" data-property-row={label}>
    <span class="w-24 shrink-0 truncate text-xs text-muted-foreground" title={label}>{label}</span>
    <div class="flex min-w-0 flex-1 items-center gap-1.5">{@render value()}</div>
  </div>
{/snippet}

{#snippet muted(text: string)}<span class="text-sm text-muted-foreground">{text}</span>{/snippet}

<div class="space-y-4">
  <div class="space-y-1">
    {#snippet status()}
      <StatusIcon status={issue.status} showLabel onchange={editable ? (status) => onsave({ status }) : undefined} />
    {/snippet}
    {@render row('Status', status)}
    {#snippet priority()}
      <PriorityIcon priority={issue.priority} showLabel onchange={editable ? (priority) => onsave({ priority }) : undefined} />
    {/snippet}
    {@render row('Priority', priority)}
    {#snippet assignee()}
      {#if editable}
        <OptionPopover align="end" label="Assignee" value={issue.assignee?.id ?? null} options={memberOptions} onpick={(assignee_id) => onsave({ assignee_id })}>
          {#if issue.assignee}<Identity name={issue.assignee.name} size="sm" />{:else}{@render muted('No assignee')}{/if}
        </OptionPopover>
      {:else if issue.assignee}
        <Identity name={issue.assignee.name} size="sm" />
      {:else}
        {@render muted('No assignee')}
      {/if}
    {/snippet}
    {@render row('Assignee', assignee)}
    {#snippet project()}
      {#if editable}
        <OptionPopover align="end" label="Project" value={issue.project?.id ?? null} options={projectOptions} onpick={(project_id) => onsave({ project_id })}>
          {#if issue.project}<span class="truncate text-sm">{issue.project.name}</span>{:else}{@render muted('No project')}{/if}
        </OptionPopover>
      {:else if issue.project}
        <a href={href(`/project/${issue.project.id}`)} class="truncate text-sm hover:underline">{issue.project.name}</a>
      {:else}
        {@render muted('No project')}
      {/if}
    {/snippet}
    {@render row('Project', project)}
    {#snippet goal()}
      {#if editable}
        <OptionPopover align="end" label="Goal" value={issue.goal?.id ?? null} options={goalOptions} onpick={(goal_id) => onsave({ goal_id })}>
          {#if issue.goal}<span class="truncate text-sm">{issue.goal.title}</span>{:else}{@render muted('No goal')}{/if}
        </OptionPopover>
      {:else if issue.goal}
        <a href={href(`/goals/${issue.goal.id}`)} class="truncate text-sm hover:underline">{issue.goal.title}</a>
      {:else}
        {@render muted('No goal')}
      {/if}
    {/snippet}
    {@render row('Goal', goal)}
    {#snippet parent()}
      {#if editable}
        <OptionPopover align="end" label="Parent" value={issue.parent?.id ?? null} options={parentOptions} onpick={(parent_id) => onsave({ parent_id })}>
          {#if issue.parent}<span class="truncate text-sm">{issue.parent.identifier} {issue.parent.title}</span>{:else}{@render muted('No parent')}{/if}
        </OptionPopover>
      {:else if issue.parent}
        <a href={href(`/issues/${issue.parent.identifier}`)} class="truncate text-sm hover:underline">{issue.parent.identifier} {issue.parent.title}</a>
      {:else}
        {@render muted('No parent')}
      {/if}
    {/snippet}
    {@render row('Parent', parent)}
  </div>
  <Separator />
  <div class="space-y-1">
    {#snippet createdBy()}
      {#if issue.created_by}<Identity name={issue.created_by.name} size="sm" />{:else}{@render muted('Unknown')}{/if}
    {/snippet}
    {@render row('Created by', createdBy)}
    {#if issue.started_at}
      {#snippet started()}<span class="text-sm">{formatDate(issue.started_at!)}</span>{/snippet}
      {@render row('Started', started)}
    {/if}
    {#if issue.completed_at}
      {#snippet completed()}<span class="text-sm">{formatDate(issue.completed_at!)}</span>{/snippet}
      {@render row('Completed', completed)}
    {/if}
    {#if issue.cancelled_at}
      {#snippet cancelled()}<span class="text-sm">{formatDate(issue.cancelled_at!)}</span>{/snippet}
      {@render row('Cancelled', cancelled)}
    {/if}
    {#snippet created()}<span class="text-sm">{formatDate(issue.created_at)}</span>{/snippet}
    {@render row('Created', created)}
    {#snippet updated()}<span class="text-sm">{formatDate(issue.updated_at)}</span>{/snippet}
    {@render row('Updated', updated)}
  </div>
</div>
