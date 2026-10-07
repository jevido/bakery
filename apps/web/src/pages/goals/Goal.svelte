<script lang="ts">
  // Paperclip's GoalDetail (ui/src/pages/GoalDetail.tsx) with its
  // GoalProperties panel (ui/src/components/GoalProperties.tsx; MIT, see
  // NOTICE): the level and status over the title and description, edited in
  // place, Sub-goals and Issues in tabs, and the properties on the right.
  // Paperclip's tab of linked Projects becomes the Goal's Issues: a Project
  // here serves no Goal, an Issue does. The owner is a Member, where
  // Paperclip's is an agent. Only a Member with manage_work changes or
  // deletes anything; Delete is The Bakery's own.
  import { Plus } from '@lucide/svelte'
  import * as Tabs from '$lib/components/ui/tabs'
  import { Separator } from '$lib/components/ui/separator'
  import { api, ApiError } from '../../lib/api'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import { formatDate } from '../../lib/format'
  import GoalTree from '../../lib/GoalTree.svelte'
  import InlineEditor from '../../lib/InlineEditor.svelte'
  import NewGoalDialog from '../../lib/NewGoalDialog.svelte'
  import NewIssueDialog from '../../lib/NewIssueDialog.svelte'
  import OptionPopover from '../../lib/OptionPopover.svelte'
  import PageSkeleton from '../../lib/PageSkeleton.svelte'
  import { go, href } from '../../lib/router.svelte'
  import StatusIcon from '../../lib/StatusIcon.svelte'
  import { session, type Member } from '../../lib/session.svelte'
  import Button from '../../lib/ui/Button.svelte'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import StatusBadge from '../../lib/ui/StatusBadge.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import NotFound from '../NotFound.svelte'
  import {
    deleteGoal,
    getGoal,
    goalLevelLabels,
    goalLevels,
    goalStatuses,
    listGoals,
    updateGoal,
    type Goal,
    type GoalDetail,
    type GoalInput,
  } from '../../lib/work'

  let { id }: { id: number } = $props()

  let goal = $state.raw<GoalDetail | null>(null)
  let goals = $state.raw<Goal[]>([])
  let members = $state.raw<Member[]>([])
  let missing = $state(false)
  let loadError = $state('')
  let creating = $state(false)
  let creatingIssue = $state(false)

  const editable = $derived(session.can('manage_work'))

  function load() {
    getGoal(id)
      .then((g) => (goal = g))
      .catch((e) => {
        if (e instanceof ApiError && e.status === 404) missing = true
        else loadError = e.message
      })
    listGoals()
      .then((gs) => (goals = gs))
      .catch(() => {})
  }
  load()
  api<{ members: Member[] }>('GET', '/members')
    .then((r) => (members = r.members))
    .catch(() => {})

  $effect(() => breadcrumb.set({ label: 'Goals', href: href('/goals') }, { label: goal?.title ?? 'Goal' }))

  // Every Goal under this one, so the Sub-goals tab shows their whole tree.
  const descendants = $derived.by(() => {
    const out: Goal[] = []
    const walk = (parent: number) => {
      for (const g of goals) {
        if (g.parent_id === parent) {
          out.push(g)
          walk(g.id)
        }
      }
    }
    walk(id)
    return out
  })
  const parent = $derived(goals.find((g) => g.id === goal?.parent_id))
  // A Goal cannot move under itself or one of its own Sub-goals.
  const parentOptions = $derived([
    { value: null, label: 'No parent' },
    ...goals.filter((g) => g.id !== id && !descendants.some((d) => d.id === g.id)).map((g) => ({ value: g.id as number | null, label: g.title })),
  ])

  async function save(patch: GoalInput) {
    try {
      const updated = await updateGoal(id, patch)
      if (goal) goal = { ...goal, ...updated }
      goals = goals.map((g) => (g.id === id ? updated : g))
    } catch (e) {
      toast.error(e instanceof ApiError ? (Object.values(e.errors)[0] ?? e.message) : String(e))
    }
  }

  async function remove() {
    await deleteGoal(id)
    toast.success('Goal deleted.')
    go('/goals')
  }
</script>

