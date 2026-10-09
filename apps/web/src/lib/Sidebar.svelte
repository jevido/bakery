<script lang="ts">
  // Paperclip's Sidebar in its streamlined mode (ui/src/components/Sidebar.tsx
  // and primary-sidebar-styles.ts; MIT, see NOTICE): the Guild menu in a 60px
  // header, then The Bakery's nav. The first section holds Dashboard and
  // the Inbox with its count of Unread Issues and Actionable
  // Approvals ("unread", as Paperclip labels it); the Work section holds Projects,
  // Issues, Goals and Routines; the Guild section holds Agents, Activity and Costs, as
  // Paperclip's streamlined Sidebar puts Agents beside Audit and Costs in its
  // Org section, which is what The Bakery's Guild section is. Approvals have no
  // item, as in Paperclip: they are reached through the Inbox, the
  // Dashboard's Pending Approvals card and an Issue's page. The Guild's
  // settings, Notifications, Keys & Tokens and Settings open under the
  // settings sidebar instead.
  import { CircleDot, DollarSign, FolderOpen, HardDrive, History, Inbox, LayoutDashboard, Repeat, Server, Target, Users } from '@lucide/svelte'
  import type { Component } from 'svelte'
  import GuildMenu from './GuildMenu.svelte'
  import { badges, pollBadges, refreshBadges } from './inbox.svelte'
  import { href, router } from './router.svelte'
  import { session } from './session.svelte'
  import { sidebar } from './sidebar.svelte'
  import SidebarNavItem from './SidebarNavItem.svelte'
  import SidebarSection from './SidebarSection.svelte'

  type Item = { label: string; path: string; icon: Component<{ class?: string }>; routes: string[]; badge?: number; badgeLabel?: string }
  type Section = { label?: string; items: Item[] }

  let sections = $derived.by((): Section[] => {
    const items = (...list: (Item | false)[]) => list.filter((i): i is Item => i !== false)
    return [
      {
        items: [
          { label: 'Dashboard', path: '/', icon: LayoutDashboard, routes: ['dashboard'] },
          { label: 'Inbox', path: '/inbox', icon: Inbox, routes: ['inbox'], badge: badges.inbox, badgeLabel: 'unread' },
        ],
      },
      {
        label: 'Work',
        items: [
          {
            label: 'Projects',
            path: '/projects',
            icon: FolderOpen,
            routes: ['projects', 'project', 'project-edit', 'project-permissions', 'project-first-environment-new', 'environment', 'environment-edit', 'environment-new', 'application', 'application-legacy', 'database', 'database-legacy', 'service', 'service-legacy'],
          },
          { label: 'Issues', path: '/issues', icon: CircleDot, routes: ['issues', 'issue'] },
          { label: 'Goals', path: '/goals', icon: Target, routes: ['goals', 'goal'] },
          { label: 'Routines', path: '/routines', icon: Repeat, routes: ['routines', 'routine'] },
        ],
      },
      {
        label: 'Infrastructure',
        items: items(
          { label: 'Servers', path: '/servers', icon: Server, routes: ['servers', 'server-new', 'server'] },
          session.can('manage_servers') && { label: 'S3 Storage', path: '/storages', icon: HardDrive, routes: ['storages'] },
        ),
      },
      {
        label: 'Guild',
        items: [
          { label: 'Agents', path: '/agents/all', icon: Users, routes: ['agents', 'agent'] },
          { label: 'Activity', path: '/activity', icon: History, routes: ['activity'] },
          { label: 'Costs', path: '/costs', icon: DollarSign, routes: ['costs'] },
        ],
      },
    ]
  })

  // The count is read again on every route change and Guild switch.
  $effect(() => {
    void router.route
    void session.guild?.id
    refreshBadges()
  })
  $effect(() => pollBadges())
</script>

<aside class="chrome primary-sidebar-surface flex h-full min-h-0 w-full flex-col">
  <div class="flex h-(--sz-60px) shrink-0 items-center gap-1 px-3">
    <GuildMenu />
  </div>
  <!-- The auto-hiding scrollbar keeps its gutter, which the rail has no room for. -->
  <nav
    class={['flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-3 py-2 pointer-coarse:gap-3', !sidebar.rail && 'scrollbar-auto-hide']}
    aria-label="Main"
  >
    {#each sections as section, i (section.label ?? i)}
      {#if section.items.length > 0}
        <SidebarSection label={section.label}>
          {#each section.items as item (item.path)}
            <SidebarNavItem href={href(item.path)} label={item.label} icon={item.icon} active={item.routes.includes(router.route.name)} badge={item.badge} badgeLabel={item.badgeLabel} />
          {/each}
        </SidebarSection>
      {/if}
    {/each}
  </nav>
</aside>
