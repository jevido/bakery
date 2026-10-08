<script lang="ts">
  // Paperclip's StatusIcon over its StatusGlyph (ui/src/components/
  // StatusIcon.tsx and StatusGlyph.tsx; MIT, see NOTICE): one shape per Issue
  // status, in theme.css's --status-task-icon-* hues. With onchange it is
  // also the status picker; without it, only the glyph (and its label).
  import { Ban, Circle, CircleCheck, CircleDashed, CircleDot, CircleMinus } from '@lucide/svelte'
  import type { Component } from 'svelte'
  import * as Popover from '@bakery/ui/components/ui/popover'
  import { issueStatuses, workLabel, type IssueStatus } from './work'

  let {
    status,
    size = 'md',
    showLabel = false,
    onchange,
    class: className = '',
  }: {
    status: IssueStatus
    size?: 'sm' | 'md' | 'lg'
    showLabel?: boolean
    onchange?: (status: IssueStatus) => void
    class?: string
  } = $props()

  const px = { sm: 14, md: 16, lg: 20 }
  const icons: Partial<Record<IssueStatus, Component<{ size?: number; class?: string; style?: string }>>> = {
    backlog: CircleDashed,
    todo: Circle,
    in_review: CircleDot,
    done: CircleCheck,
    blocked: CircleMinus,
    cancelled: Ban,
  }

  let open = $state(false)
</script>

{#snippet glyph(s: IssueStatus, title?: string)}
  {@const Icon = icons[s]}
  {@const style = `color: var(--status-task-icon-${s})`}
  <span class="inline-flex shrink-0" role={title ? 'img' : undefined} aria-label={title} aria-hidden={title ? undefined : true} title={title}>
    {#if Icon}
      <Icon size={px[size]} class={['inline-block shrink-0 align-middle', className].join(' ')} {style} />
    {:else}
      <!-- in_progress: Lucide's spinner arc on the same 10-unit circle as the others. -->
      <svg
        width={px[size]}
        height={px[size]}
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        class={['inline-block shrink-0 align-middle motion-safe:animate-spin', className].join(' ')}
        {style}
      >
        <circle cx="12" cy="12" r="10" pathLength="100" stroke-dasharray="80 20" />
      </svg>
    {/if}
  </span>
{/snippet}

{#if !onchange}
  {#if showLabel}
    <span class="inline-flex items-center gap-1.5">{@render glyph(status, workLabel(status))}<span class="text-sm">{workLabel(status)}</span></span>
  {:else}
    {@render glyph(status, workLabel(status))}
  {/if}
{:else}
  <Popover.Root bind:open>
    <Popover.Trigger
      aria-label="Change status (current: {workLabel(status)})"
      class={showLabel
        ? '-mx-1 inline-flex min-h-5 cursor-pointer items-center gap-1.5 rounded px-1 py-0.5 transition-colors hover:bg-accent/50'
        : 'inline-flex cursor-pointer items-center justify-center rounded-sm focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none'}
    >
      {@render glyph(status)}
      {#if showLabel}<span class="text-sm">{workLabel(status)}</span>{/if}
    </Popover.Trigger>
    <Popover.Content align="start" class="w-40 p-1">
      <div role="listbox" aria-label="Status">
        {#each issueStatuses as s (s)}
          <button
            type="button"
            role="option"
            aria-selected={s === status}
            class={['flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left text-xs hover:bg-accent/50', s === status && 'bg-accent']}
            onclick={() => {
              open = false
              onchange(s)
            }}
          >
            {@render glyph(s)}
            {workLabel(s)}
          </button>
        {/each}
      </div>
    </Popover.Content>
  </Popover.Root>
{/if}
