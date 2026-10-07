<script lang="ts" module>
  export type BannerTone = 'info' | 'warning' | 'danger'
</script>

<script lang="ts">
  // Paperclip's InlineBanner (ui/src/components/InlineBanner.tsx, its tones
  // from lib/status-colors.ts brandBanner; MIT, see NOTICE): a full-width
  // notice with an icon, a title, a body and its actions on the right.
  import { CircleAlert, Info, TriangleAlert } from '@lucide/svelte'
  import type { Snippet } from 'svelte'
  import { cn } from '../utils'

  let {
    tone = 'info',
    title,
    children,
    actions,
    compact = false,
    class: className,
    ...rest
  }: {
    tone?: BannerTone
    title?: string
    children?: Snippet
    actions?: Snippet
    compact?: boolean
    class?: string
    [key: `data-${string}`]: string
  } = $props()

  const tones: Record<BannerTone, string> = {
    info: 'border-blue-600/40 bg-blue-100/50 text-blue-700 dark:border-blue-600/35 dark:bg-blue-600/8 dark:text-blue-300',
    warning: 'border-amber-500/50 bg-amber-100/60 text-amber-700 dark:border-amber-500/35 dark:bg-amber-500/7 dark:text-amber-500',
    danger: 'border-destructive/40 bg-red-100 text-red-700 dark:bg-red-900/50 dark:text-red-300',
  }
  const Icon = $derived(tone === 'warning' ? TriangleAlert : tone === 'danger' ? CircleAlert : Info)
</script>

<div
  role="note"
  class={cn('flex flex-col gap-2 rounded-lg border sm:flex-row sm:items-start sm:justify-between', compact ? 'p-3' : 'p-4', tones[tone], className)}
  {...rest}
>
  <div class="flex items-start gap-2.5">
    <Icon class="mt-0.5 size-4 shrink-0" aria-hidden="true" />
    <div class="space-y-1 text-sm">
      {#if title}<p class="leading-tight font-medium">{title}</p>{/if}
      {#if children}<div class="leading-snug opacity-90">{@render children()}</div>{/if}
    </div>
  </div>
  {#if actions}
    <div class="flex shrink-0 flex-wrap items-center gap-2 pl-6 sm:pl-0">{@render actions()}</div>
  {/if}
</div>
