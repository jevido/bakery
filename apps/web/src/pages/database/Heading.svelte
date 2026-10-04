<script lang="ts">
  // Coolify's Database heading (resources/views/livewire/project/database/heading.blade.php,
  // Apache-2.0, see NOTICE): on phones the name, the status and a full-width
  // Actions menu above the page; on desktop Start, or Restart and Stop, are
  // moved into the top bar's resource slot, where the breadcrumb shows the
  // name and status. The buttons are shown only to those who may change it;
  // Restart and Stop open the page's confirmation modals by their hidden
  // triggers, as Coolify's do.
  import Icon from '../../lib/Icon.svelte'
  import StatusSummary from '../../lib/ui/StatusSummary.svelte'
  import { portalTo } from '../../lib/ui/portal'

  let {
    name,
    status,
    detail,
    error,
    stopped,
    canWrite,
    busy,
    onstart,
  }: {
    name: string
    /** Coolify's status string, e.g. `running`. */
    status: string
    /** The type and version, as a muted line. */
    detail: string
    /** Why the last start failed, or how the container exited. */
    error?: string
    /** Coolify offers Start for an exited Database; The Bakery for one meant to be stopped. */
    stopped: boolean
    canWrite: boolean
    busy: boolean
    onstart: () => void
  } = $props()

  let open = $state(false)
  let menu = $state<HTMLDivElement>()

  const click = (id: string) => document.getElementById(id)?.click()

  function outside(e: MouseEvent) {
    if (open && menu && !menu.contains(e.target as Node)) open = false
  }
</script>

<svelte:window onclick={outside} onkeydown={(e) => e.key === 'Escape' && (open = false)} />

<nav class="chrome w-full max-w-none pb-4 md:pb-6 lg:pb-0">
  <div class="mb-3 w-full xl:hidden">
    <div class="flex min-w-0 flex-col items-start gap-2">
      <h1 class="max-w-full min-w-0 truncate text-[24px]! leading-7! font-semibold! tracking-tight! text-black dark:text-fg">{name}</h1>
      <div class="relative flex w-full min-w-0 items-center gap-2">
        <StatusSummary {status} title="Database status" align="right" />
      </div>
    </div>
  </div>

  <p class="mb-3 text-[13px] text-neutral-500 dark:text-fg-dim">{detail}</p>
  {#if error}<p class="mb-3 text-[13px] text-error">{error}</p>{/if}

  {#if canWrite}
    <div class="w-full xl:hidden">
      <div class="relative mb-3" bind:this={menu}>
        <button type="button" class="button w-full justify-between" onclick={() => (open = !open)} aria-expanded={open} aria-haspopup="menu">
          <span class="inline-flex items-center gap-2">Actions</span>
          <span class={['inline-flex transition-transform', open && 'rotate-180']}>
            <Icon name="chevron-down" class="size-3 opacity-55" />
          </span>
        </button>
        {#if open}
          <div class="listbox-panel top-full! right-0! left-0! mt-1! w-full! min-w-0!" role="menu">
            {#if stopped}
              <button
                type="button"
                role="menuitem"
                class="listbox-option justify-start! gap-2.5!"
                disabled={busy}
                onclick={() => {
                  open = false
                  onstart()
                }}
              >
                <Icon name="play-circle" class="size-3.5 opacity-70" />
                Start
              </button>
            {:else}
              <button
                type="button"
                role="menuitem"
                class="listbox-option justify-start! gap-2.5!"
                disabled={busy}
                onclick={() => {
                  open = false
                  click('database-restart-trigger')
                }}
              >
                <Icon name="restart" class="size-3.5 opacity-70" />
                Restart
              </button>
              <button
                type="button"
                role="menuitem"
                class="listbox-option justify-start! gap-2.5!"
                disabled={busy}
                onclick={() => {
                  open = false
                  click('database-stop-trigger')
                }}
              >
                <Icon name="stop" class="size-3.5 text-error" />
                Stop
              </button>
            {/if}
          </div>
        {/if}
      </div>
    </div>
  {/if}

  <!-- Moved into the top bar; never this component's first or last node. -->
  <div class="hidden w-full items-center xl:flex xl:w-auto" {@attach portalTo('#resource-action-hud-slot')}>
    <div class="flex w-auto min-w-0 items-center justify-end gap-1 overflow-visible">
      {#if canWrite}
        <div class="flex shrink-0 items-center gap-0.5">
          {#if stopped}
            <button type="button" class="button button-highlighted" disabled={busy} onclick={onstart}>Start</button>
          {:else}
            <button type="button" class="button button-highlighted" disabled={busy} onclick={() => click('database-restart-trigger')}>Restart</button>
            <button type="button" class="button" disabled={busy} onclick={() => click('database-stop-trigger')}>Stop</button>
          {/if}
        </div>
      {/if}
    </div>
  </div>
</nav>
