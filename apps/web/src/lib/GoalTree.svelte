<script lang="ts">
  // Paperclip's GoalTree (ui/src/components/GoalTree.tsx; MIT, see NOTICE):
  // the Goals in one bordered box, each under its parent and indented by
  // depth, a Goal with Sub-goals folding open and shut. A Goal whose parent
  // is not among `goals` is a root, so a Goal's page can pass its Sub-goals.
  import { ChevronRight } from '@lucide/svelte'
  import { href } from './router.svelte'
  import StatusBadge from '@bakery/ui/StatusBadge.svelte'
  import type { Goal } from './work'

  let { goals }: { goals: Goal[] } = $props()

  const ids = $derived(new Set(goals.map((g) => g.id)))
  const roots = $derived(goals.filter((g) => g.parent_id === null || !ids.has(g.parent_id)))
  const childrenOf = (id: number) => goals.filter((g) => g.parent_id === id)

  // Folded Goals, by id; every Goal starts open as in Paperclip.
  let folded = $state<Record<number, boolean>>({})
</script>

{#snippet node(goal: Goal, depth: number)}
  {@const children = childrenOf(goal.id)}
  <div>
    <a
      href={href(`/goals/${goal.id}`)}
      class="flex items-center gap-2 px-3 py-1.5 text-sm text-inherit no-underline transition-colors hover:bg-accent/50"
      style:padding-left="{depth * 16 + 12}px"
      data-goal={goal.title}
    >
      {#if children.length > 0}
        <button
          type="button"
          class="rounded-sm border-0 bg-transparent p-0.5 text-muted-foreground hover:bg-accent"
          aria-label="{goal.title} subtree"
          aria-expanded={!folded[goal.id]}
          onclick={(e) => {
            e.preventDefault()
            e.stopPropagation()
            folded[goal.id] = !folded[goal.id]
          }}
        >
          <ChevronRight class={['size-3 transition-transform', !folded[goal.id] && 'rotate-90']} />
        </button>
      {:else}
        <span class="w-4"></span>
      {/if}
      <span class="text-xs text-muted-foreground capitalize">{goal.level}</span>
      <span class="flex-1 truncate">{goal.title}</span>
      <StatusBadge status={goal.status} />
    </a>
    {#if children.length > 0 && !folded[goal.id]}
      {#each children as child (child.id)}
        {@render node(child, depth + 1)}
      {/each}
    {/if}
  </div>
{/snippet}

{#if goals.length === 0}
  <p class="text-sm text-muted-foreground">No goals.</p>
{:else}
  <div class="border py-1">
    {#each roots as goal (goal.id)}
      {@render node(goal, 0)}
    {/each}
  </div>
{/if}
