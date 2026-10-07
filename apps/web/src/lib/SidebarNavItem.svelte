<script lang="ts">
  // Paperclip's SidebarNavItem (ui/src/components/SidebarNavItem.tsx; MIT, see
  // NOTICE): one row of the sidebar, an inset pill when active. On the rail
  // the label is clipped but stays the link's name, and a tooltip names it.
  import type { Component } from 'svelte'
  import * as Tooltip from '$lib/components/ui/tooltip'
  import { RAIL_HIDDEN_LABEL, sidebar } from './sidebar.svelte'
  import { cn } from './utils'

  let {
    href,
    label,
    icon: Icon,
    active = false,
  }: { href: string; label: string; icon: Component<{ class?: string }>; active?: boolean } = $props()
</script>

{#snippet link(props: Record<string, unknown> = {})}
  <a
    {...props}
    type={undefined}
    {href}
    aria-current={active ? 'page' : undefined}
    onclick={() => sidebar.closeDrawer()}
    class={cn(
      'mx-2 flex items-center gap-2.5 rounded-lg px-2 py-1.5 text-(length:--text-compact) font-medium transition-colors pointer-coarse:py-1',
      // On the rail the pill fills the 40px column, centred on its icon.
      sidebar.rail && 'mx-0 justify-center gap-0 px-0',
      active ? 'bg-sidebar-accent text-sidebar-accent-foreground' : 'text-foreground/80 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground',
    )}
  >
    <span data-slot="sidebar-nav-icon" class="relative shrink-0"><Icon class="size-4" /></span>
    <span class={sidebar.rail ? RAIL_HIDDEN_LABEL : 'min-w-0 flex-1 truncate'}>{label}</span>
  </a>
{/snippet}

{#if sidebar.rail}
  <Tooltip.Root>
    <Tooltip.Trigger>
      {#snippet child({ props })}
        {@render link(props)}
      {/snippet}
    </Tooltip.Trigger>
    <Tooltip.Content side="right">{label}</Tooltip.Content>
  </Tooltip.Root>
{:else}
  {@render link()}
{/if}
