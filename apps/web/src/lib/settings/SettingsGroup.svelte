<script lang="ts">
  // One group of a Paperclip settings page (ui/src/pages/CompanySettings.tsx;
  // MIT, see NOTICE): a caps label over its fields, 2xl wide. The Danger
  // Zone's label is red and its body sits on a red wash. With an `id` it is a
  // section the configuration sidebar scrolls to and flashes; `actions` sit
  // to the right of the label.
  import type { Snippet } from 'svelte'

  let {
    id,
    label,
    hint,
    destructive = false,
    wide = false,
    actions,
    children,
    ...rest
  }: {
    id?: string
    label: string
    hint?: string
    destructive?: boolean
    wide?: boolean
    actions?: Snippet
    children: Snippet
    [key: `data-${string}`]: unknown
  } = $props()
</script>

<section {id} class={['scroll-mt-28 space-y-4 rounded-md', !wide && 'max-w-2xl']} {...rest}>
  <div class="flex flex-wrap items-start justify-between gap-2">
    <div class="min-w-0">
      <div class={['text-xs font-medium tracking-wide uppercase', destructive ? 'text-destructive' : 'text-muted-foreground']}>{label}</div>
      {#if hint}<p class="mt-1 text-sm text-muted-foreground">{hint}</p>{/if}
    </div>
    {#if actions}<div class="flex flex-wrap items-center gap-2">{@render actions()}</div>{/if}
  </div>
  <div class={['space-y-3', destructive && 'bg-destructive/5 px-4 py-4']}>{@render children()}</div>
</section>
