<script lang="ts">
  // Coolify's shell (resources/views/layouts/app.blade.php, components/navbar.blade.php
  // and components/top-breadcrumb.blade.php): fixed top bar with the brand and the
  // breadcrumb, a collapsible sidebar on desktop and a slide-over sheet on mobile.
  // `.chrome` keeps the legacy element rules in app.css off this markup.
  import type { Snippet } from 'svelte'
  import { breadcrumb } from './breadcrumb.svelte'
  import GuildSwitcher from './GuildSwitcher.svelte'
  import Icon, { type IconName } from './Icon.svelte'
  import { href, router } from './router.svelte'
  import { session } from './session.svelte'
  import StatusBadge from './ui/StatusBadge.svelte'
  import StatusSummary from './ui/StatusSummary.svelte'
  import UserMenu from './UserMenu.svelte'

  let { children }: { children: Snippet } = $props()

  type Item = { label: string; title?: string; path: string; icon: IconName; routes: string[] }
  type Section = { label: string; items: Item[] }

  // Coolify's sections and order, holding only the pages The Bakery has.
  let sections = $derived.by((): Section[] => {
    const items = (...list: (Item | false)[]) => list.filter((i): i is Item => i !== false)
    return [
      {
        label: 'Workspace',
        items: items(
          { label: 'Dashboard', path: '/', icon: 'dashboard', routes: ['dashboard'] },
          { label: 'Projects', path: '/projects', icon: 'projects', routes: ['projects', 'project', 'project-edit', 'environment', 'environment-edit', 'environment-new', 'application', 'application-legacy', 'database', 'database-legacy', 'service', 'service-legacy'] },
        ),
      },
      {
        label: 'Infrastructure',
        items: items(
          { label: 'Servers', path: '/servers', icon: 'servers', routes: ['servers', 'server-new', 'server'] },
          session.can('manage_servers') && { label: 'S3 Storage', path: '/storages', icon: 'storages', routes: ['storages'] },
        ),
      },
      {
        label: 'Manage',
        items: items(
          { label: 'Guild', path: '/guild', icon: 'teams', routes: ['guild', 'guild-new'] },
          session.can('manage_notifications') && { label: 'Notifications', path: '/notifications', icon: 'notifications', routes: ['notifications'] },
          { label: 'Keys & Tokens', path: '/security/api-tokens', icon: 'keys', routes: ['security'] },
          session.can('manage_servers') && { label: 'Settings', path: '/settings', icon: 'settings', routes: ['settings'] },
        ),
      },
    ]
  })

  let collapsed = $state(localStorage.getItem('sidebarCollapsed') === 'true')
  // Width transitions start only after the first paint, so a collapsed sidebar
  // does not animate shut on load.
  let ready = $state(false)
  $effect(() => {
    requestAnimationFrame(() => (ready = true))
  })

  function toggleSidebar() {
    collapsed = !collapsed
    localStorage.setItem('sidebarCollapsed', String(collapsed))
  }

  // The mobile sheet; a route change closes it.
  let open = $state(false)
  $effect(() => {
    void router.route
    open = false
  })

  // The collapsed sidebar shows each item's name in a floating tooltip.
  let tooltip = $state({ text: '', x: 0, y: 0, show: false })
  function hover(e: PointerEvent) {
    if (!collapsed) return
    const el = (e.target as Element).closest('.menu-item')
    if (!el) {
      tooltip.show = false
      return
    }
    const text = el.getAttribute('title') || el.getAttribute('aria-label') || ''
    if (!text) return
    const rect = el.getBoundingClientRect()
    tooltip = { text, x: rect.right + 8, y: rect.top + rect.height / 2, show: true }
  }
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && (open = false)} />

