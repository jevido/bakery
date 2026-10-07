<script lang="ts">
  // Paperclip's shell (ui/src/components/Layout.tsx and SidebarShell.tsx; MIT,
  // see NOTICE): the resizable sidebar with its account strip, collapsing to
  // the 64px icon rail, beside the breadcrumb bar and `main`. Below 768px the
  // sidebar is a drawer over a scrim. Left out until Issues and agents exist:
  // the command palette, search, the properties panel and the mobile bottom nav.
  import type { Snippet } from 'svelte'
  import * as Tooltip from '$lib/components/ui/tooltip'
  import AccountMenu from './AccountMenu.svelte'
  import BreadcrumbBar from './BreadcrumbBar.svelte'
  import { router } from './router.svelte'
  import SettingsSidebar from './SettingsSidebar.svelte'
  import Sidebar from './Sidebar.svelte'
  import { MAX_SIDEBAR_WIDTH, MIN_SIDEBAR_WIDTH, SIDEBAR_RAIL_WIDTH, SIDEBAR_WIDTH_STEP, sidebar } from './sidebar.svelte'
  import TransferOffers from './TransferOffers.svelte'
  import { cn } from './utils'

  let { children }: { children: Snippet } = $props()

  // Settings routes swap the primary sidebar for the settings sidebar, as
  // Paperclip's Layout does for its company settings.
  const SETTINGS_ROUTES = ['guild', 'guild-new', 'notifications', 'security', 'profile', 'settings']
  const Nav = $derived(SETTINGS_ROUTES.includes(router.route.name) ? SettingsSidebar : Sidebar)

  // A route change closes the drawer.
  $effect(() => {
    void router.route
    sidebar.open = false
  })

  // The width the sidebar takes in the flow, and the width of its panel: wider
  // than its place while peeking, when it floats over the page.
  const reserved = $derived(sidebar.collapsed ? SIDEBAR_RAIL_WIDTH : sidebar.width)
  const panel = $derived(sidebar.collapsed && !sidebar.peeking ? SIDEBAR_RAIL_WIDTH : sidebar.width)
  const overlay = $derived(panel > reserved)

  // Dragging the handle resizes the expanded sidebar; the width is stored when
  // the drag ends.
  let drag: { x: number; width: number } | null = $state(null)
  function dragStart(e: PointerEvent) {
    if (sidebar.collapsed) return
    e.preventDefault()
    ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
    drag = { x: e.clientX, width: sidebar.width }
  }
  function dragMove(e: PointerEvent) {
    if (drag) sidebar.setWidth(drag.width + e.clientX - drag.x, false)
  }
  function dragEnd() {
    if (!drag) return
    drag = null
    sidebar.setWidth(sidebar.width)
  }
  function resizeKey(e: KeyboardEvent) {
    const next = { ArrowLeft: sidebar.width - SIDEBAR_WIDTH_STEP, ArrowRight: sidebar.width + SIDEBAR_WIDTH_STEP, Home: MIN_SIDEBAR_WIDTH, End: MAX_SIDEBAR_WIDTH }[e.key]
    if (next === undefined) return
    e.preventDefault()
    sidebar.setWidth(next)
  }

  // The rail peeks open when keyboard focus enters it, so tabbing reaches
  // every label. A pointer gets each item's tooltip instead of Paperclip's
  // hover peek, which would open over the tooltips before they show.
  let peekTimer: ReturnType<typeof setTimeout> | undefined
  function focusIn(e: FocusEvent) {
    if (!sidebar.collapsed || !(e.target as HTMLElement).matches(':focus-visible')) return
    clearTimeout(peekTimer)
    sidebar.peeking = true
  }
  function focusOut(e: FocusEvent) {
    if (e.relatedTarget instanceof Node && (e.currentTarget as HTMLElement).contains(e.relatedTarget)) return
    clearTimeout(peekTimer)
    peekTimer = setTimeout(() => (sidebar.peeking = false), 120)
  }

  // Paperclip's `[` toggles the sidebar, except while typing.
  function shortcut(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      sidebar.open = false
      sidebar.peeking = false
      return
    }
    if (e.key !== '[' || e.metaKey || e.ctrlKey || e.altKey) return
    const t = e.target as HTMLElement | null
    if (t?.closest('input, textarea, select, [contenteditable=""], [contenteditable="true"]')) return
    e.preventDefault()
    sidebar.toggle()
  }