{#snippet row(label: string, value: import('svelte').Snippet)}
  <div class="flex items-start gap-3 py-1.5">
    <span class="mt-0.5 w-20 shrink-0 text-xs text-muted-foreground">{label}</span>
    <div class="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">{@render value()}</div>
  </div>
{/snippet}

{#if missing}
  <NotFound title="Goal not found" description="This goal does not exist or you cannot see it." />
{:else if loadError}
  <p class="text-sm text-destructive">{loadError}</p>
{:else if goal === null}
  <PageSkeleton />
{:else}
  {@const g = goal}
  <div class="flex flex-col gap-6 lg:flex-row">
    <div class="min-w-0 flex-1 space-y-6">
      <div class="space-y-3">
        <div class="flex items-center gap-2">
          <span class="text-xs text-muted-foreground uppercase">{g.level}</span>
          <StatusBadge status={g.status} />
        </div>
        <InlineEditor label="Title" value={g.title} {editable} as="h2" class="text-xl font-bold" onsave={(title) => save({ title })} />
        <InlineEditor
          label="Description"
          value={g.description}
          {editable}
          multiline
          placeholder="Add a description..."
          class="text-sm text-muted-foreground"
          onsave={(description) => save({ description })}
        />
      </div>

      <Tabs.Root value="children">
        <Tabs.List>
          <Tabs.Trigger value="children">Sub-Goals ({descendants.filter((d) => d.parent_id === id).length})</Tabs.Trigger>
          <Tabs.Trigger value="issues">Issues ({g.issues.length})</Tabs.Trigger>
        </Tabs.List>
        <Tabs.Content value="children" class="mt-4 space-y-3">
          {#if editable}
            <div class="flex items-center justify-start">
              <Button class="h-8 px-3 text-xs" onclick={() => (creating = true)}><Plus class="size-3.5" />Sub Goal</Button>
            </div>
          {/if}
          {#if descendants.length === 0}
            <p class="text-sm text-muted-foreground">No sub-goals.</p>
          {:else}
            <GoalTree goals={descendants} />
          {/if}
        </Tabs.Content>
        <Tabs.Content value="issues" class="mt-4 space-y-3">
          {#if editable}
            <div class="flex items-center justify-start">
              <Button class="h-8 px-3 text-xs" onclick={() => (creatingIssue = true)}><Plus class="size-3.5" />New Issue</Button>
            </div>
          {/if}
          {#if g.issues.length === 0}
            <p class="text-sm text-muted-foreground">No issues.</p>
          {:else}
            <div class="border">
              {#each g.issues as issue (issue.id)}
                <a
                  href={href(`/issues/${issue.identifier}`)}
                  class="flex items-center gap-3 border-b px-4 py-2 text-sm text-inherit no-underline transition-colors last:border-b-0 hover:bg-accent/50"
                >
                  <StatusIcon status={issue.status} />
                  <span class="shrink-0 font-mono text-xs text-muted-foreground">{issue.identifier}</span>
                  <span class="min-w-0 flex-1 truncate">{issue.title}</span>
                </a>
              {/each}
            </div>
          {/if}
        </Tabs.Content>
      </Tabs.Root>
    </div>

    <aside class="w-full shrink-0 space-y-4 border-t pt-4 lg:w-72 lg:border-t-0 lg:border-l lg:pt-0 lg:pl-6" aria-label="Properties">
      <div class="space-y-1">
        {#snippet status()}
          {#if editable}
            <OptionPopover align="end" label="Status" value={g.status} options={goalStatuses.map((s) => ({ value: s, label: s }))} onpick={(status) => save({ status })}>
              <StatusBadge status={g.status} />
            </OptionPopover>
          {:else}
            <StatusBadge status={g.status} />
          {/if}
        {/snippet}
        {@render row('Status', status)}
        {#snippet level()}
          {#if editable}
            <OptionPopover align="end" label="Level" value={g.level} options={goalLevels.map((l) => ({ value: l, label: goalLevelLabels[l] }))} onpick={(level) => save({ level })}>
              <span class="text-sm">{goalLevelLabels[g.level]}</span>
            </OptionPopover>
          {:else}
            <span class="text-sm">{goalLevelLabels[g.level]}</span>
          {/if}
        {/snippet}
        {@render row('Level', level)}
        {#snippet owner()}
          {#if editable}
            <OptionPopover
              align="end"
              label="Owner"
              value={g.owner?.id ?? null}
              options={[{ value: null, label: 'None' }, ...members.map((m) => ({ value: m.id as number | null, label: m.name }))]}
              onpick={(owner_id) => save({ owner_id })}
            >
              <span class={['text-sm', !g.owner && 'text-muted-foreground']}>{g.owner?.name ?? 'None'}</span>
            </OptionPopover>
          {:else}
            <span class={['text-sm', !g.owner && 'text-muted-foreground']}>{g.owner?.name ?? 'None'}</span>
          {/if}
        {/snippet}
        {@render row('Owner', owner)}
        {#snippet parentGoal()}
          {#if editable}
            <OptionPopover align="end" label="Parent goal" value={g.parent_id} options={parentOptions} onpick={(parent_id) => save({ parent_id })}>
              <span class={['text-sm', !parent && 'text-muted-foreground']}>{parent?.title ?? 'None'}</span>
            </OptionPopover>
          {:else if parent}
            <a href={href(`/goals/${parent.id}`)} class="text-sm hover:underline">{parent.title}</a>
          {:else}
            <span class="text-sm text-muted-foreground">None</span>
          {/if}
        {/snippet}
        {@render row('Parent Goal', parentGoal)}
      </div>
      <Separator />
      <div class="space-y-1">
        {#snippet created()}<span class="text-sm">{formatDate(g.created_at)}</span>{/snippet}
        {@render row('Created', created)}
        {#snippet updated()}<span class="text-sm">{formatDate(g.updated_at)}</span>{/snippet}
        {@render row('Updated', updated)}
      </div>
      {#if editable}
        <Separator />
        <ConfirmationModal
          title="Delete goal?"
          buttonTitle="Delete goal"
          variant="error"
          actions={[`The goal "${g.title}" will be permanently deleted.`, 'Its sub-goals move under its parent goal, and its issues no longer serve a goal.']}
          confirmWithText={false}
          step2ButtonText="Delete"
          onconfirm={remove}
        />
      {/if}
    </aside>
  </div>
{/if}

<NewGoalDialog bind:open={creating} parentId={id} oncreated={load} />
<NewIssueDialog bind:open={creatingIssue} goalId={id} oncreated={load} />
