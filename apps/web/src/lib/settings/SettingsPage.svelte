<script lang="ts">
  // The frame of Paperclip's settings pages (ui/src/pages/CompanySettings.tsx
  // and CompanyAccess.tsx; MIT, see NOTICE): the icon and title over the
  // page's groups, with the page's actions on the right.
  import type { Component, Snippet } from 'svelte'

  let {
    icon: Icon,
    title,
    actions,
    children,
    ...rest
  }: { icon: Component<{ class?: string }>; title: string; actions?: Snippet; children: Snippet; [key: `data-${string}`]: unknown } = $props()
</script>

<div class="chrome max-w-6xl space-y-8" {...rest}>
  <div class="flex flex-wrap items-center justify-between gap-3">
    <div class="flex items-center gap-2">
      <Icon class="size-5 text-muted-foreground" />
      <h1 class="text-lg font-semibold">{title}</h1>
    </div>
    {#if actions}<div class="flex flex-wrap items-center gap-2">{@render actions()}</div>{/if}
  </div>
  {@render children()}
</div>
