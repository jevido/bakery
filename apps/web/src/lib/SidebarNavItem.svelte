<script lang="ts">
  // Paperclip's SidebarNavItem (ui/src/components/SidebarNavItem.tsx; MIT, see
  // NOTICE): one row of the sidebar, an inset pill when active. On the rail
  // the label is clipped but stays the link's name, and a tooltip names it.
  // An `inline` item sits in a page's own nav: never a rail, and it leaves
  // the drawer alone. A `brandIcon` is a brand mark masked from
  // /svgs/<name>.svg in place of a Lucide icon, as Coolify draws them.
  import type { Component } from 'svelte'
  import * as Tooltip from '$lib/components/ui/tooltip'
  import { RAIL_HIDDEN_LABEL, sidebar } from './sidebar.svelte'
  import { cn } from './utils'

  let {
    href,
    label,
    icon: Icon,
    brandIcon,
    active = false,
    inline = false,
    testid,
  }: {
    href: string
    label: string
    icon?: Component<{ class?: string }>
    brandIcon?: string
    active?: boolean
    inline?: boolean
    testid?: string
  } = $props()

  const rail = $derived(!inline && sidebar.rail)
</script>

{#snippet link(props: Record<string, unknown> = {})}
  <a
    {...props}
    type={undefined}
    {href}
    data-testid={testid}
    aria-current={active ? 'page' : undefined}
    onclick={() => !inline && sidebar.closeDrawer()}
    class={cn(
      'mx-2 flex items-center gap-2.5 rounded-lg px-2 py-1.5 text-(length:--text-compact) font-medium transition-colors pointer-coarse:py-1',
      // On the rail the pill fills the 40px column, centred on its icon.
      rail && 'mx-0 justify-center gap-0 px-0',
      active ? 'bg-sidebar-accent text-sidebar-accent-foreground' : 'text-foreground/80 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground',
    )}
  >
    <span data-slot="sidebar-nav-icon" class="relative shrink-0">
      {#if brandIcon}
        <span
          class="block size-4 bg-current"
          style="mask: url('/svgs/{brandIcon}.svg') center / contain no-repeat; -webkit-mask: url('/svgs/{brandIcon}.svg') center / contain no-repeat;"
        ></span>
      {:else if Icon}
        <Icon class="size-4" />
      {/if}
    </span>
    <span class={rail ? RAIL_HIDDEN_LABEL : 'min-w-0 flex-1 truncate'}>{label}</span>
  </a>
{/snippet}

{#if rail}
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
