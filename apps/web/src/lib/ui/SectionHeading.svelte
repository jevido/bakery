<script lang="ts">
  // Coolify's section-heading (resources/views/components/section-heading.blade.php):
  // a 14px title, an optional muted subtitle and a right-aligned action,
  // either "View all" to `href` or the `actions` snippet.
  import type { Snippet } from 'svelte'
  import Icon, { type IconName } from '../Icon.svelte'

  let {
    title,
    subtitle,
    href,
    actionLabel = 'View all',
    icon = 'arrow-right',
    class: className = '',
    actions,
  }: {
    title: string
    subtitle?: string
    href?: string
    actionLabel?: string
    icon?: IconName | null
    class?: string
    actions?: Snippet
  } = $props()
</script>

<div class={['chrome mb-3 min-w-0', className]}>
  <div class="flex items-center justify-between gap-4">
    <h2 class="min-w-0 truncate text-[14px]! leading-5! font-semibold! text-black dark:text-fg">{title}</h2>
    {#if actions}
      <div class="shrink-0">{@render actions()}</div>
    {:else if href}
      <a {href} class="button group">
        {actionLabel}
        {#if icon}
          <Icon name={icon} class="size-3 opacity-70 transition-transform duration-150 ease-out group-hover:translate-x-0.5" />
        {/if}
      </a>
    {/if}
  </div>
  {#if subtitle}
    <p class="mt-0.5 text-[11px] text-neutral-500 dark:text-fg-faint">{subtitle}</p>
  {/if}
</div>
