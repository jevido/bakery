<script lang="ts">
  // Paperclip's SidebarNavItem (ui/src/components/SidebarNavItem.tsx; MIT, see
  // NOTICE): one row of the sidebar, an inset pill when active. On the rail
  // the label is clipped but stays the link's name, and a tooltip names it.
  // An `inline` item sits in a page's own nav: never a rail, and it leaves
  // the drawer alone. A `brandIcon` is a brand mark masked from
  // /svgs/<name>.svg in place of a Lucide icon, as Coolify draws them. A
  // `badge` above 0 is a pill at the right; on the rail it is a dot on the
  // icon, and the link's name carries the count ("Inbox, 3 unread"). An
  // `iconNode` draws its own icon, as an Agent's in the Chats section.
  import type { Component, Snippet } from 'svelte'
  import { Badge } from '@bakery/ui/components/ui/badge'
  import * as Tooltip from '@bakery/ui/components/ui/tooltip'
  import { RAIL_HIDDEN_LABEL, sidebar } from './sidebar.svelte'
  import { cn } from '@bakery/ui/utils'

  let {
    href,
    label,
    icon: Icon,
    iconNode,
    brandIcon,
    active = false,
    inline = false,
    badge,
    badgeLabel,
    testid,
    class: className,
  }: {
    href: string
    label: string
    icon?: Component<{ class?: string }>
    iconNode?: Snippet
    brandIcon?: string
    active?: boolean
    inline?: boolean
    badge?: number
    /** The noun after the count on the rail, e.g. "unread". */
    badgeLabel?: string
    testid?: string
    class?: string
  } = $props()

  const rail = $derived(!inline && sidebar.rail)
  const hasBadge = $derived(badge != null && badge > 0)
  const railLabel = $derived(hasBadge ? `${label}, ${badge}${badgeLabel ? ` ${badgeLabel}` : ''}` : label)
</script>

{#snippet link(props: Record<string, unknown> = {})}
  <a
    {...props}
    type={undefined}
    {href}
    data-testid={testid}
    aria-current={active ? 'page' : undefined}
    aria-label={rail && hasBadge ? railLabel : undefined}
    onclick={() => !inline && sidebar.closeDrawer()}
    class={cn(
      'mx-2 flex items-center gap-2.5 rounded-lg px-2 py-1.5 text-(length:--text-compact) font-medium transition-colors pointer-coarse:py-1',
      // On the rail the pill fills the 40px column, centred on its icon.
      rail && 'mx-0 justify-center gap-0 px-0',
      active ? 'bg-sidebar-accent text-sidebar-accent-foreground' : 'text-foreground/80 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground',
      className,
    )}
  >
    <span data-slot="sidebar-nav-icon" class="relative shrink-0">
      {#if brandIcon}
        <span
          class="block size-4 bg-current"
          style="mask: url('/svgs/{brandIcon}.svg') center / contain no-repeat; -webkit-mask: url('/svgs/{brandIcon}.svg') center / contain no-repeat;"
        ></span>
      {:else if iconNode}
        {@render iconNode()}
      {:else if Icon}
        <Icon class="size-4" />
      {/if}
      {#if rail && hasBadge}
        <span
          data-slot="sidebar-nav-badge-dot"
          class="absolute -top-0.5 -right-0.5 size-2 rounded-full bg-primary shadow-(--shadow-sidebar-icon-badge)"
          aria-hidden="true"
        ></span>
      {/if}
    </span>
    <span class={rail ? RAIL_HIDDEN_LABEL : 'min-w-0 flex-1 truncate'}>{label}</span>
    {#if !rail && hasBadge}
      <Badge variant="ghost" data-testid="sidebar-nav-badge" class="ml-auto bg-primary px-1.5 leading-none text-primary-foreground">{badge}</Badge>
    {/if}
  </a>
{/snippet}

{#if rail}
  <Tooltip.Root>
    <Tooltip.Trigger>
      {#snippet child({ props })}
        {@render link(props)}
      {/snippet}
    </Tooltip.Trigger>
    <Tooltip.Content side="right">{railLabel}</Tooltip.Content>
  </Tooltip.Root>
{:else}
  {@render link()}
{/if}
