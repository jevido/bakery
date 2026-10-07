<script lang="ts">
  // Paperclip's CompanySettingsSidebar (ui/src/components/CompanySettingsSidebar.tsx
  // and ContextualSidebarFrame.tsx; MIT, see NOTICE): on settings routes it
  // takes the primary sidebar's place, with "Back to app" in the 60px header.
  // Paperclip lists its settings in one group; The Bakery splits them by whose
  // they are (the Current guild's, the person's, the installation's).
  import { ArrowLeft, Bell, KeyRound, Settings, Shield, ShieldAlert, SlidersHorizontal, UserRoundPen, Users } from '@lucide/svelte'
  import type { Component } from 'svelte'
  import { guildPath, href, router } from './router.svelte'
  import { session } from './session.svelte'
  import { sidebar } from './sidebar.svelte'
  import SidebarNavItem from './SidebarNavItem.svelte'
  import SidebarSection from './SidebarSection.svelte'

  type Item = { label: string; path: string; icon: Component<{ class?: string }>; active: boolean }
  type Section = { label: string; items: Item[] }

  let sections = $derived.by((): Section[] => {
    const route = router.route
    const guildPage = route.name === 'guild' ? route.page : null
    const items = (...list: (Item | false)[]) => list.filter((i): i is Item => i !== false)
    return [
      {
        label: session.guild?.name ?? 'Guild',
        items: items(
          { label: 'General', path: guildPath(), icon: SlidersHorizontal, active: guildPage === '' || route.name === 'guild-new' },
          { label: 'Members', path: guildPath('members'), icon: Users, active: guildPage === 'members' },
          { label: 'Roles', path: guildPath('roles'), icon: Shield, active: guildPage === 'roles' },
          session.can('manage_notifications') && { label: 'Notifications', path: '/notifications', icon: Bell, active: route.name === 'notifications' },
          { label: 'Danger Zone', path: guildPath('danger'), icon: ShieldAlert, active: guildPage === 'danger' },
        ),
      },
      {
        label: 'Account',
        items: [
          { label: 'Profile', path: '/profile', icon: UserRoundPen, active: route.name === 'profile' },
          { label: 'Keys & Tokens', path: '/security/api-tokens', icon: KeyRound, active: route.name === 'security' },
        ],
      },
      {
        label: 'Instance',
        items: items(session.can('manage_servers') && { label: 'Settings', path: '/settings', icon: Settings, active: route.name === 'settings' }),
      },
    ]
  })
</script>

<aside class="chrome primary-sidebar-surface flex h-full min-h-0 w-full flex-col" data-testid="settings-sidebar">
  <div class="flex h-(--sz-60px) shrink-0 items-center px-3">
    <div class="flex w-full flex-col gap-0.5">
      <SidebarNavItem href={href('/')} label="Back to app" icon={ArrowLeft} />
    </div>
  </div>
  <nav
    class={['flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-3 py-2 pointer-coarse:gap-3', !sidebar.rail && 'scrollbar-auto-hide']}
    aria-label="Settings"
  >
    {#each sections as section (section.label)}
      {#if section.items.length > 0}
        <SidebarSection label={section.label}>
          {#each section.items as item (item.path)}
            <SidebarNavItem href={href(item.path)} label={item.label} icon={item.icon} active={item.active} />
          {/each}
        </SidebarSection>
      {/if}
    {/each}
  </nav>
</aside>
