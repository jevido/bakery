<script lang="ts" module>
  import type { RoutineInput } from '../../lib/routines'

  /** The Overview's edit form: every field but the title, which the header edits. */
  export type RoutineDraft = Omit<RoutineInput, 'status'>
</script>

<script lang="ts">
  // Paperclip's OverviewSection (ui/src/components/routine-sections/
  // editable-sections.tsx; MIT, see NOTICE), the Overview being edited:
  // the Markdown description, chips for the default Agent, Project, Goal,
  // parent Issue and Priority, and the two policies with their help, as
  // the Create routine dialog has them, and the Routine variables under the
  // description. Its save bar is the page's.
  import { Bot, CornerLeftUp, FolderOpen, Target } from '@lucide/svelte'
  import * as Select from '@bakery/ui/components/ui/select'
  import { listAgents, type Agent } from '../../lib/agents'
  import { api } from '../../lib/api'
  import MarkdownField from '../../lib/MarkdownField.svelte'
  import OptionPopover from '../../lib/OptionPopover.svelte'
  import PriorityIcon from '../../lib/PriorityIcon.svelte'
  import RoutineVariables from './RoutineVariables.svelte'
  import { catchUpPolicies, concurrencyPolicies, policyHelp, type CatchUpPolicy, type ConcurrencyPolicy } from '../../lib/routines'
  import { listGoals, listIssues, priorities, workLabel, type Goal, type Issue } from '../../lib/work'

  let {
    draft = $bindable(),
    title,
    errors = {},
  }: { draft: RoutineDraft; title: string; errors?: Record<string, string> } = $props()

  let agents = $state.raw<Agent[]>([])
  let projects = $state.raw<{ id: number; name: string }[]>([])
  let goals = $state.raw<Goal[]>([])
  let issues = $state.raw<Issue[]>([])

  api<{ projects: { id: number; name: string }[] }>('GET', '/projects').then((r) => (projects = r.projects)).catch(() => {})
  listAgents().then((as) => (agents = as.filter((a) => a.status !== 'terminated'))).catch(() => {})
  listGoals().then((gs) => (goals = gs)).catch(() => {})
  listIssues().then((is) => (issues = is)).catch(() => {})

  const agentName = $derived(agents.find((a) => a.id === draft.assignee_agent_id)?.name)
  const projectName = $derived(projects.find((p) => p.id === draft.project_id)?.name)
  const goalTitle = $derived(goals.find((g) => g.id === draft.goal_id)?.title)
  const parent = $derived(issues.find((i) => i.id === draft.parent_issue_id))
</script>

<div class="space-y-4" data-routine-overview-mode="edit">
  <MarkdownField bind:value={draft.description} label="Description" placeholder="Add instructions..." />
  <RoutineVariables {title} description={draft.description} bind:variables={draft.variables} {errors} />
  <div class="flex flex-wrap items-center gap-1.5 border-t pt-3">
    <OptionPopover
      chip
      label="Agent"
      value={draft.assignee_agent_id}
      options={[{ value: null, label: 'No default agent' }, ...agents.map((a) => ({ value: a.id, label: a.name }))]}
      onpick={(v) => (draft.assignee_agent_id = v)}
    >
      <Bot class="size-3 text-muted-foreground" />
      {agentName ?? 'Agent'}
    </OptionPopover>
    <OptionPopover
      chip
      label="Project"
      value={draft.project_id}
      options={[{ value: null, label: 'No project' }, ...projects.map((p) => ({ value: p.id, label: p.name }))]}
      onpick={(v) => (draft.project_id = v)}
    >
      <FolderOpen class="size-3 text-muted-foreground" />
      {projectName ?? 'Project'}
    </OptionPopover>
    <OptionPopover
      chip
      label="Goal"
      value={draft.goal_id}
      options={[{ value: null, label: 'No goal' }, ...goals.map((g) => ({ value: g.id, label: g.title }))]}
      onpick={(v) => (draft.goal_id = v)}
    >
      <Target class="size-3 text-muted-foreground" />
      {goalTitle ?? 'Goal'}
    </OptionPopover>
    <OptionPopover
      chip
      label="Parent issue"
      value={draft.parent_issue_id}
      options={[{ value: null, label: 'No parent' }, ...issues.map((i) => ({ value: i.id, label: `${i.identifier} ${i.title}` }))]}
      onpick={(v) => (draft.parent_issue_id = v)}
    >
      <CornerLeftUp class="size-3 text-muted-foreground" />
      {parent ? parent.identifier : 'Parent issue'}
    </OptionPopover>
    <OptionPopover chip label="Priority" value={draft.priority} options={priorities.map((p) => ({ value: p, label: workLabel(p) }))} onpick={(v) => (draft.priority = v)}>
      <PriorityIcon priority={draft.priority} />
      {workLabel(draft.priority)}
    </OptionPopover>
  </div>
  <div class="grid gap-4 border-t pt-3 sm:grid-cols-2">
    <div class="space-y-2">
      <p class="text-xs font-medium tracking-wide text-muted-foreground uppercase">Concurrency</p>
      <Select.Root type="single" value={draft.concurrency_policy} onValueChange={(v) => (draft.concurrency_policy = v as ConcurrencyPolicy)}>
        <Select.Trigger class="w-full" aria-label="Concurrency policy">{workLabel(draft.concurrency_policy)}</Select.Trigger>
        <Select.Content>
          {#each concurrencyPolicies as p (p)}
            <Select.Item value={p} label={workLabel(p)} />
          {/each}
        </Select.Content>
      </Select.Root>
      <p class="text-xs text-muted-foreground">{policyHelp[draft.concurrency_policy]}</p>
    </div>
    <div class="space-y-2">
      <p class="text-xs font-medium tracking-wide text-muted-foreground uppercase">Catch-up</p>
      <Select.Root type="single" value={draft.catch_up_policy} onValueChange={(v) => (draft.catch_up_policy = v as CatchUpPolicy)}>
        <Select.Trigger class="w-full" aria-label="Catch-up policy">{workLabel(draft.catch_up_policy)}</Select.Trigger>
        <Select.Content>
          {#each catchUpPolicies as p (p)}
            <Select.Item value={p} label={workLabel(p)} />
          {/each}
        </Select.Content>
      </Select.Root>
      <p class="text-xs text-muted-foreground">{policyHelp[draft.catch_up_policy]}</p>
    </div>
  </div>
</div>
