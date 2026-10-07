<script lang="ts">
  // Paperclip's EntityRow (components/EntityRow.tsx, MIT, see NOTICE): one
  // row of a bordered list, with a leading tile, a title and subtitle, and
  // trailing details pinned right. With `href` the title links the row; the
  // trailing snippet sits above that link so its own buttons stay clickable.
  //
  // Left out: `identifier`, `meta`, `titlePriority` and `secondaryRow`, until
  // a page here needs them.
  import type { Snippet } from 'svelte'

  let {
    leading,
    title,
    subtitle,
    trailing,
    href,
    selected = false,
    reserveSubtitleSpace = false,
    class: className = '',
  }: {
    leading?: Snippet
    title: string
    subtitle?: string
    trailing?: Snippet
    href?: string
    selected?: boolean
    reserveSubtitleSpace?: boolean
    class?: string
  } = $props()
</script>

<div
  class={[
    'relative flex items-center gap-3 border-b border-border px-4 py-2 text-sm transition-colors last:border-b-0',
    href && 'cursor-pointer hover:bg-accent/50',
    selected && 'bg-accent/30',
    className,
  ]}
>
  {#if href}<a {href} class="absolute inset-0" aria-label="Open {title}"></a>{/if}
  {#if leading}<div class="flex shrink-0 items-center gap-2">{@render leading()}</div>{/if}
  <div class="min-w-0 flex-1">
    <div class="flex items-center gap-2">
      <span class="truncate" {title}>{title}</span>
    </div>
    {#if subtitle || reserveSubtitleSpace}
      <p class={['mt-0.5 min-h-4 truncate text-xs text-muted-foreground', !subtitle && 'invisible']} aria-hidden={!subtitle}>
        {subtitle}
      </p>
    {/if}
  </div>
  {#if trailing}<div class="relative z-10 flex shrink-0 items-center gap-2">{@render trailing()}</div>{/if}
</div>