{#snippet navbar(collapsed: boolean, desktop: boolean)}
  <nav
    class={[
      'chrome flex flex-1 flex-col border-r border-neutral-200 bg-white px-2 pt-2 lg:px-3 dark:border-white/[0.06] dark:bg-panel',
      collapsed && 'sidebar-collapsed',
    ]}
    onpointerover={hover}
    onpointerleave={() => (tooltip.show = false)}
  >
    <ul role="list" class="-mx-1 flex min-h-0 flex-1 flex-col gap-y-0.5 overflow-y-auto px-1 pb-2">
      {#each sections as section, i (section.label)}
        {#if section.items.length > 0}
          <li class={['nav-section', i > 0 && 'mt-3', collapsed && 'lg:hidden']}>{section.label}</li>
          {#each section.items as item (item.path)}
            <li>
              <a
                title={item.title ?? item.label}
                href={href(item.path)}
                class={['menu-item', item.routes.includes(router.route.name) && 'menu-item-active', collapsed && 'lg:justify-center lg:px-0']}
                aria-current={item.routes.includes(router.route.name) ? 'page' : undefined}
              >
                <Icon name={item.icon} class="menu-item-icon" />
                <span class={['menu-item-label', collapsed && 'lg:hidden']}>{item.label}</span>
              </a>
            </li>
          {/each}
        {/if}
      {/each}
      <li class="flex-1" aria-hidden="true"></li>
    </ul>
    {#if desktop}
      <div
        class={[
          'sticky bottom-0 -mx-2 mt-auto hidden items-center gap-1 bg-white px-2 py-2 lg:-mx-3 lg:flex lg:px-3 dark:bg-panel',
          collapsed ? 'flex-col-reverse justify-center' : 'justify-between',
        ]}
      >
        <UserMenu sidebar {collapsed} />
        <button
          type="button"
          onclick={toggleSidebar}
          title="Toggle sidebar"
          aria-label="Toggle sidebar"
          aria-pressed={collapsed}
          class="menu-item w-8 shrink-0 justify-center px-0"
        >
          <svg class="menu-item-icon" viewBox="0 0 24 24" fill="none">
            <rect x="3" y="4" width="18" height="16" rx="2" stroke="currentColor" stroke-width="1.6" />
            <path d="M9 4v16" stroke="currentColor" stroke-width="1.6" />
          </svg>
        </button>
      </div>
    {/if}
  </nav>
{/snippet}

<div class="text-black dark:text-inherit">
  <!-- Desktop top bar -->
  <header class="chrome fixed inset-x-0 top-0 z-50 hidden h-12 items-center bg-white/95 backdrop-blur lg:flex dark:bg-panel/95">
    <div
      class={[
        'flex h-full shrink-0 items-center gap-2 border-r border-neutral-200 transition-[width] duration-200 dark:border-white/[0.06]',
        collapsed ? 'w-16 justify-center px-0' : 'w-56 px-4',
      ]}
    >
      <a href={href('/')} title="The Bakery" class="flex items-center transition-opacity hover:opacity-80">
        {#if collapsed}
          <img src="/favicon.svg" alt="The Bakery" class="size-5" />
        {:else}
          <span class="text-[15px] font-semibold tracking-tight text-black dark:text-white">The Bakery</span>
        {/if}
      </a>
    </div>
    <div class="flex h-full min-w-0 flex-1 items-center gap-0.5 border-b border-neutral-200 pr-4 pl-3 dark:border-white/[0.06]">
      <div class="relative flex min-w-0 flex-1 items-center">
        <!-- The Guild switcher leads the breadcrumb, as Coolify's team switcher does. -->
        <div class="shrink-0"><GuildSwitcher /></div>
        <nav aria-label="Breadcrumb" class="flex min-w-0 items-center gap-0.5 text-[13px]">
          {#each breadcrumb.crumbs as crumb, i (i)}
            <span class="shrink-0 px-0.5 text-neutral-300 dark:text-fg-faint">/</span>
            {#if crumb.href}
              <a
                href={crumb.href}
                class="flex h-8 min-w-0 shrink items-center rounded-md px-2 opacity-70 transition-[background-color,opacity] hover:bg-neutral-100 hover:opacity-100 dark:hover:bg-white/[0.05]"
              >
                <span class="min-w-0 truncate font-semibold text-black dark:text-fg">{crumb.label}</span>
              </a>
            {:else}
              <span class="flex h-8 min-w-0 items-center truncate px-2 font-semibold text-black dark:text-fg" aria-current="page">
                {crumb.label}
              </span>
            {/if}
            {#if crumb.summary}
              <div class="ml-1"><StatusSummary status={crumb.summary} title={crumb.summaryTitle} /></div>
            {:else if crumb.status}
              <StatusBadge status={crumb.status.label} type={crumb.status.type} class="ml-1 shrink-0" />
            {/if}
          {/each}
        </nav>
        <!-- A Server page's switcher and status summary dock here, after the breadcrumb. -->
        <div id="server-topbar-context" class="min-w-0"></div>
      </div>
      <!-- Resource actions dock here on desktop (the Application heading's Links and Actions). -->
      <div id="resource-action-hud-slot" class="hidden shrink-0 items-center xl:flex"></div>
    </div>
  </header>

  <!-- Mobile slide-over sidebar -->
  <div class="chrome relative z-[1000] lg:hidden">
    {#if open}
      <div class="fixed inset-0 bg-black/50" onclick={() => (open = false)} aria-hidden="true"></div>
    {/if}
    <div class={['fixed inset-y-0 right-0 flex', !open && 'pointer-events-none']}>
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Navigation menu"
        inert={!open}
        class={[
          'relative flex h-full w-72 max-w-[85vw] min-w-0 flex-col overflow-hidden rounded-l-2xl border-l border-neutral-200 bg-white shadow-[-8px_0_30px_-6px_rgba(0,0,0,0.18)] transition-transform dark:border-white/[0.12] dark:bg-panel dark:shadow-[-8px_0_30px_-4px_rgba(0,0,0,0.5)]',
          // Closed, it moves its own width plus its shadow's reach off-screen,
          // so the shadow does not show at the edge of the page.
          open ? 'translate-x-0 duration-300 ease-[cubic-bezier(0.32,0.72,0,1)]' : 'translate-x-[calc(100%+3rem)] duration-200 ease-in',
        ]}
      >
        <div class="flex h-12 shrink-0 items-center justify-between gap-1.5 border-b border-neutral-200 px-4 dark:border-white/[0.06]">
          <a href={href('/')} title="The Bakery" class="text-[15px] font-semibold tracking-tight text-black transition-opacity hover:opacity-80 dark:text-white">
            The Bakery
          </a>
          <button
            type="button"
            onclick={() => (open = false)}
            aria-label="Close menu"
            class="-mr-1.5 flex size-8 shrink-0 items-center justify-center rounded-md text-neutral-500 transition-colors hover:bg-neutral-100 hover:text-black active:scale-95 dark:text-fg-dim dark:hover:bg-white/[0.06] dark:hover:text-fg"
          >
            <svg class="size-5" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor" aria-hidden="true">
              <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
        <div class="flex min-h-0 min-w-0 flex-1 flex-col overflow-y-auto pb-2">
          {@render navbar(false, false)}
        </div>
      </div>
    </div>
  </div>

  <!-- Desktop sidebar, below the top bar -->
  <div
    class={[
      'hidden min-w-0 lg:fixed lg:top-12 lg:bottom-0 lg:left-0 lg:z-40 lg:flex lg:flex-col',
      collapsed ? 'lg:w-16' : 'lg:w-56',
      ready && 'transition-[width] duration-200',
    ]}
  >
    <div class="flex min-w-0 grow flex-col overflow-visible">
      {@render navbar(collapsed, true)}
    </div>
  </div>

  <!-- Mobile top bar -->
  <div
    class="chrome sticky top-0 z-40 flex items-center justify-between gap-x-4 border-b border-neutral-200/60 bg-white/95 px-4 py-3 backdrop-blur-sm sm:px-6 lg:hidden dark:border-white/[0.06] dark:bg-panel/95"
  >
    <div class="flex min-w-0 flex-1 items-center gap-2.5">
      <a
        href={href('/')}
        class="flex size-8 shrink-0 items-center justify-center rounded-lg bg-neutral-100 transition-opacity hover:opacity-80 dark:bg-white/[0.06]"
      >
        <img src="/favicon.svg" alt="The Bakery" class="h-[18px] w-[18px]" />
      </a>
      <GuildSwitcher />
    </div>
    <div class="flex shrink-0 items-center gap-1">
      <UserMenu />
      <button
        type="button"
        onclick={() => (open = !open)}
        class="-mr-1 flex size-9 items-center justify-center rounded-md text-neutral-500 transition-transform duration-100 ease-out hover:bg-neutral-100 hover:text-black active:scale-90 dark:text-fg-dim dark:hover:bg-white/[0.06] dark:hover:text-fg"
      >
        <span class="sr-only">Open sidebar</span>
        <svg class="size-5" viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <rect x="3" y="4" width="18" height="16" rx="2" stroke="currentColor" stroke-width="1.6" />
          <path d="M9 4v16" stroke="currentColor" stroke-width="1.6" />
        </svg>
      </button>
    </div>
  </div>

  <main
    class={[
      'min-h-screen bg-neutral-50 px-5 py-6 sm:px-8 lg:px-10 lg:pt-[calc(3rem+1.75rem)] lg:pb-10 dark:bg-app',
      collapsed ? 'lg:ml-16' : 'lg:ml-56',
      ready && 'transition-[margin] duration-200',
    ]}
  >
    <div class="w-full max-w-none">
      {@render children()}
    </div>
  </main>

  {#if collapsed && tooltip.show}
    <div
      style="left: {tooltip.x}px; top: {tooltip.y}px;"
      class="pointer-events-none fixed z-[10000] -translate-y-1/2 rounded-lg border border-neutral-700 bg-neutral-900 px-2 py-1 text-xs font-medium whitespace-nowrap text-white shadow-dropdown dark:border-white/10 dark:bg-raised"
    >
      {tooltip.text}
    </div>
  {/if}
</div>
