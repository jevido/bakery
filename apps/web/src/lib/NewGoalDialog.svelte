<script lang="ts">
  // Paperclip's NewGoalDialog (ui/src/components/NewGoalDialog.tsx; MIT, see
  // NOTICE) on the dashboard's Modal: a title, a Markdown description, and
  // chips for status, level and parent Goal. Ctrl/⌘+Enter creates it.
  // Left out: the expand toggle and image uploads, which The Bakery has no
  // assets for yet.
  import { Layers, Target } from '@lucide/svelte'
  import { ApiError } from './api'
  import MarkdownField from './MarkdownField.svelte'
  import OptionPopover from './OptionPopover.svelte'
  import Button from './ui/Button.svelte'
  import Modal from './ui/Modal.svelte'
  import StatusBadge from '@bakery/ui/StatusBadge.svelte'
  import { toast } from './ui/toast.svelte'
  import { createGoal, goalLevelLabels, goalLevels, goalStatuses, listGoals, type Goal, type GoalLevel, type GoalStatus } from './work'

  let {
    open = $bindable(false),
    parentId = null,
    oncreated,
  }: {
    open?: boolean
    /** The parent a New sub-goal starts under. */
    parentId?: number | null
    oncreated?: (goal: Goal) => void
  } = $props()

  let title = $state('')
  let description = $state('')
  let status = $state<GoalStatus>('planned')
  let level = $state<GoalLevel>('task')
  // undefined until picked, so the dialog follows `parentId` until then.
  let pickedParent = $state<number | null | undefined>(undefined)
  let goals = $state.raw<Goal[]>([])
  let saving = $state(false)
  let error = $state('')

  const parent = $derived(pickedParent === undefined ? parentId : pickedParent)
  const parentTitle = $derived(goals.find((g) => g.id === parent)?.title)

  $effect(() => {
    if (open) listGoals().then((gs) => (goals = gs)).catch(() => {})
  })

  function reset() {
    title = ''
    description = ''
    status = 'planned'
    level = 'task'
    pickedParent = undefined
    error = ''
  }

  async function submit() {
    if (!title.trim() || saving) return
    saving = true
    error = ''
    try {
      const goal = await createGoal({ title: title.trim(), description: description.trim(), status, level, parent_id: parent })
      toast.success(parentId ? 'Sub-goal created.' : 'Goal created.')
      open = false
      reset()
      oncreated?.(goal)
    } catch (e) {
      error = e instanceof ApiError ? (Object.values(e.errors)[0] ?? e.message) : String(e)
    } finally {
      saving = false
    }
  }
</script>

<Modal bind:open variant="none" title={parentId ? 'New sub-goal' : 'New goal'} onclose={reset}>
  <div class="space-y-3">
    <!-- svelte-ignore a11y_autofocus -->
    <input
      class="w-full border-0 bg-transparent px-0 text-lg font-semibold shadow-none outline-none placeholder:text-muted-foreground/50 focus:ring-0"
      placeholder="Goal title"
      aria-label="Goal title"
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
      <OptionPopover chip label="Status" value={status} options={goalStatuses.map((s) => ({ value: s, label: s }))} onpick={(v) => (status = v)}>
        <StatusBadge status={status} />
      </OptionPopover>
      <OptionPopover
        chip
        label="Level"
        value={level}
        options={goalLevels.map((l) => ({ value: l, label: goalLevelLabels[l] }))}
        onpick={(v) => (level = v)}
      >
        <Layers class="size-3 text-muted-foreground" />
        {goalLevelLabels[level]}
      </OptionPopover>
      <OptionPopover
        chip
        label="Parent goal"
        value={parent}
        options={[{ value: null, label: 'No parent' }, ...goals.map((g) => ({ value: g.id, label: g.title }))]}
        onpick={(v) => (pickedParent = v)}
      >
        <Target class="size-3 text-muted-foreground" />
        {parentTitle ?? 'Parent goal'}
      </OptionPopover>
    </div>
    {#if error}<p class="text-sm text-destructive">{error}</p>{/if}
  </div>
  {#snippet footer()}
    <Button variant="highlighted" disabled={!title.trim()} loading={saving} onclick={submit}>
      {saving ? 'Creating…' : parentId ? 'Create sub-goal' : 'Create goal'}
    </Button>
  {/snippet}
</Modal>
