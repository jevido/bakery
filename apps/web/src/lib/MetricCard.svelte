<script lang="ts">
  // Paperclip's MetricCard (components/MetricCard.tsx, MIT, see NOTICE): a
  // large number, its label and an optional line under it, with a faint icon
  // on the right. With `href` the whole card is a link.
  import type { Snippet } from 'svelte'
  import Icon, { type IconName } from './Icon.svelte'

  let {
    icon,
    value,
    label,
    href,
    description,
  }: {
    icon: IconName
    value: string | number
    label: string
    href?: string
    description?: Snippet
  } = $props()
</script>

{#snippet inner()}
  <div class={['h-full rounded-lg px-4 py-4 transition-colors sm:px-5 sm:py-5', href && 'cursor-pointer hover:bg-accent/50']}>
    <div class="flex items-start justify-between gap-3">
      <div class="min-w-0 flex-1">
        <p class="text-2xl font-semibold tracking-tight tabular-nums sm:text-3xl">{value}</p>
        <p class="mt-1 text-xs font-medium text-muted-foreground sm:text-sm">{label}</p>
        {#if description}
          <div class="mt-1.5 hidden text-xs text-muted-foreground/70 sm:block">{@render description()}</div>
        {/if}
      </div>
      <Icon name={icon} class="mt-1.5 size-4 shrink-0 text-muted-foreground/50" />
    </div>
  </div>
{/snippet}

{#if href}
  <a {href} class="h-full text-inherit no-underline">{@render inner()}</a>
{:else}
  {@render inner()}
{/if}
