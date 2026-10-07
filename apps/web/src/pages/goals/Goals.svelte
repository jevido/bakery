<script lang="ts">
  // Paperclip's Goals page (ui/src/pages/Goals.tsx; MIT, see NOTICE): New
  // Goal above the Guild's Goals as a tree, or the empty state with Add
  // Goal. Only a Member with manage_work gets the buttons.
  import { Plus } from '@lucide/svelte'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import GoalTree from '../../lib/GoalTree.svelte'
  import NewGoalDialog from '../../lib/NewGoalDialog.svelte'
  import PageSkeleton from '../../lib/PageSkeleton.svelte'
  import { session } from '../../lib/session.svelte'
  import Button from '../../lib/ui/Button.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import { listGoals, type Goal } from '../../lib/work'

  let goals = $state.raw<Goal[] | null>(null)
  let loadError = $state('')
  let creating = $state(false)

  const load = () =>
    listGoals()
      .then((gs) => (goals = gs))
      .catch((e) => (loadError = e.message))
  load()

  $effect(() => breadcrumb.set({ label: 'Goals' }))
</script>

<div class="space-y-4">
  {#if loadError}<p class="text-sm text-destructive">{loadError}</p>{/if}
  {#if goals === null && !loadError}
    <PageSkeleton />
  {:else if goals && goals.length === 0}
    <Empty title="No goals yet." icon="goals">
      {#if session.can('manage_work')}
        <Button variant="highlighted" onclick={() => (creating = true)}><Plus class="size-4" />Add Goal</Button>
      {/if}
    </Empty>
  {:else if goals}
    {#if session.can('manage_work')}
      <div class="flex items-center justify-start">
        <Button class="h-8 px-3 text-xs" onclick={() => (creating = true)}><Plus class="size-3.5" />New Goal</Button>
      </div>
    {/if}
    <GoalTree {goals} />
  {/if}
</div>

<NewGoalDialog bind:open={creating} oncreated={load} />
