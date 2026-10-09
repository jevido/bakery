<script lang="ts">
  // Paperclip's Create routine form (the composer in ui/src/pages/Routines.tsx;
  // MIT, see NOTICE) on the dashboard's Modal, shaped as NewIssueDialog: a
  // title, a Markdown description, chips for the Agent, Project, Goal and
  // Priority, and the two policies with their help. The Bakery adds an
  // optional Schedule, added as a schedule Routine trigger right after the
  // Routine is created; Paperclip adds it on the Routine page instead.
  import { Bot, FolderOpen, Target } from '@lucide/svelte'
  import * as Select from '@bakery/ui/components/ui/select'
  import { Switch } from '@bakery/ui/components/ui/switch'
  import { listAgents, type Agent } from './agents'
  import { api, ApiError } from './api'
  import MarkdownField from './MarkdownField.svelte'
  import OptionPopover from './OptionPopover.svelte'
  import PriorityIcon from './PriorityIcon.svelte'
  import { go } from './router.svelte'
  import {
    addTrigger,
    catchUpPolicies,
    concurrencyPolicies,
    createRoutine,
    policyHelp,
    routineRunStatusLabel,
    type CatchUpPolicy,
    type ConcurrencyPolicy,
  } from './routines'
  import { localTimeZone } from './schedule'
  import ScheduleEditor from './ScheduleEditor.svelte'
  import Button from './ui/Button.svelte'
  import Modal from './ui/Modal.svelte'
  import { toast } from './ui/toast.svelte'
  import { listGoals, priorities, workLabel, type Goal, type Priority } from './work'

  let { open = $bindable(false) }: { open?: boolean } = $props()

  let title = $state('')
  let description = $state('')
  let agent = $state<number | null>(null)
  let project = $state<number | null>(null)
  let goal = $state<number | null>(null)
  let priority = $state<Priority>('medium')
  let concurrency = $state<ConcurrencyPolicy>('coalesce_if_active')
  let catchUp = $state<CatchUpPolicy>('skip_missed')
  let scheduled = $state(false)
  let cron = $state('0 10 * * *')
  let timezone = $state(localTimeZone())
  let cronValid = $state(true)
  let agents = $state.raw<Agent[]>([])
  let projects = $state.raw<{ id: number; name: string }[]>([])
  let goals = $state.raw<Goal[]>([])
  let saving = $state(false)
  let error = $state('')

  $effect(() => {
    if (!open) return
    api<{ projects: { id: number; name: string }[] }>('GET', '/projects').then((r) => (projects = r.projects)).catch(() => {})
    listAgents().then((as) => (agents = as)).catch(() => {})
    listGoals().then((gs) => (goals = gs)).catch(() => {})
  })

  const agentName = $derived(agents.find((a) => a.id === agent)?.name)
  const projectName = $derived(projects.find((p) => p.id === project)?.name)
  const goalTitle = $derived(goals.find((g) => g.id === goal)?.title)
  const policyLabel = (p: string) => workLabel(routineRunStatusLabel(p))

  function reset() {
    title = ''
    description = ''
    agent = project = goal = null
    priority = 'medium'
    concurrency = 'coalesce_if_active'
    catchUp = 'skip_missed'
    scheduled = false
    cron = '0 10 * * *'
    timezone = localTimeZone()
    error = ''
  }

  async function submit() {
    if (!title.trim() || saving || (scheduled && !cronValid)) return
    saving = true
    error = ''
    try {
      const routine = await createRoutine({
        title: title.trim(),
        description: description.trim(),
        assignee_agent_id: agent,
        project_id: project,
        goal_id: goal,
        priority,
        concurrency_policy: concurrency,
        catch_up_policy: catchUp,
      })
      if (scheduled) {
        try {
          await addTrigger(routine.id, { kind: 'schedule', cron_expression: cron, timezone })
        } catch (e) {
          // The Routine stands; its page is where the Schedule is fixed.
          toast.error('Schedule not added', e instanceof ApiError ? (Object.values(e.errors)[0] ?? e.message) : String(e))
        }
      }
      toast.success('Routine created', routine.title)
      open = false
      reset()
      go(`/routines/${routine.id}`)
    } catch (e) {
      error = e instanceof ApiError ? (Object.values(e.errors)[0] ?? e.message) : String(e)
    } finally {
      saving = false
    }
  }
</script>

