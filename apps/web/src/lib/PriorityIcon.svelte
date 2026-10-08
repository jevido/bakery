<script lang="ts">
  // Paperclip's PriorityIcon (ui/src/components/PriorityIcon.tsx, with
  // priorityColor from ui/src/lib/status-colors.ts; MIT, see NOTICE): an
  // arrow per Priority, critical a warning sign. With onchange it is also the
  // priority picker.
  import { AlertTriangle, ArrowDown, ArrowUp, Minus } from '@lucide/svelte'
  import * as Popover from '@bakery/ui/components/ui/popover'
  import { priorities, workLabel, type Priority } from './work'

  let {
    priority,
    showLabel = false,
    onchange,
    class: className = '',
  }: { priority: Priority; showLabel?: boolean; onchange?: (priority: Priority) => void; class?: string } = $props()

  const config = {
    critical: { icon: AlertTriangle, color: 'text-red-600 dark:text-red-400' },
    high: { icon: ArrowUp, color: 'text-orange-600 dark:text-orange-400' },
    medium: { icon: Minus, color: 'text-yellow-600 dark:text-yellow-400' },
    low: { icon: ArrowDown, color: 'text-blue-600 dark:text-blue-400' },
  }

  let open = $state(false)
</script>

{#snippet icon(p: Priority, title?: string)}
  {@const Icon = config[p].icon}
  <span
    class={['inline-flex shrink-0 items-center justify-center', config[p].color, className]}
    role={title ? 'img' : undefined}
    aria-label={title}
    aria-hidden={title ? undefined : true}
    {title}
  >
    <Icon class="size-3.5" />
  </span>
{/snippet}

{#if !onchange}
  {#if showLabel}
    <span class="inline-flex items-center gap-1.5">{@render icon(priority)}<span class="text-sm">{workLabel(priority)}</span></span>
  {:else}
    {@render icon(priority, `${workLabel(priority)} priority`)}
  {/if}
{:else}
  <Popover.Root bind:open>
    <Popover.Trigger
      aria-label="Change priority (current: {workLabel(priority)})"
      class={showLabel
        ? '-mx-1 inline-flex min-h-5 cursor-pointer items-center gap-1.5 rounded px-1 py-0.5 transition-colors hover:bg-accent/50'
        : 'inline-flex cursor-pointer items-center justify-center rounded-sm focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none'}
    >
      {@render icon(priority)}
      {#if showLabel}<span class="text-sm">{workLabel(priority)}</span>{/if}
    </Popover.Trigger>
    <Popover.Content align="start" class="w-36 p-1">
      <div role="listbox" aria-label="Priority">
        {#each priorities as p (p)}
          <button
            type="button"
            role="option"
            aria-selected={p === priority}
            class={['flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left text-xs hover:bg-accent/50', p === priority && 'bg-accent']}
            onclick={() => {
              open = false
              onchange(p)
            }}
          >
            {@render icon(p)}
            {workLabel(p)}
          </button>
        {/each}
      </div>
    </Popover.Content>
  </Popover.Root>
{/if}
