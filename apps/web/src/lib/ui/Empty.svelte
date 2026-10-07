<script lang="ts">
  // Paperclip's EmptyState (components/EmptyState.tsx, MIT, see NOTICE) under
  // Coolify's empty props: an optional icon on a muted square, a bold title,
  // a muted message and the actions, centred. `size` only changes the
  // spacing and the icon, for empty states inside a small card.
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

  const spacing = $derived({ sm: 'py-8', base: 'py-16', lg: 'py-24' }[size])
  const iconBox = $derived(size === 'sm' ? 'mb-3 p-3' : 'mb-4 p-4')
  const iconSize = $derived(size === 'sm' ? 'size-6' : 'size-10')
</script>

<div class={['flex w-full flex-col items-center justify-center px-6 text-center', spacing, className]}>
  {#if icon}
    <div class={['bg-muted/50', iconBox]}>
      <Icon name={icon} class={[iconSize, 'text-muted-foreground/50'].join(' ')} />
    </div>
  {/if}
  <p class="mb-1.5 text-base font-semibold text-foreground">{title}</p>
  {#if description}<p class="max-w-md text-sm text-muted-foreground">{description}</p>{/if}
  {#if children}
    <div class="mt-4 flex flex-wrap items-center justify-center gap-2">{@render children()}</div>
  {/if}
</div>
