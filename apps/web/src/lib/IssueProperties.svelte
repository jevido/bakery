<script lang="ts">
  // Paperclip's IssueProperties with its PropertyRow
  // (ui/src/components/issue-properties/IssueProperties.tsx and
  // primitives.tsx; MIT, see NOTICE): the Issue's status, priority,
  // Assignee, Project, Goal and parent, each a picker, then who created it
  // and when it started, completed, was cancelled and last changed. While a
  // Run holds the Issue's Checkout, a row under Assignee names its Agent and
  // jumps to that Run through `onrun`. Every
  // pick is one PATCH, sent through `onsave`. Without `editable` the values
  // are only shown, a Project, Goal or parent as a link. "Blocked by" and
  // "Blocking" follow Parent, each Issue as a pill linking to it. Left out
  // with what the Issue does not have yet: labels, reviewers and approvers,
  // the execution workspace and the agent's model.
  import { X } from '@lucide/svelte'
  import type { Snippet } from 'svelte'
  import BlockerPicker from './BlockerPicker.svelte'
  import { Separator } from '@bakery/ui/components/ui/separator'
  import { formatDate } from './format'
  import Assignee from './Assignee.svelte'
  import Actor from './Actor.svelte'
  import type { Agent } from './agents'
  import OptionPopover from './OptionPopover.svelte'
  import PriorityIcon from './PriorityIcon.svelte'
  import { href } from './router.svelte'
  import type { Member } from './session.svelte'
  import StatusIcon from './StatusIcon.svelte'
  import { assigneeKey, assigneePatch, type Blocker, type Goal, type Issue, type IssueDetail, type IssueInput } from './work'

  let {
    issue,
    members,
    agents = [],
    projects,
    goals,
    issues,
    editable,
    onsave,
    onrun,
  }: {
    issue: IssueDetail
    members: Member[]
    /** The Guild's Agents that are not terminated, listed under the Members as Assignees. */
    agents?: Agent[]
    projects: { id: number; name: string }[]
    goals: Goal[]
    /** The Guild's Issues, to pick a parent from. */
    issues: Issue[]
    editable: boolean
    onsave: (patch: IssueInput) => unknown
    /** Shows the Run holding the Checkout among the Issue's Runs. */
    onrun?: (id: number) => void
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
  const blockedByIds = $derived(issue.blocked_by.map((b) => b.id))
  const none = { value: null, label: 'None' }
  const assigneeOptions = $derived([
    { value: null, label: 'No assignee' },
    ...members.map((m) => ({ value: assigneeKey({ id: m.id, kind: 'member' }), label: m.name })),
    ...agents.map((a) => ({ value: assigneeKey({ id: a.id, kind: 'agent' }), label: a.name })),
  ])
  const projectOptions = $derived([{ value: null, label: 'No project' }, ...projects.map((p) => ({ value: p.id as number | null, label: p.name }))])
  const goalOptions = $derived([{ value: null, label: 'No goal' }, ...goals.map((g) => ({ value: g.id as number | null, label: g.title }))])
  const parentOptions = $derived([
    { ...none, label: 'No parent' },
    ...issues.filter((i) => !below.has(i.id)).map((i) => ({ value: i.id as number | null, label: `${i.identifier} ${i.title}` })),
  ])
</script>

<!-- wrap, for rows of pills, lines the label up with the first one. -->
{#snippet row(label: string, value: Snippet, wrap = false)}
  <div class={['flex w-full min-w-0 gap-3 py-1', wrap ? 'items-start' : 'items-center']} data-property-row={label} data-property-label={label}>
    <span class={['w-24 shrink-0 truncate text-xs text-muted-foreground', wrap && 'mt-0.5']} title={label}>{label}</span>
    <div class="flex min-w-0 flex-1 items-center gap-1.5">{@render value()}</div>
  </div>
{/snippet}

{#snippet pill(b: Blocker, onremove?: () => void)}
  <span class="group relative inline-flex max-w-full min-w-0" data-slot="issue-reference" data-blocker={b.identifier}>
    <a
      href={href(`/issues/${b.identifier}`)}
      title={`${b.identifier}: ${b.title}`}
      class={['inline-flex max-w-full min-w-0 items-center gap-1 rounded-full border border-border px-2 py-0.5 text-xs no-underline hover:bg-accent/50', onremove && 'pr-6']}
    >
      <StatusIcon status={b.status} size="sm" />
      <span class="shrink-0">{b.identifier}</span>
      <span class="min-w-0 truncate text-muted-foreground">{b.title}</span>
    </a>
    {#if onremove}
      <button
        type="button"
        aria-label={`Remove ${b.identifier} as blocker`}
        class="absolute top-1/2 right-1 inline-flex size-4 -translate-y-1/2 items-center justify-center rounded-full text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
        onclick={onremove}
      >
        <X class="size-3" />
      </button>
    {/if}
  </span>
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
        <OptionPopover align="end" label="Assignee" value={assigneeKey(issue.assignee)} options={assigneeOptions} onpick={(key) => onsave(assigneePatch(key))}>
          {#if issue.assignee}<Assignee assignee={issue.assignee} />{:else}{@render muted('No assignee')}{/if}
        </OptionPopover>
      {:else if issue.assignee}
        <Assignee assignee={issue.assignee} />
      {:else}
        {@render muted('No assignee')}
      {/if}
    {/snippet}
    {@render row('Assignee', assignee)}
    {#if issue.checkout}
      {@const checkout = issue.checkout}
      {#snippet checkedOut()}
        <span class="inline-flex min-w-0 items-center gap-1.5 text-xs" title="Checked out {formatDate(checkout.checked_out_at)}">
          <span class="shrink-0 text-muted-foreground">Checked out by</span>
          {#if checkout.agent}<Actor member={null} agent={checkout.agent} />{/if}
          <button type="button" class="shrink-0 font-mono text-muted-foreground hover:text-foreground hover:underline" data-checkout-run={checkout.run_id} onclick={() => onrun?.(checkout.run_id)}>
            Run #{checkout.run_id}
          </button>
        </span>
      {/snippet}
      {@render row('Checkout', checkedOut)}
    {/if}
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
    {#snippet blockedBy()}
      <div class="flex min-w-0 flex-col items-start gap-1">
        {#each issue.blocked_by as b (b.id)}
          {@render pill(b, editable ? () => onsave({ blocked_by_ids: blockedByIds.filter((id) => id !== b.id) }) : undefined)}
        {/each}
        {#if editable}
          <BlockerPicker self={issue.id} chosen={blockedByIds} {issues} onchange={(blocked_by_ids) => onsave({ blocked_by_ids })} />
        {:else if issue.blocked_by.length === 0}
          {@render muted('None')}
        {/if}
      </div>
    {/snippet}
    {@render row('Blocked by', blockedBy, true)}
    {#snippet blocking()}
      <div class="flex min-w-0 flex-col items-start gap-1">
        {#each issue.blocking as b (b.id)}
          {@render pill(b)}
        {:else}
          {@render muted('None')}
        {/each}
      </div>
    {/snippet}
    {@render row('Blocking', blocking, true)}
  </div>
  <Separator />
  <div class="space-y-1">
    {#snippet createdBy()}
      {#if issue.created_by || issue.created_by_agent}<Actor member={issue.created_by} agent={issue.created_by_agent} />{:else}{@render muted('Unknown')}{/if}
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