<Modal bind:open variant="none" title="New routine" subtitle="Define the recurring work first. Default project and agent are optional for draft routines." isLarge onclose={reset}>
  <div class="space-y-3">
    <!-- svelte-ignore a11y_autofocus -->
    <input
      class="w-full border-0 bg-transparent px-0 text-lg font-semibold shadow-none outline-none placeholder:text-muted-foreground/50 focus:ring-0"
      placeholder="Routine title"
      aria-label="Routine title"
      bind:value={title}
      autofocus
      onkeydown={(e) => {
        if (e.key === 'Enter') {
          e.preventDefault()
          submit()
        }
      }}
    />
    <MarkdownField bind:value={description} placeholder="Add instructions..." onsubmit={submit} />
    <div class="flex flex-wrap items-center gap-1.5 border-t pt-3">
      <OptionPopover
        chip
        label="Agent"
        value={agent}
        options={[{ value: null, label: 'No default agent' }, ...agents.map((a) => ({ value: a.id, label: a.name }))]}
        onpick={(v) => (agent = v)}
      >
        <Bot class="size-3 text-muted-foreground" />
        {agentName ?? 'Agent'}
      </OptionPopover>
      <OptionPopover
        chip
        label="Project"
        value={project}
        options={[{ value: null, label: 'No project' }, ...projects.map((p) => ({ value: p.id, label: p.name }))]}
        onpick={(v) => (project = v)}
      >
        <FolderOpen class="size-3 text-muted-foreground" />
        {projectName ?? 'Project'}
      </OptionPopover>
      <OptionPopover
        chip
        label="Goal"
        value={goal}
        options={[{ value: null, label: 'No goal' }, ...goals.map((g) => ({ value: g.id, label: g.title }))]}
        onpick={(v) => (goal = v)}
      >
        <Target class="size-3 text-muted-foreground" />
        {goalTitle ?? 'Goal'}
      </OptionPopover>
      <OptionPopover chip label="Priority" value={priority} options={priorities.map((p) => ({ value: p, label: workLabel(p) }))} onpick={(v) => (priority = v)}>
        <PriorityIcon {priority} />
        {workLabel(priority)}
      </OptionPopover>
    </div>
    <div class="grid gap-4 border-t pt-3 sm:grid-cols-2">
      <div class="space-y-2">
        <p class="text-xs font-medium tracking-wide text-muted-foreground uppercase">Concurrency</p>
        <Select.Root type="single" value={concurrency} onValueChange={(v) => (concurrency = v as ConcurrencyPolicy)}>
          <Select.Trigger class="w-full" aria-label="Concurrency policy">{policyLabel(concurrency)}</Select.Trigger>
          <Select.Content>
            {#each concurrencyPolicies as p (p)}
              <Select.Item value={p} label={policyLabel(p)} />
            {/each}
          </Select.Content>
        </Select.Root>
        <p class="text-xs text-muted-foreground">{policyHelp[concurrency]}</p>
      </div>
      <div class="space-y-2">
        <p class="text-xs font-medium tracking-wide text-muted-foreground uppercase">Catch-up</p>
        <Select.Root type="single" value={catchUp} onValueChange={(v) => (catchUp = v as CatchUpPolicy)}>
          <Select.Trigger class="w-full" aria-label="Catch-up policy">{policyLabel(catchUp)}</Select.Trigger>
          <Select.Content>
            {#each catchUpPolicies as p (p)}
              <Select.Item value={p} label={policyLabel(p)} />
            {/each}
          </Select.Content>
        </Select.Root>
        <p class="text-xs text-muted-foreground">{policyHelp[catchUp]}</p>
      </div>
    </div>
    <div class="space-y-3 border-t pt-3">
      <label class="flex items-center justify-between gap-3">
        <span>
          <span class="block text-xs font-medium tracking-wide text-muted-foreground uppercase">Schedule</span>
          <span class="block text-xs text-muted-foreground">Run it by itself on a cron schedule. Run now always works.</span>
        </span>
        <Switch bind:checked={scheduled} aria-label="Add a schedule" />
      </label>
      {#if scheduled}
        <ScheduleEditor bind:cron bind:timezone bind:valid={cronValid} />
      {/if}
    </div>
    {#if error}<p class="text-sm text-destructive">{error}</p>{/if}
  </div>
  {#snippet footer()}
    <Button variant="highlighted" disabled={!title.trim() || (scheduled && !cronValid)} loading={saving} onclick={submit}>
      {saving ? 'Creating…' : 'Create routine'}
    </Button>
  {/snippet}
</Modal>
