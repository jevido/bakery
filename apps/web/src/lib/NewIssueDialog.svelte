<script lang="ts">
  // Paperclip's NewIssueDialog (ui/src/components/NewIssueDialog.tsx; MIT,
  // see NOTICE) on the dashboard's Modal: a title, a Markdown description,
  // and chips for status, priority, Assignee, Project, Goal and parent Issue.
  // Ctrl/⌘+Enter creates it. Opened from a Goal, a Project or an Issue, that
  // one is preset. Left out: drafts, image uploads, the agent model options
  // and execution workspaces, which The Bakery has no use for yet.
  import { CircleDot, FolderOpen, Target, User } from '@lucide/svelte'
  import { api, ApiError } from './api'
  import MarkdownField from './MarkdownField.svelte'
  import OptionPopover from './OptionPopover.svelte'
  import PriorityIcon from './PriorityIcon.svelte'
  import { href } from './router.svelte'
  import { session, type Member } from './session.svelte'
  import StatusIcon from './StatusIcon.svelte'
  import Button from './ui/Button.svelte'
  import Modal from './ui/Modal.svelte'
  import { toast } from './ui/toast.svelte'
  import {
    createIssue,
    issueStatuses,
    listGoals,
    listIssues,
    priorities,
    workLabel,
    type Goal,
    type Issue,
    type IssueDetail,
    type IssueStatus,
    type Priority,
  } from './work'

  let {
    open = $bindable(false),
    projectId = null,
    goalId = null,
    parentId = null,
    status: presetStatus = 'todo',
    oncreated,
  }: {
    open?: boolean
    projectId?: number | null
    goalId?: number | null
    parentId?: number | null
    status?: IssueStatus
    oncreated?: (issue: IssueDetail) => void
  } = $props()

  let title = $state('')
  let description = $state('')
  // undefined until picked, so the dialog follows its presets until then.
  let picked = $state<{ status?: IssueStatus; project?: number | null; goal?: number | null; parent?: number | null }>({})
  let priority = $state<Priority>('medium')
  let assignee = $state<number | null>(null)
  let members = $state.raw<Member[]>([])
  let projects = $state.raw<{ id: number; name: string }[]>([])
  let goals = $state.raw<Goal[]>([])
  let issues = $state.raw<Issue[]>([])
  let saving = $state(false)
  let error = $state('')

  const status = $derived(picked.status ?? presetStatus)
  const project = $derived(picked.project === undefined ? projectId : picked.project)
  const goal = $derived(picked.goal === undefined ? goalId : picked.goal)
  const parent = $derived(picked.parent === undefined ? parentId : picked.parent)
  const me = $derived(session.member?.id ?? null)

  $effect(() => {
    if (!open) return
    api<{ members: Member[] }>('GET', '/members').then((r) => (members = r.members)).catch(() => {})
    api<{ projects: { id: number; name: string }[] }>('GET', '/projects').then((r) => (projects = r.projects)).catch(() => {})
    listGoals().then((gs) => (goals = gs)).catch(() => {})
    listIssues().then((is) => (issues = is)).catch(() => {})
  })

  const assigneeName = $derived(assignee === me ? 'Me' : members.find((m) => m.id === assignee)?.name)
  const projectName = $derived(projects.find((p) => p.id === project)?.name)
  const goalTitle = $derived(goals.find((g) => g.id === goal)?.title)
  const parentIssue = $derived(issues.find((i) => i.id === parent))

  function reset() {
    title = ''
    description = ''
    picked = {}
    priority = 'medium'
    assignee = null
    error = ''
  }

  async function submit() {
    if (!title.trim() || saving) return
    saving = true
    error = ''
    try {
      const issue = await createIssue({
        title: title.trim(),
        description: description.trim(),
        status,
        priority,
        assignee_id: assignee,
        project_id: project,
        goal_id: goal,
        parent_id: parent,
      })
      toast.success(`${issue.identifier} created`, issue.title, { label: `Open ${issue.identifier}`, href: href(`/issues/${issue.identifier}`) })
      open = false
      reset()
      oncreated?.(issue)
    } catch (e) {
      error = e instanceof ApiError ? (Object.values(e.errors)[0] ?? e.message) : String(e)
    } finally {
      saving = false
    }
  }
</script>

<Modal bind:open variant="none" title={parentId ? 'New sub-issue' : 'New issue'} onclose={reset}>
  <div class="space-y-3">
    <!-- svelte-ignore a11y_autofocus -->
    <input
      class="w-full border-0 bg-transparent px-0 text-lg font-semibold shadow-none outline-none placeholder:text-muted-foreground/50 focus:ring-0"
      placeholder="Issue title"
      aria-label="Issue title"
      bind:value={title}
      autofocus
      onkeydown={(e) => {
        if (e.key === 'Enter') {
          e.preventDefault()
          submit()
        }
      }}
    />
    <MarkdownField bind:value={description} placeholder="Add description..." onsubmit={submit} />
    <div class="flex flex-wrap items-center gap-1.5 border-t pt-3">
      <OptionPopover chip label="Status" value={status} options={issueStatuses.map((s) => ({ value: s, label: workLabel(s) }))} onpick={(v) => (picked.status = v)}>
        <StatusIcon {status} size="sm" />
        {workLabel(status)}
      </OptionPopover>
      <OptionPopover chip label="Priority" value={priority} options={priorities.map((p) => ({ value: p, label: workLabel(p) }))} onpick={(v) => (priority = v)}>
        <PriorityIcon {priority} />
        {workLabel(priority)}
      </OptionPopover>
      <OptionPopover
        chip
        label="Assignee"
        value={assignee}
        options={[
          { value: null, label: 'No assignee' },
          ...(me !== null ? [{ value: me, label: 'Me' }] : []),
          ...members.filter((m) => m.id !== me).map((m) => ({ value: m.id, label: m.name })),
        ]}
        onpick={(v) => (assignee = v)}
      >
        <User class="size-3 text-muted-foreground" />
        {assigneeName ?? 'Assignee'}
      </OptionPopover>
      <OptionPopover
        chip
        label="Project"
        value={project}
        options={[{ value: null, label: 'No project' }, ...projects.map((p) => ({ value: p.id, label: p.name }))]}
        onpick={(v) => (picked.project = v)}
      >
        <FolderOpen class="size-3 text-muted-foreground" />
        {projectName ?? 'Project'}
      </OptionPopover>
      <OptionPopover
        chip
        label="Goal"
        value={goal}
        options={[{ value: null, label: 'No goal' }, ...goals.map((g) => ({ value: g.id, label: g.title }))]}
        onpick={(v) => (picked.goal = v)}
      >
        <Target class="size-3 text-muted-foreground" />
        {goalTitle ?? 'Goal'}
      </OptionPopover>
      <OptionPopover
        chip
        label="Parent issue"
        value={parent}
        options={[{ value: null, label: 'No parent' }, ...issues.map((i) => ({ value: i.id, label: `${i.identifier} ${i.title}` }))]}
        onpick={(v) => (picked.parent = v)}
      >
        <CircleDot class="size-3 text-muted-foreground" />
        {parentIssue?.identifier ?? 'Parent issue'}
      </OptionPopover>
    </div>
    {#if error}<p class="text-sm text-destructive">{error}</p>{/if}
  </div>
  {#snippet footer()}
    <Button variant="highlighted" disabled={!title.trim()} loading={saving} onclick={submit}>
      {saving ? 'Creating…' : parentId ? 'Create sub-issue' : 'Create issue'}
    </Button>
  {/snippet}
</Modal>
