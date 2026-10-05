<script lang="ts">
  // Coolify's Servers page (resources/views/livewire/server/index.blade.php,
  // Apache-2.0, see NOTICE): "New server", search, a remembered table/grid
  // switch and each Server with its description and status.
  //
  // Left out: "Import transfer" (dev only in Coolify, no Transfer here) and the
  // grid card's metrics chart (Coolify draws it only with Sentinel metrics).
  import { api } from '../../lib/api'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import Icon from '../../lib/Icon.svelte'
  import { href, serverPath } from '../../lib/router.svelte'
  import { session } from '../../lib/session.svelte'
  import type { Server } from '../../lib/types'
  import Empty from '../../lib/ui/Empty.svelte'
  import Spinner from '../../lib/ui/Spinner.svelte'
  import { serverStatus } from './status'

  type ViewMode = 'table' | 'grid'
  const viewKey = 'bakery-servers-view'

  let servers = $state.raw<Server[] | null>(null)
  let loadError = $state('')
  let search = $state('')
  let viewMode = $state<ViewMode>(localStorage.getItem(viewKey) === 'grid' ? 'grid' : 'table')

  async function load() {
    const r = await api<{ servers: Server[] }>('GET', '/servers')
    servers = r.servers
  }

  $effect(() => {
    load().catch((e) => (loadError = e.message))
    const t = setInterval(() => load().catch(() => {}), 10000)
    return () => clearInterval(t)
  })

  // The search box filters 150 ms after the last key, as Coolify's debounce does.
  let query = $state('')
  $effect(() => {
    const value = search
    const timer = setTimeout(() => (query = value.trim().toLowerCase()), 150)
    return () => clearTimeout(timer)
  })

  const rows = $derived((servers ?? []).map((s) => ({ server: s, description: s.description, status: serverStatus(s) })))
  const filtered = $derived(
    rows.filter((r) => !query || [r.server.name, r.description, r.status.label].some((v) => v.toLowerCase().includes(query))),
  )

  function setViewMode(mode: ViewMode) {
    viewMode = mode
    localStorage.setItem(viewKey, mode)
  }

  const toggleIdle =
    'text-neutral-400 hover:bg-neutral-100 hover:text-black dark:text-fg-faint dark:hover:bg-white/[0.06] dark:hover:text-fg'

  $effect(() => breadcrumb.set({ label: 'Servers' }))
</script>

