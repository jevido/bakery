<script lang="ts">
  // Coolify's Servers page (resources/views/livewire/server/index.blade.php,
  // Apache-2.0, see NOTICE): "New server", search, a remembered table/grid
  // switch and each Server with its description and status.
  //
  // Laid out as Paperclip's Projects page (ui/src/pages/Projects.tsx, MIT):
  // a CollectionToolbar, and the Servers as EntityRows in one bordered card,
  // or as cards in the grid view.
  //
  // Left out: "Import transfer" (dev only in Coolify, no Transfer here) and the
  // grid card's metrics chart (Coolify draws it only with Sentinel metrics).
  import { buttonVariants } from '$lib/components/ui/button'
  import { Card } from '$lib/components/ui/card'
  import { api } from '../../lib/api'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import CollectionToolbar from '../../lib/CollectionToolbar.svelte'
  import EntityRow from '../../lib/EntityRow.svelte'
  import Icon from '../../lib/Icon.svelte'
  import PageHeader from '../../lib/PageHeader.svelte'
  import PageSkeleton from '../../lib/PageSkeleton.svelte'
  import ProjectTile from '../../lib/ProjectTile.svelte'
  import { href, serverPath } from '../../lib/router.svelte'
  import SearchField from '../../lib/SearchField.svelte'
  import { session } from '../../lib/session.svelte'
  import type { Server } from '../../lib/types'
  import Empty from '../../lib/ui/Empty.svelte'
  import StatusBadge from '../../lib/ui/StatusBadge.svelte'
  import ViewToggle from '../../lib/ViewToggle.svelte'
  import { serverStatus } from './status'

  type ViewMode = 'table' | 'grid'
  const viewKey = 'bakery-servers-view'

  let servers = $state.raw<Server[] | null>(null)
  let loadError = $state('')
  let searchText = $state('')
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
    const value = searchText
    const timer = setTimeout(() => (query = value.trim().toLowerCase()), 150)
    return () => clearTimeout(timer)
  })

  const rows = $derived((servers ?? []).map((s) => ({ server: s, status: serverStatus(s) })))
  const filtered = $derived(
    rows.filter((r) => !query || [r.server.name, r.server.description, r.status.label].some((v) => v.toLowerCase().includes(query))),
  )

  function setViewMode(mode: ViewMode) {
    viewMode = mode
    localStorage.setItem(viewKey, mode)
  }

  $effect(() => breadcrumb.set({ label: 'Servers' }))
</script>

{#snippet warning(status: { label: string; type: string; detail: string })}
  {#if status.type !== 'success'}
    <span
      title={status.detail || status.label}
      aria-label="Server status: {status.label}"
      class="flex size-6 shrink-0 items-center justify-center rounded-md text-destructive"
    >
      <Icon name="alert-triangle" class="size-4" />
    </span>
  {/if}
{/snippet}

<div class="chrome w-full space-y-4">
  <PageHeader title="Servers">
    {#snippet actions()}
      {#if session.can('manage_servers')}
        <a href={href('/servers/new')} class={buttonVariants({ variant: 'outline', size: 'sm' })}>
          <Icon name="plus" class="size-3.5" />
          New server
        </a>
      {/if}
    {/snippet}
  </PageHeader>

  {#if loadError}
    <p class="text-sm text-destructive">{loadError}</p>
  {:else if servers === null}
    <PageSkeleton />
  {:else if servers.length === 0}
    <Empty title="No servers yet" description="Add a server to deploy applications, databases, and services." icon="servers" />
  {:else}
    <CollectionToolbar ariaLabel="Servers controls">
      {#snippet search()}
        <SearchField bind:value={searchText} label="Search servers" />
      {/snippet}
      {#snippet controls()}
        <span class="text-xs text-muted-foreground tabular-nums" data-testid="servers-count">
          {filtered.length}
          {filtered.length === 1 ? 'server' : 'servers'}
        </span>
        <ViewToggle value={viewMode} onchange={setViewMode} />
      {/snippet}
    </CollectionToolbar>

    {#if filtered.length === 0}
      <Card class="block py-0">
        <Empty title="No matching servers" description="Try a different search." icon="search" size="sm" />
      </Card>
    {:else if viewMode === 'grid'}
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
        {#each filtered as row (row.server.id)}
          <Card interactive class="relative min-h-28 gap-0 p-4" data-testid="server">
            <a href={href(serverPath(row.server.id))} class="absolute inset-0 rounded-lg" aria-label="Open {row.server.name}"></a>
            <div class="flex items-start gap-3">
              <ProjectTile size="lg" icon="servers" />
              <div class="min-w-0 flex-1">
                <h2 class="truncate text-sm font-medium" title={row.server.name}>{row.server.name}</h2>
                <p class="mt-0.5 min-h-4 truncate text-xs text-muted-foreground">{row.server.description}</p>
              </div>
              {@render warning(row.status)}
            </div>
            <div class="relative z-10 mt-auto pt-4">
              <span data-testid="server-status"><StatusBadge label={row.status.label} type={row.status.type} /></span>
            </div>
          </Card>
        {/each}
      </div>
    {:else}
      <Card class="block gap-0 overflow-hidden py-0">
        {#each filtered as row (row.server.id)}
          <EntityRow
            title={row.server.name}
            subtitle={row.server.description || undefined}
            reserveSubtitleSpace
            href={href(serverPath(row.server.id))}
            data-testid="server"
          >
            {#snippet leading()}<ProjectTile size="sm" icon="servers" />{/snippet}
            {#snippet trailing()}
              {@render warning(row.status)}
              <span data-testid="server-status"><StatusBadge label={row.status.label} type={row.status.type} /></span>
            {/snippet}
          </EntityRow>
        {/each}
      </Card>
    {/if}
  {/if}
</div>