</script>

<svelte:window onkeydown={shortcut} />

<Tooltip.Provider delayDuration={0}>
  <div class={cn('bg-background text-foreground', sidebar.mobile ? 'min-h-dvh overflow-x-clip' : 'flex h-dvh overflow-clip')}>
    {#if sidebar.mobile}
      {#if sidebar.open}
        <button type="button" class="chrome fixed inset-0 z-40 bg-black/50" onclick={() => (sidebar.open = false)} aria-label="Close sidebar"></button>
      {/if}
      <div
        class={cn(
          'fixed inset-y-0 left-0 z-50 flex w-60 flex-col overflow-hidden bg-background transition-transform duration-100 ease-out',
          sidebar.open ? 'translate-x-0' : '-translate-x-full',
        )}
        inert={!sidebar.open}
        role="dialog"
        aria-modal="true"
        aria-label="Navigation"
        data-testid="sidebar-drawer"
      >
        <div class="flex min-h-0 flex-1 overflow-hidden"><Nav /></div>
        <AccountMenu />
      </div>
    {:else}
      <div class="relative h-full shrink-0" style:width="{reserved}px">
        <div
          class={cn(
            'absolute inset-y-0 left-0 flex flex-col overflow-hidden',
            overlay ? 'z-30 border-r border-border bg-background shadow-lg' : 'z-0',
          )}
          style:width="{panel}px"
          data-testid="sidebar"
          data-collapsed={sidebar.collapsed ? '' : undefined}
          onfocusin={focusIn}
          onfocusout={focusOut}
        >
          <div class="flex min-h-0 flex-1"><Nav /></div>
          <AccountMenu />
        </div>
        {#if !sidebar.collapsed}
          <!-- A focusable separator is ARIA's window splitter: its value moves with the arrow keys. -->
          <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
          <div
            role="separator"
            aria-label="Resize sidebar"
            aria-orientation="vertical"
            aria-valuemin={MIN_SIDEBAR_WIDTH}
            aria-valuemax={MAX_SIDEBAR_WIDTH}
            aria-valuenow={sidebar.width}
            tabindex="0"
            class="group absolute inset-y-0 -right-1 z-20 w-2 cursor-col-resize touch-none outline-none"
            onpointerdown={dragStart}
            onpointermove={dragMove}
            onpointerup={dragEnd}
            onpointercancel={dragEnd}
            onlostpointercapture={dragEnd}
            onkeydown={resizeKey}
            data-testid="sidebar-resize"
          >
            <div class={cn('mx-auto h-full w-0.5 transition-colors', drag ? 'bg-ring' : 'bg-transparent group-hover:bg-ring group-focus-visible:bg-ring')}></div>
          </div>
        {/if}
      </div>
    {/if}

    <div class={cn('flex min-w-0 flex-col', sidebar.mobile ? 'w-full' : 'h-full flex-1')}>
      <div class={cn(sidebar.mobile && 'sticky top-0 z-20 bg-background/95 backdrop-blur supports-backdrop-filter:bg-background/85')}>
        <BreadcrumbBar />
      </div>
      <main id="main-content" tabindex="-1" class={cn('flex-1 p-4 outline-none md:p-6', sidebar.mobile ? 'overflow-visible' : 'overflow-auto [scrollbar-gutter:stable]')}>
        <TransferOffers />
        {@render children()}
      </main>
    </div>
  </div>
</Tooltip.Provider>
