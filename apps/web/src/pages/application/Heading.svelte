<script lang="ts" module>
  import type { IconName } from '../../lib/Icon.svelte'

  /** One row of the Actions menu. */
  export type Action = { label: string; icon: IconName; run: () => void; disabled?: boolean; danger?: boolean }
</script>

<script lang="ts">
  // Coolify's Application heading (resources/views/livewire/project/application/heading.blade.php,
  // Apache-2.0, see NOTICE): on phones the name, the status, the Links pill and a
  // full-width Actions menu above the page; on desktop the Links and Actions are
  // moved into the top bar's resource slot, where the breadcrumb shows the name
  // and status. Actions is shown only to those who may deploy.
  import Icon from '../../lib/Icon.svelte'
  import StatusSummary from '../../lib/ui/StatusSummary.svelte'
  import { portalTo } from '../../lib/ui/portal'
  import Links from './Links.svelte'

  let {
    name,
    urls,
    status,
    actions,
    mobileActions = actions,
  }: {
    name: string
    urls: string[]
    /** Coolify's status string, e.g. `running:healthy`; null until known. */
    status: string | null
    /** Empty for a viewer: no Actions button at all. */
    actions: Action[]
    /** The phone menu, which Coolify orders a little differently. */
    mobileActions?: Action[]
  } = $props()

  let mobileOpen = $state(false)
  let desktopOpen = $state(false)
  let mobileMenu = $state<HTMLDivElement>()
  let desktopMenu = $state<HTMLDivElement>()

  function outside(e: MouseEvent) {
    const target = e.target as Node
    if (mobileOpen && mobileMenu && !mobileMenu.contains(target)) mobileOpen = false
    if (desktopOpen && desktopMenu && !desktopMenu.contains(target)) desktopOpen = false
  }

  function escape(e: KeyboardEvent) {
    if (e.key !== 'Escape') return
    mobileOpen = false
    desktopOpen = false
  }
</script>

<svelte:window onclick={outside} onkeydown={escape} />

{#snippet items(list: Action[], close: () => void)}
  {#each list as action (action.label)}
    <button
      type="button"
      role="menuitem"
      class="listbox-option justify-start! gap-2.5! disabled:cursor-not-allowed disabled:opacity-50"
      disabled={action.disabled}
      onclick={() => {
        close()
        action.run()
      }}
    >
      <Icon name={action.icon} class={['size-3.5', action.danger ? 'text-error' : 'opacity-70'].join(' ')} />
      {action.label}
    </button>
  {/each}
{/snippet}

<nav class="chrome w-full max-w-none pb-4 md:pb-6 lg:pb-0">
  <div class="mb-3 w-full xl:hidden">
    <div class="flex min-w-0 flex-col items-start gap-2">
      <h1 class="max-w-full min-w-0 truncate text-[24px]! leading-7! font-semibold! tracking-tight! text-black dark:text-fg">{name}</h1>
      <div class="relative flex w-full min-w-0 items-center gap-2">
        {#if status}<StatusSummary {status} align="right" />{/if}
        <Links {urls} compact />
      </div>
    </div>
  </div>

  {#if actions.length > 0}
    <div class="w-full xl:hidden">
      <div class="relative mb-3" bind:this={mobileMenu}>
        <button
          type="button"
          class="button w-full justify-between"
          onclick={() => (mobileOpen = !mobileOpen)}
          aria-expanded={mobileOpen}
          aria-haspopup="menu"
        >
          <span class="inline-flex items-center gap-2">Actions</span>
          <span class={['inline-flex transition-transform', mobileOpen && 'rotate-180']}>
            <Icon name="chevron-down" class="size-3 opacity-55" />
          </span>
        </button>
        {#if mobileOpen}
          <div class="listbox-panel top-full! right-0! left-0! mt-1! w-full! min-w-0!" role="menu">
            {@render items(mobileActions, () => (mobileOpen = false))}
          </div>
        {/if}
      </div>
    </div>
  {/if}

  <!-- Moved into the top bar; never this component's first or last node. -->
  <div class="hidden w-full items-center xl:flex xl:w-auto" {@attach portalTo('#resource-action-hud-slot')}>
    <div class="flex w-full min-w-0 items-center justify-start gap-1 overflow-visible xl:w-auto xl:justify-end">
      <div class="flex shrink-0 items-center gap-0.5">
        <div class="shrink-0"><Links {urls} /></div>
        {#if actions.length > 0}
          <div class="relative" bind:this={desktopMenu}>
            <button
              type="button"
              class="button button-highlighted"
              onclick={() => (desktopOpen = !desktopOpen)}
              aria-expanded={desktopOpen}
              aria-haspopup="menu"
            >
              Actions
              <Icon name="chevron-down" class="size-3 opacity-55" />
            </button>
            {#if desktopOpen}
              <div class="listbox-panel top-full! right-0! left-auto! mt-1! w-60! min-w-0!" role="menu">
                {@render items(actions, () => (desktopOpen = false))}
              </div>
            {/if}
          </div>
        {/if}
      </div>
    </div>
  </div>
</nav>
