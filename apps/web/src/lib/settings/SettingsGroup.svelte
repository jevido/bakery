<script lang="ts">
  // One group of a Paperclip settings page (ui/src/pages/CompanySettings.tsx;
  // MIT, see NOTICE): a caps label over its fields, 2xl wide. The Danger
  // Zone's label is red and its body sits on a red wash.
  import type { Snippet } from 'svelte'

  let {
    label,
    hint,
    destructive = false,
    wide = false,
    children,
    ...rest
  }: { label: string; hint?: string; destructive?: boolean; wide?: boolean; children: Snippet; [key: `data-${string}`]: unknown } = $props()
</script>

<section class={['space-y-4', !wide && 'max-w-2xl']} {...rest}>
  <div>
    <div class={['text-xs font-medium tracking-wide uppercase', destructive ? 'text-destructive' : 'text-muted-foreground']}>{label}</div>
    {#if hint}<p class="mt-1 text-sm text-muted-foreground">{hint}</p>{/if}
  </div>
  <div class={['space-y-3', destructive && 'bg-destructive/5 px-4 py-4']}>{@render children()}</div>
</section>
