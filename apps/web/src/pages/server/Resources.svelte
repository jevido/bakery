<script lang="ts">
  // Coolify's Server Resources page (resources/views/livewire/server/resources.blade.php,
  // app/Livewire/Server/Resources.php; Apache-2.0, see NOTICE), the Managed
  // tab only: every Application, Database and Service on this Server with its
  // Project, Environment, type and status, searchable and paged.
  import Icon from '../../lib/Icon.svelte'
  import { statusLabel, statusTitle, statusTone, typeLabels } from '../../lib/resources'
  import type { Server } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import ClientPagination from '../../lib/ui/ClientPagination.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import SettingsSection from '../../lib/ui/SettingsSection.svelte'
  import Spinner from '../../lib/ui/Spinner.svelte'
  import StatusBadge from '../../lib/ui/StatusBadge.svelte'
  import { serverResources, type Row } from './resources'

  let { server }: { server: Server } = $props()

  let rows = $state.raw<Row[] | null>(null)
  let loadError = $state('')
  let refreshing = $state(false)
  let search = $state('')
  let page = $state(1)
  let pageSize = $state(10)

  async function refresh() {
    refreshing = true
    try {
      rows = await serverResources(server)
      loadError = ''
    } catch (e) {
      loadError = e instanceof Error ? e.message : String(e)
    } finally {
      refreshing = false
    }
  }

  const serverId = $derived(server.id)
  $effect(() => {
    void serverId
    rows = null
    refresh()
  })

  // Coolify's search is debounced by 300 ms and matches the name.
  let query = $state('')
  $effect(() => {
    const value = search
    const timer = setTimeout(() => (query = value.trim().toLowerCase()), 300)
    return () => clearTimeout(timer)
  })

  const filtered = $derived((rows ?? []).filter((r) => !query || r.name.toLowerCase().includes(query)))
  const totalPages = $derived(Math.max(1, Math.ceil(filtered.length / pageSize)))
  const paginated = $derived(filtered.slice((Math.min(page, totalPages) - 1) * pageSize, Math.min(page, totalPages) * pageSize))
  $effect(() => {
    if (page > totalPages) page = totalPages
  })
</script>

<div class="chrome application-settings-form w-full min-w-0">
  <SettingsSection
    id="server-resources-section"
    title="Resources"
    helper="Review The Bakery's resources running on this server."
    flush
  >
    {#snippet actions()}
      <Button loading={refreshing} onclick={refresh} data-testid="refresh-resources">
        {#if !refreshing}<Icon name="refresh" class="size-3.5" />{/if}
        Refresh
      </Button>
    {/snippet}

    <div class="border-b border-neutral-200 p-3 dark:border-white/[0.08]">
      <div class="relative w-full max-w-sm">
        <Icon
          name="search"
          class="pointer-events-none absolute top-1/2 left-2.5 z-10 size-3.5 -translate-y-1/2 text-neutral-400 dark:text-fg-faint"
        />
        <input
          bind:value={search}
          oninput={() => (page = 1)}
          type="search"
          placeholder="Search resources by name"
          aria-label="Search resources by name"
          class="input h-8! w-full rounded-lg! border-neutral-200! bg-white! py-0! pr-8! pl-8! text-[12px]! shadow-none! placeholder:text-neutral-400 focus:border-accent! focus:ring-0! dark:border-white/[0.08]! dark:bg-white/[0.035]! dark:text-fg! dark:placeholder:text-fg-faint"
        />
        {#if search}
          <button
            type="button"
            onclick={() => {
              search = ''
              page = 1
            }}
            class="absolute top-1/2 right-2 flex size-5 -translate-y-1/2 items-center justify-center rounded text-neutral-400 transition-colors hover:bg-neutral-100 hover:text-black dark:text-fg-faint dark:hover:bg-white/[0.07] dark:hover:text-fg"
            aria-label="Clear search"
          >
            <Icon name="x" class="size-3" />
          </button>
        {/if}
      </div>
    </div>

    {#if loadError}
      <p class="p-4 text-sm text-error">{loadError}</p>
    {:else if rows === null}
      <div class="p-6"><Spinner text="Loading resources…" /></div>
    {:else if filtered.length === 0}
      <div class="p-6" data-testid="resources-empty">
        <Empty
          size="sm"
          title={query ? 'No matching resources' : 'No managed resources'}
          description={query ? 'Try another name or clear the search.' : 'Resources assigned to this server will appear here.'}
          icon="projects"
        />
      </div>
    {:else}
      <div class="data-table" data-testid="server-resources">
        <div class="data-table-header server-resources-managed-table-grid">
          <span>Name</span>
          <span>Project</span>
          <span>Environment</span>
          <span>Type</span>
          <span>Status</span>
        </div>
        {#each paginated as r (r.key)}
          <div
            class="data-table-row server-resources-managed-table-grid border-b border-neutral-200 last:border-b-0 dark:border-white/[0.08]"
            data-testid="server-resource"
          >
            <div class="min-w-0">
              <a class="block max-w-full truncate text-[12px] font-medium text-neutral-950 hover:underline dark:text-fg" href={r.href}>
                {r.name}
              </a>
            </div>
            <div class="truncate text-[11px] text-neutral-600 dark:text-fg-dim">{r.project}</div>
            <div class="truncate text-[11px] text-neutral-600 dark:text-fg-dim">{r.environment}</div>
            <div class="text-[11px] text-neutral-600 dark:text-fg-dim">{typeLabels[r.type]}</div>
            <div><StatusBadge status={statusLabel(r)} type={statusTone(r)} title={statusTitle(r)} /></div>
          </div>
        {/each}
      </div>
      <ClientPagination bind:page bind:pageSize total={filtered.length} storageKey="bakery.page-size.server-resources" />
    {/if}
  </SettingsSection>
</div>
