<script lang="ts">
  // Paperclip's EmptyState (components/EmptyState.tsx) under Coolify's empty
  // props: an optional icon on a muted square, a title, a description and
  // actions, centred.
  import type { Snippet } from 'svelte'
  import Icon, { type IconName } from '../Icon.svelte'

  let {
    title,
    description,
    size = 'base',
    icon,
    class: className = '',
    children,
  }: {
    title: string
    description?: string
    size?: 'sm' | 'base' | 'lg'
    icon?: IconName
    class?: string
    children?: Snippet
  } = $props()

  const spacing = $derived({ sm: 'min-h-44 py-8', base: 'min-h-80 py-16', lg: 'min-h-96 py-16' }[size])
  const iconBox = $derived({ sm: 'mb-3 p-3', base: 'mb-4 p-4', lg: 'mb-4 p-4' }[size])
  const iconSize = $derived({ sm: 'size-6', base: 'size-10', lg: 'size-12' }[size])
  const titleClass = $derived(size === 'sm' ? 'text-sm font-semibold text-foreground' : 'text-base font-semibold text-foreground')
</script>

<div class={['chrome empty-state flex w-full flex-col items-center justify-center px-6 text-center', spacing, className]}>
  {#if icon}
    <div class={['bg-muted/50', iconBox]}>
      <Icon name={icon} class={[iconSize, 'text-muted-foreground/50'].join(' ')} />
    </div>
  {/if}
  <h2 class={titleClass}>{title}</h2>
  {#if description}<p class="mt-1.5 max-w-md text-sm text-muted-foreground">{description}</p>{/if}
  {#if children}
    <div class="mt-4 flex flex-wrap items-center justify-center gap-2">{@render children()}</div>
  {/if}
</div>
