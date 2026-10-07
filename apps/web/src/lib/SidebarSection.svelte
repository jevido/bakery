<script lang="ts">
  // Paperclip's SidebarSection (ui/src/components/SidebarSection.tsx; MIT, see
  // NOTICE), without collapsing: a mono caps label over its items. On the
  // rail the label becomes a thin divider of the same height, so the icons
  // stay where they were.
  import type { Snippet } from 'svelte'
  import { sidebar } from './sidebar.svelte'

  let { label, children }: { label?: string; children: Snippet } = $props()
</script>

<div>
  {#if label}
    <div class="px-3 py-1.5 pointer-coarse:py-1">
      <div class="flex min-h-6 min-w-0 items-center">
        {#if sidebar.rail}
          <span class="sr-only">{label}</span>
          <div class="h-px w-full bg-border/60" aria-hidden="true"></div>
        {:else}
          <div class="inline-flex max-w-full min-w-0 items-center px-1 py-0.5">
            <span class="font-mono text-(length:--text-nano) font-medium tracking-widest text-muted-foreground/60 uppercase">{label}</span>
          </div>
        {/if}
      </div>
    </div>
  {/if}
  <div class={['flex flex-col gap-0.5', label && 'mt-0.5']}>{@render children()}</div>
</div>