{#snippet warning(status: { label: string; type: string; detail: string })}
  {#if status.type !== 'success'}
    <span
      title={status.detail || status.label}
      aria-label="Server status: {status.label}"
      class="ml-auto flex size-6 shrink-0 items-center justify-center rounded-md text-red-500 dark:text-red-400"
    >
      <Icon name="alert-triangle" class="size-4" />
    </span>
  {/if}
{/snippet}

<div class="chrome application-settings-form w-full">
  <div class="mb-5 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
    <h1 class="min-w-0 text-[24px]! leading-7! font-semibold! tracking-tight!">Servers</h1>
    {#if session.isAdmin}
      <div class="flex flex-wrap items-center gap-2">
        <a href={href('/servers/new')} class="button button-highlighted w-fit shrink-0 whitespace-nowrap">
          <Icon name="plus" class="size-3.5" />
          New server
        </a>
      </div>
    {/if}
  </div>

  {#if loadError}
    <p class="text-sm text-error">{loadError}</p>
  {:else if servers === null}
    <Spinner text="Loading…" />
  {:else if servers.length === 0}
    <Empty title="No servers yet" description="Add a server to deploy applications, databases, and services." icon="servers" />
  {:else}
    <div class="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div class="relative w-full sm:max-w-sm">
        <Icon
          name="search"
          class="pointer-events-none absolute top-1/2 left-2.5 z-10 size-3.5 -translate-y-1/2 text-neutral-400 dark:text-fg-faint"
        />
        <input
          bind:value={search}
          type="search"
          placeholder="Search servers"
          aria-label="Search servers"
          class="input h-8! w-full rounded-lg! border-neutral-200! bg-white! py-0! pr-8! pl-8! text-[12px]! shadow-none! placeholder:text-neutral-400 focus:border-accent! focus:ring-0! dark:border-white/[0.08]! dark:bg-white/[0.035]! dark:text-fg! dark:placeholder:text-fg-faint"
        />
        {#if search}
          <button
            type="button"
            onclick={() => (search = '')}
            class="absolute top-1/2 right-2 flex size-5 -translate-y-1/2 items-center justify-center rounded text-neutral-400 transition-colors hover:bg-neutral-100 hover:text-black dark:text-fg-faint dark:hover:bg-white/[0.07] dark:hover:text-fg"
            aria-label="Clear search"
          >
            <Icon name="x" class="size-3" />
          </button>
        {/if}
      </div>

      <div class="flex items-center gap-3">
        <span class="text-[11px] text-neutral-500 dark:text-fg-faint" data-testid="servers-count">
          {filtered.length}
          {filtered.length === 1 ? 'server' : 'servers'}
        </span>
        <div class="view-toggle">
          <button
            type="button"
            onclick={() => setViewMode('table')}
            class={['flex size-7.5 items-center justify-center rounded-md transition-colors', viewMode === 'table' ? 'control-selected' : toggleIdle]}
            aria-label="Table view"
            aria-pressed={viewMode === 'table'}
          >
            <Icon name="unordered-list" class="size-3.5" />
          </button>
          <button
            type="button"
            onclick={() => setViewMode('grid')}
            class={['flex size-7.5 items-center justify-center rounded-md transition-colors', viewMode === 'grid' ? 'control-selected' : toggleIdle]}
            aria-label="Grid view"
            aria-pressed={viewMode === 'grid'}
          >
            <Icon name="grid" class="size-3.5" />
          </button>
        </div>
      </div>
    </div>

    {#if filtered.length === 0}
      <div
        class="flex min-h-52 flex-col items-center justify-center rounded-xl border border-neutral-200 bg-white px-6 text-center dark:border-white/[0.08] dark:bg-white/[0.05]"
      >
        <Icon name="search" class="mb-3 size-6 text-neutral-300 dark:text-fg-faint" />
        <p class="text-[13px] font-medium">No matching servers</p>
        <p class="mt-1 text-[12px] text-neutral-500 dark:text-fg-dim">Try a different search.</p>
      </div>
    {:else if viewMode === 'grid'}
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
        {#each filtered as row (row.server.id)}
          <a
            href={href(serverPath(row.server.id))}
            data-testid="server"
            class="group relative flex min-h-28 flex-col rounded-xl border border-neutral-200 bg-white p-3 shadow-sm transition-all hover:-translate-y-px hover:border-neutral-300 hover:no-underline hover:shadow-md dark:border-white/[0.08] dark:bg-white/[0.05] dark:hover:border-white/[0.14]"
          >
            <div class="relative z-10 flex items-start gap-3">
              <div
                class="flex size-8 shrink-0 items-center justify-center rounded-lg border border-neutral-200 bg-neutral-50 text-neutral-500 dark:border-white/[0.1] dark:bg-white/[0.04] dark:text-fg-dim"
              >
                <Icon name="servers" class="size-4" />
              </div>
              <div class="min-w-0 flex-1">
                <h2 class="truncate text-[13px]! leading-4! font-semibold! text-black dark:text-fg">{row.server.name}</h2>
                <p class="mt-0.5 truncate text-[11px] text-neutral-500 dark:text-fg-faint">{row.description}</p>
              </div>
              {@render warning(row.status)}
            </div>
          </a>
        {/each}
      </div>
    {:else}
      <div class="overflow-x-auto rounded-xl border border-neutral-200 bg-white shadow-sm dark:border-white/[0.08] dark:bg-white/[0.05]">
        <div
          class="grid min-w-[480px] grid-cols-[minmax(0,1fr)_9.5rem] border-b border-neutral-200 bg-neutral-50 px-4 py-2.5 text-[11px] font-medium text-neutral-500 dark:border-white/[0.08] dark:bg-white/[0.05] dark:text-fg-faint"
        >
          <div>Server</div>
          <div>Status</div>
        </div>
        {#each filtered as row (row.server.id)}
          <a
            href={href(serverPath(row.server.id))}
            data-testid="server"
            class="grid min-h-14 min-w-[480px] grid-cols-[minmax(0,1fr)_9.5rem] items-center border-b border-neutral-200 px-4 py-2.5 text-[12px] transition-colors last:border-b-0 hover:bg-neutral-50 hover:no-underline dark:border-white/[0.07] dark:hover:bg-white/[0.025]"
          >
            <div class="flex min-w-0 items-center gap-3">
              <div
                class="flex size-8 shrink-0 items-center justify-center rounded-lg border border-neutral-200 bg-neutral-50 text-neutral-500 dark:border-white/[0.1] dark:bg-white/[0.035] dark:text-fg-dim"
              >
                <Icon name="servers" class="size-4" />
              </div>
              <div class="min-w-0">
                <p class="truncate text-[13px] font-semibold text-black dark:text-fg">{row.server.name}</p>
                <p class="truncate text-[11px] text-neutral-500 dark:text-fg-faint">{row.description}</p>
              </div>
              {@render warning(row.status)}
            </div>
            <div class="text-[11px] font-medium text-neutral-600 dark:text-fg-dim" data-testid="server-status">
              <span>{row.status.label}</span>
            </div>
          </a>
        {/each}
      </div>
    {/if}
  {/if}
</div>
