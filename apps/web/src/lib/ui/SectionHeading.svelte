<script lang="ts">
  // Coolify's section-heading (resources/views/components/section-heading.blade.php)
  // in Paperclip's type: a small semibold title, an optional muted subtitle
  // and a right-aligned action, either "View all" to `href` (Paperclip's
  // small outline button) or the `actions` snippet.
  import type { Snippet } from 'svelte'
  import { Button as UiButton } from '$lib/components/ui/button'
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
    <h2 class="min-w-0 truncate text-sm leading-5 font-semibold text-foreground">{title}</h2>
    {#if actions}
      <div class="shrink-0">{@render actions()}</div>
    {:else if href}
      <UiButton {href} variant="outline" size="sm" class="group">
        {actionLabel}
        {#if icon}
          <Icon name={icon} class="size-3 opacity-70 transition-transform duration-150 ease-out group-hover:translate-x-0.5" />
        {/if}
      </UiButton>
    {/if}
  </div>
  {#if subtitle}
    <p class="mt-0.5 text-xs text-muted-foreground">{subtitle}</p>
  {/if}
</div>
