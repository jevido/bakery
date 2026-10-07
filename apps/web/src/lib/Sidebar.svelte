<script lang="ts">
  // Paperclip's Sidebar in its streamlined mode (ui/src/components/Sidebar.tsx
  // and primary-sidebar-styles.ts; MIT, see NOTICE): the Guild menu in a 60px
  // header, then The Bakery's nav. Agents, Issues and Inbox get their places
  // here in later phases; the Guild, Notifications, Keys & Tokens and
  // Settings open under the settings sidebar instead.
  import { FolderOpen, HardDrive, LayoutDashboard, Server } from '@lucide/svelte'
  import type { Component } from 'svelte'
  import GuildMenu from './GuildMenu.svelte'
  import { href, router } from './router.svelte'
  import { session } from './session.svelte'
  import { sidebar } from './sidebar.svelte'
  import SidebarNavItem from './SidebarNavItem.svelte'
  import SidebarSection from './SidebarSection.svelte'

  type Item = { label: string; path: string; icon: Component<{ class?: string }>; routes: string[] }
  type Section = { label?: string; items: Item[] }

  let sections = $derived.by((): Section[] => {
    const items = (...list: (Item | false)[]) => list.filter((i): i is Item => i !== false)
    return [
      { items: [{ label: 'Dashboard', path: '/', icon: LayoutDashboard, routes: ['dashboard'] }] },
      {
        label: 'Work',
        items: [
          {
            label: 'Projects',
            path: '/projects',
            icon: FolderOpen,
            routes: ['projects', 'project', 'project-edit', 'project-permissions', 'project-first-environment-new', 'environment', 'environment-edit', 'environment-new', 'application', 'application-legacy', 'database', 'database-legacy', 'service', 'service-legacy'],
          },
        ],
      },
      {
        label: 'Infrastructure',
        items: items(
          { label: 'Servers', path: '/servers', icon: Server, routes: ['servers', 'server-new', 'server'] },
          session.can('manage_servers') && { label: 'S3 Storage', path: '/storages', icon: HardDrive, routes: ['storages'] },
        ),
      },
    ]
  })
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
            <SidebarNavItem href={href(item.path)} label={item.label} icon={item.icon} active={item.routes.includes(router.route.name)} />
          {/each}
        </SidebarSection>
      {/if}
    {/each}
  </nav>
</aside>
