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
  import { statusBadgeClasses } from '@bakery/ui/statusColors'

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
    success: `border-success/25 ${statusBadgeClasses.success}`,
    error: `border-destructive/25 ${statusBadgeClasses.error}`,
    warning: `border-warning/25 ${statusBadgeClasses.warning}`,
    running: 'border-primary/25 bg-primary/10 text-primary',
    pending: 'border-border text-muted-foreground/70',
    idle: 'border-border bg-muted/50 text-muted-foreground',
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
    <span class="block text-sm font-medium">{title}</span>
    {#if description}
      <span class="mt-0.5 block text-xs text-muted-foreground">{description}</span>
    {/if}
    {#if children}
      <div class="mt-0.5 text-xs break-words text-muted-foreground">{@render children()}</div>
    {/if}
  </span>
</div>
