<script lang="ts" module>
  export type CheckpointStatus = 'idle' | 'pending' | 'running' | 'success' | 'warning' | 'error'
</script>

<script lang="ts">
  // Coolify's checkpoint-item (resources/views/components/checkpoint-item.blade.php,
  // Apache-2.0, see NOTICE): one step of a process dialog with its status
  // mark, title, description and detail. `warning` is The Bakery's: a check
  // that failed without being required.
  import type { Snippet } from 'svelte'
  import Icon from '../Icon.svelte'

  let {
    title,
    description,
    status = 'idle',
    class: className = '',
    children,
    ...rest
  }: {
    title: string
    description?: string
    status?: CheckpointStatus
    class?: string
    children?: Snippet
    [key: `data-${string}`]: string
  } = $props()

  const statusClasses: Record<CheckpointStatus, string> = {
    success: 'border-emerald-500/25 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
    error: 'border-red-500/25 bg-red-500/10 text-red-600 dark:text-red-400',
    warning: 'border-warning/25 bg-warning/15 text-warning-700 dark:text-warning',
    running: 'border-primary/25 bg-primary/10 text-primary',
    pending: 'border-neutral-200 text-neutral-400 dark:border-white/[0.1] dark:text-fg-faint',
    idle: 'border-neutral-200 bg-neutral-50 text-neutral-500 dark:border-white/[0.08] dark:bg-white/[0.035] dark:text-fg-dim',
  }
</script>

<div {...rest} class={['flex min-h-14 items-center gap-3 px-4 py-3', className]}>
  <span class={['flex size-8 shrink-0 items-center justify-center rounded-lg border', statusClasses[status]]}>
    {#if status === 'success'}
      <Icon name="check-circle" class="size-4" />
    {:else if status === 'error' || status === 'warning'}
      <Icon name="alert-circle" class="size-4" />
    {:else if status === 'running'}
      <svg class="size-4 animate-spin" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" aria-hidden="true">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
        <path
          class="opacity-75"
          fill="currentColor"
          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
        ></path>
      </svg>
    {:else}
      <span class="size-1.5 rounded-full bg-current opacity-50"></span>
    {/if}
  </span>
  <span class="min-w-0">
    <span class="block text-[13px] font-semibold">{title}</span>
    {#if description}
      <span class="mt-0.5 block text-[11px] text-neutral-500 dark:text-fg-faint">{description}</span>
    {/if}
    {#if children}
      <div class="mt-0.5 text-[11px] break-words text-neutral-500 dark:text-fg-faint">{@render children()}</div>
    {/if}
  </span>
</div>
