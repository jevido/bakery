<script lang="ts">
  // Coolify's empty (resources/views/components/empty.blade.php): a dashed
  // card with an optional icon badge, a title, a description and actions.
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

  const minHeight = $derived({ sm: 'min-h-44', base: 'min-h-80', lg: 'min-h-96' }[size])
  const iconBox = $derived({ sm: 'mb-3 size-10', base: 'mb-4 size-11', lg: 'mb-4 size-12' }[size])
  const iconSize = $derived({ sm: 'size-4.5', base: 'size-5', lg: 'size-6' }[size])
  const titleClass = $derived(
    {
      sm: 'text-[14px] font-semibold text-black dark:text-fg',
      base: 'text-[15px] font-semibold text-black dark:text-fg',
      lg: 'text-base font-semibold text-black dark:text-fg',
    }[size],
  )
  const descriptionClass = $derived(
    size === 'sm'
      ? 'mt-1 max-w-sm text-[12px] leading-5 text-neutral-500 dark:text-fg-dim'
      : 'mt-1 max-w-sm text-[13px] leading-5 text-neutral-500 dark:text-fg-dim',
  )
</script>

<div
  class={[
    'chrome empty-state flex w-full flex-col items-center justify-center rounded-xl border border-dashed border-neutral-300 px-6 py-10 text-center dark:border-white/[0.1]',
    minHeight,
    className,
  ]}
>
  {#if icon}
    <div
      class={[
        'flex items-center justify-center rounded-xl border border-neutral-200 bg-white text-neutral-400 shadow-sm dark:border-white/[0.08] dark:bg-white/[0.035] dark:text-fg-faint',
        iconBox,
      ]}
    >
      <Icon name={icon} class={iconSize} />
    </div>
  {/if}
  <h2 class={titleClass}>{title}</h2>
  {#if description}<p class={descriptionClass}>{description}</p>{/if}
  {#if children}
    <div class="mt-4 flex flex-wrap items-center justify-center gap-2">{@render children()}</div>
  {/if}
</div>
