<script lang="ts">
  // Coolify's Server Resources page (resources/views/livewire/server/resources.blade.php,
  // app/Livewire/Server/Resources.php; Apache-2.0, see NOTICE), the Managed
  // tab only: every Application, Database and Service on this Server with its
  // Project, Environment, type and status, searchable and paged.
  import { untrack } from 'svelte'
  import Icon from '../../lib/Icon.svelte'
  import SearchField from '../../lib/SearchField.svelte'
  import SettingsGroup from '../../lib/settings/SettingsGroup.svelte'
  import { statusLabel, statusTitle, statusTone, typeLabels } from '../../lib/resources'
  import type { Server } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import ClientPagination from '../../lib/ui/ClientPagination.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import Spinner from '../../lib/ui/Spinner.svelte'
  import StatusBadge from '@bakery/ui/StatusBadge.svelte'
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

  // Only a new Server reloads the list: the frame reads the Server again
  // every 10 s, and refresh() reads the prop, which would track it.
  const serverId = $derived(server.id)
  $effect(() => {
    void serverId
    rows = null
    untrack(refresh)
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

<SettingsGroup id="server-resources-section" label="Resources" hint="Review The Bakery's resources running on this server." wide>
  {#snippet actions()}
    <Button loading={refreshing} onclick={refresh} data-testid="refresh-resources">
      {#if !refreshing}<Icon name="refresh" class="size-3.5" />{/if}
      Refresh
    </Button>
  {/snippet}

  <SearchField bind:value={search} label="Search resources by name" oninput={() => (page = 1)} />

  <!-- Dense rows as in database/BackupExecutions.svelte: the name and status
       always, Project, Environment and type beside them where there is room. -->
  <div class="overflow-hidden rounded-md border border-border">
    {#if loadError}
      <p class="p-4 text-sm text-destructive">{loadError}</p>
    {:else if rows === null}
      <div class="p-4"><Spinner text="Loading resources…" /></div>
    {:else if filtered.length === 0}
      <div class="p-4" data-testid="resources-empty">
        <Empty
          size="sm"
          title={query ? 'No matching resources' : 'No managed resources'}
          description={query ? 'Try another name or clear the search.' : 'Resources assigned to this server will appear here.'}
          icon="projects"
        />
      </div>
    {:else}
      <div data-testid="server-resources">
        {#each paginated as r (r.key)}
          <div class="flex items-center gap-3 border-b border-border px-4 py-2.5 last:border-b-0" data-testid="server-resource">
            <a class="min-w-0 flex-1 truncate text-sm font-medium text-foreground hover:underline" href={r.href}>{r.name}</a>
            <span class="hidden w-32 truncate text-xs text-muted-foreground sm:block" title="Project">{r.project}</span>
            <span class="hidden w-32 truncate text-xs text-muted-foreground lg:block" title="Environment">{r.environment}</span>
            <span class="hidden w-24 text-xs text-muted-foreground md:block">{typeLabels[r.type]}</span>
            <StatusBadge status={statusLabel(r)} type={statusTone(r)} title={statusTitle(r)} />
          </div>
        {/each}
      </div>
    {/if}
  </div>
  {#if rows && filtered.length > 0}
    <ClientPagination bind:page bind:pageSize total={filtered.length} storageKey="bakery.page-size.server-resources" />
  {/if}
</SettingsGroup>
