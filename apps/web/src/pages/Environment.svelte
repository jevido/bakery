<script lang="ts">
  // Coolify's Environment page (resources/views/livewire/project/resource/index.blade.php
  // with its resourceIndex() script, and app/Livewire/Project/Resource/Index.php;
  // Apache-2.0, see NOTICE): every Application, Database and Service of one
  // Environment with search, Filter, Sort, a remembered table/grid switch and
  // client-side pages. Tags are left out until Tags exist.
  //
  // Laid out as Paperclip's ProjectDetail (ui/src/pages/ProjectDetail.tsx,
  // MIT), as the Project page is: its header with the name and actions, the
  // list toolbar, then the Resources as EntityRows in one card or as cards.
  import { buttonVariants } from '$lib/components/ui/button'
  import { Card } from '$lib/components/ui/card'
  import * as Popover from '$lib/components/ui/popover'
  import { api } from '../lib/api'
  import { breadcrumb } from '../lib/breadcrumb.svelte'
  import CollectionToolbar from '../lib/CollectionToolbar.svelte'
  import EntityRow from '../lib/EntityRow.svelte'
  import Icon from '../lib/Icon.svelte'
  import PageHeader from '../lib/PageHeader.svelte'
  import PageSkeleton from '../lib/PageSkeleton.svelte'
  import ProjectTile from '../lib/ProjectTile.svelte'
  import {
    displayDomain,
    firstDomain,
    statusLabel,
    statusState,
    statusTitle,
    statusTone,
    typeLabels,
    type ResourceItem,
  } from '../lib/resources'
  import { applicationPath, databasePath, href, servicePath } from '../lib/router.svelte'
  import { projectAccess } from '../lib/projectAccess.svelte'
  import SearchField from '../lib/SearchField.svelte'
  import SortPopover from '../lib/SortPopover.svelte'
  import type { Database, Deployment, Environment, Server, Service } from '../lib/types'
  import ClientPagination from '../lib/ui/ClientPagination.svelte'
  import Empty from '../lib/ui/Empty.svelte'
  import StatusBadge from '../lib/ui/StatusBadge.svelte'
  import ViewToggle from '../lib/ViewToggle.svelte'

  let { projectId, id }: { projectId: number; id: number } = $props()

  type SortKey = 'name-asc' | 'name-desc' | 'type' | 'status'
  type ViewMode = 'table' | 'grid'
  type FilterKey = 'typeFilters' | 'serverFilters' | 'statusFilters'

  const viewKey = 'bakery.environment-resource-view'
  const pageSizeKey = 'bakery.page-size.environment-resources'
  const sortOptions: { value: SortKey; label: string }[] = [
    { value: 'name-asc', label: 'Name A–Z' },
    { value: 'name-desc', label: 'Name Z–A' },
    { value: 'type', label: 'Resource type' },
    { value: 'status', label: 'Status' },
  ]

  let environment = $state.raw<Environment | null>(null)
  let resources = $state.raw<ResourceItem[]>([])
  let loadError = $state('')

  let searchText = $state('')
  let filters = $state<Record<FilterKey, string[]>>({ typeFilters: [], serverFilters: [], statusFilters: [] })
  let sortBy = $state<SortKey>('name-asc')
  let viewMode = $state<ViewMode>(localStorage.getItem(viewKey) === 'grid' ? 'grid' : 'table')
  let page = $state(1)
  let pageSize = $state(10)

  // The search box filters 150 ms after the last key, as Coolify's debounce does.
  let query = $state('')
  $effect(() => {
    const value = searchText
    const timer = setTimeout(() => (query = value.trim().toLowerCase()), 150)
    return () => clearTimeout(timer)
  })

  async function load(projectId: number, id: number) {
    const [e, d, s, servers] = await Promise.all([
      api<{ environment: Environment }>('GET', `/environments/${id}`),
      api<{ databases: Database[] }>('GET', `/projects/${projectId}/databases`),
      api<{ services: Service[] }>('GET', `/projects/${projectId}/services`),
      // Without the Servers the page still lists everything, on "Unknown".
      api<{ servers: Server[] }>('GET', '/servers')
        .then((r) => r.servers)
        .catch(() => [] as Server[]),
    ])
    // An Application's status is its latest own Deployment's state, as its
    // page shows it; Previews have their own.
    const latest = await Promise.all(
      e.environment.applications.map((a) =>
        api<{ deployments: Deployment[] }>('GET', `/applications/${a.id}/deployments`)
          .then((r) => r.deployments.find((x) => x.preview === 0)?.status ?? '')
          .catch(() => ''),
      ),
    )
    const serverName = (sid?: number) => servers.find((x) => x.id === sid)?.name ?? 'Unknown'
    // Databases and Services run on the Local server.
    const local = servers.find((x) => x.kind === 'local')?.id
    return {
      environment: e.environment,
      resources: [
        ...e.environment.applications.map(
          (a, i): ResourceItem => ({
            key: `application-${a.id}`,
            name: a.name,
            type: 'application',
            typeLabel: typeLabels.application,
            description: '',
            fqdn: a.public_url,
            status: latest[i],
            server: serverName(a.server_id),
            href: href(applicationPath(a)),
          }),
        ),
        ...d.databases
          .filter((x) => x.environment_id === id)
          .map(
            (x): ResourceItem => ({
              key: `database-${x.id}`,
              name: x.name,
              type: 'database',
              typeLabel: typeLabels.database,
              description: '',
              fqdn: '',
              status: x.status,
              server: serverName(local),
              href: href(databasePath(x)),
            }),
          ),
        ...s.services
          .filter((x) => x.environment_id === id)
          .map(
            (x): ResourceItem => ({
              key: `service-${x.id}`,
              name: x.name,
              type: 'service',
              typeLabel: typeLabels.service,
              description: '',
              fqdn: x.components.find((c) => c.public && c.url)?.url ?? '',
              status: x.status,
              server: serverName(local),
              href: href(servicePath(x)),
            }),
          ),
      ],
    }
  }

  $effect(() => {
    environment = null
    resources = []
    loadError = ''
    load(projectId, id)
      .then((r) => {
        environment = r.environment
        resources = r.resources
      })
      .catch((e) => (loadError = e.message))
  })

  function uniqueOptions(options: { value: string; label: string }[]) {
    return [...new Map(options.filter((o) => o.value).map((o) => [o.value, o])).values()].sort((a, b) => a.label.localeCompare(b.label))
  }

  const filterGroups = $derived<{ key: FilterKey; label: string; options: { value: string; label: string }[] }[]>([
    { key: 'typeFilters', label: 'Resource types', options: uniqueOptions(resources.map((r) => ({ value: r.type, label: r.typeLabel }))) },
    { key: 'serverFilters', label: 'Servers', options: uniqueOptions(resources.map((r) => ({ value: r.server, label: r.server }))) },
    {
      key: 'statusFilters',
      label: 'Statuses',
      options: uniqueOptions(resources.map((r) => ({ value: statusState(r), label: statusLabel(r) }))),
    },
  ])
  const activeFilterCount = $derived(filters.typeFilters.length + filters.serverFilters.length + filters.statusFilters.length)
  const filterButtonText = $derived.by(() => {
    const selected = filterGroups.flatMap((g) => g.options.filter((o) => filters[g.key].includes(o.value)).map((o) => o.label))
    if (selected.length === 0) return 'Filter'
    if (selected.length === 1) return selected[0]
    return `${selected[0]} +${selected.length - 1}`
  })

  function toggleFilter(group: FilterKey, value: string) {
    filters[group] = filters[group].includes(value) ? filters[group].filter((v) => v !== value) : [...filters[group], value]
    page = 1
  }

  function clearFilters() {
    filters = { typeFilters: [], serverFilters: [], statusFilters: [] }
    page = 1
  }

  const filtered = $derived.by(() => {
    const items = resources.filter((item) => {
      const matchesType = filters.typeFilters.length === 0 || filters.typeFilters.includes(item.type)
      const matchesServer = filters.serverFilters.length === 0 || filters.serverFilters.includes(item.server)
      const matchesStatus = filters.statusFilters.length === 0 || filters.statusFilters.includes(statusState(item))
      const searchable = [item.name, item.fqdn, item.description, item.typeLabel, item.status, item.server]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
      return matchesType && matchesServer && matchesStatus && (!query || searchable.includes(query))
    })
    return items.sort((a, b) => {
      if (sortBy === 'name-desc') return b.name.localeCompare(a.name)
      if (sortBy === 'type') return a.typeLabel.localeCompare(b.typeLabel) || a.name.localeCompare(b.name)
      if (sortBy === 'status') return statusLabel(a).localeCompare(statusLabel(b)) || a.name.localeCompare(b.name)
      return a.name.localeCompare(b.name)
    })
  })
  const totalPages = $derived(Math.max(1, Math.ceil(filtered.length / pageSize)))
  const paginated = $derived(filtered.slice((Math.min(page, totalPages) - 1) * pageSize, Math.min(page, totalPages) * pageSize))
  $effect(() => {
    if (page > totalPages) page = totalPages
  })

  function setViewMode(mode: ViewMode) {
    viewMode = mode
    page = 1
    localStorage.setItem(viewKey, mode)
  }

  const newHref = $derived(href(`/project/${projectId}/environment/${id}/new`))

  const selectedFilters = $derived(
    filterGroups.flatMap((g) => g.options.filter((o) => filters[g.key].includes(o.value)).map((o) => ({ group: g.key, ...o }))),
  )
  const listCard = 'block gap-0 overflow-hidden py-0'
  const plural = (n: number, word: string) => `${n} ${word}${n === 1 ? '' : 's'}`
  const typeIcons = { application: 'browser-code', database: 'database', service: 'layers' } as const

  const crumbs = $derived(environment ? { project: environment.project_name ?? '', environment: environment.name } : null)
  $effect(() => {
    if (crumbs)
      breadcrumb.set(
        { label: 'Projects', href: href('/projects') },
        { label: crumbs.project, href: href(`/project/${projectId}`) },
        { label: crumbs.environment },
      )
  })
</script>

{#snippet status(item: ResourceItem)}
  <StatusBadge status={statusLabel(item)} type={statusTone(item)} title={statusTitle(item)} />
{/snippet}

{#snippet domain(item: ResourceItem, className: string)}
  <a
    href={firstDomain(item.fqdn)}
    target="_blank"
    rel="noopener noreferrer"
    class={['relative z-10 truncate text-xs text-muted-foreground hover:text-foreground hover:underline', className]}
    title={displayDomain(item.fqdn)}>{displayDomain(item.fqdn)}</a
  >
{/snippet}

{#if loadError}
  <p class="text-sm text-destructive">{loadError}</p>
{:else if !environment}
  <PageSkeleton />
{:else}
  <div class="chrome w-full space-y-4">
    <PageHeader title={environment.name} description={environment.description || undefined}>
      {#snippet leading()}<ProjectTile size="lg" icon="layers" />{/snippet}
      {#snippet meta()}<span>{plural(resources.length, 'resource')} in {environment?.project_name}</span>{/snippet}
      {#snippet actions()}
        {#if projectAccess.can('manage_applications')}
          <a
            href={href(`/project/${projectId}/environment/${id}/edit`)}
            class={buttonVariants({ variant: 'outline', size: 'sm' })}
            title="Environment settings"
            aria-label="Open settings for {environment?.name}"
          >
            <Icon name="settings" class="size-3.5" />
            Settings
          </a>
          <a href={newHref} class={buttonVariants({ size: 'sm' })}>
            <Icon name="plus" class="size-3.5" />
            New resource
          </a>
        {/if}
      {/snippet}
    </PageHeader>

    {#if resources.length === 0}
      <Empty title="No resources yet" description="Add an application, database, or service to this environment." icon="layers">
        {#if projectAccess.can('manage_applications')}
          <a href={newHref} class={buttonVariants({ variant: 'outline', size: 'sm' })}>
            <Icon name="plus" class="size-3.5" />
            Add resource
          </a>
        {/if}
      </Empty>
    {:else}
      <CollectionToolbar ariaLabel="Resources controls">
        {#snippet search()}
          <SearchField bind:value={searchText} label="Search resources" oninput={() => (page = 1)} />
        {/snippet}
        {#snippet controls()}
          <Popover.Root>
            <Popover.Trigger
              class={buttonVariants({ variant: 'ghost', size: 'sm', class: ['max-w-64 text-xs', activeFilterCount > 0 && 'bg-accent'].join(' ') })}
              title={activeFilterCount > 0 ? filterButtonText : 'Filter'}
            >
              <Icon name="filter" class="size-3.5 sm:size-3" />
              <span class="truncate">{activeFilterCount > 0 ? filterButtonText : 'Filter'}</span>
              {#if activeFilterCount > 0}
                <span class="shrink-0 rounded-full bg-muted px-1.5 text-[10px] font-medium text-muted-foreground tabular-nums">{activeFilterCount}</span>
              {/if}
            </Popover.Trigger>
            <Popover.Content align="start" class="w-64 p-0">
              <div class="max-h-80 overflow-y-auto p-2" role="listbox" aria-label="Filter resources" aria-multiselectable="true">
                {#each filterGroups as group (group.key)}
                  {#if group.options.length > 0}
                    <div class="px-2 pt-2 pb-1 text-[10px] font-semibold tracking-wide text-muted-foreground uppercase">{group.label}</div>
                    {#each group.options as option (`${group.key}-${option.value}`)}
                      {@const selected = filters[group.key].includes(option.value)}
                      <button
                        type="button"
                        role="option"
                        aria-selected={selected}
                        class={[
                          'flex w-full items-center justify-between gap-2 rounded-sm px-2 py-1.5 text-sm',
                          selected ? 'bg-accent/50 text-foreground' : 'text-muted-foreground hover:bg-accent/50',
                        ]}
                        onclick={() => toggleFilter(group.key, option.value)}
                      >
                        <span class="min-w-0 flex-1 truncate text-left">{option.label}</span>
                        <span
                          class={[
                            'flex size-4 shrink-0 items-center justify-center rounded-sm border',
                            selected ? 'border-primary bg-primary text-primary-foreground' : 'border-input',
                          ]}
                        >
                          {#if selected}<Icon name="check" class="size-3" />{/if}
                        </span>
                      </button>
                    {/each}
                  {/if}
                {/each}
              </div>
              <div class="border-t border-border p-2">
                <button
                  type="button"
                  class="w-full rounded-sm px-2 py-1.5 text-sm text-muted-foreground hover:bg-accent/50 hover:text-foreground"
                  onclick={clearFilters}>Clear filters</button
                >
              </div>
            </Popover.Content>
          </Popover.Root>
          <SortPopover bind:value={sortBy} options={sortOptions} label="Sort resources" onchange={() => (page = 1)} />
          <ViewToggle value={viewMode} onchange={setViewMode} />
        {/snippet}
        {#snippet feedback()}
          {#if selectedFilters.length > 0}
            <div class="flex flex-wrap items-center gap-1.5">
              {#each selectedFilters as chip (`${chip.group}-${chip.value}`)}
                <span class="inline-flex items-center gap-1 rounded-full border border-border bg-muted/50 py-0.5 pr-1 pl-2 text-xs">
                  {chip.label}
                  <button
                    type="button"
                    class="flex size-4 items-center justify-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground"
                    aria-label="Remove filter {chip.label}"
                    onclick={() => toggleFilter(chip.group, chip.value)}
                  >
                    <Icon name="x" class="size-2.5" />
                  </button>
                </span>
              {/each}
              <button type="button" class="px-1 text-xs text-muted-foreground hover:text-foreground hover:underline" onclick={clearFilters}>
                Clear all
              </button>
            </div>
          {/if}
        {/snippet}
      </CollectionToolbar>

      {#if filtered.length === 0}
        <Card class="block py-0">
          <Empty title="No matching resources" description="Try a different search or filter." icon="search" size="sm" />
        </Card>
      {:else if viewMode === 'table'}
        <Card class={listCard}>
          {#each paginated as item (item.key)}
            <EntityRow title={item.name} subtitle={item.description || displayDomain(item.fqdn) || undefined} reserveSubtitleSpace href={item.href}>
              {#snippet leading()}<ProjectTile size="sm" icon={typeIcons[item.type]} />{/snippet}
              {#snippet trailing()}
                {#if item.fqdn && item.description}{@render domain(item, 'hidden max-w-56 xl:block')}{/if}
                <span class="hidden w-40 truncate text-right text-xs text-muted-foreground lg:inline" title="{item.typeLabel} on {item.server}"
                  >{item.typeLabel} · {item.server}</span
                >
                {@render status(item)}
              {/snippet}
            </EntityRow>
          {/each}
          <ClientPagination bind:page bind:pageSize total={filtered.length} storageKey={pageSizeKey} />
        </Card>
      {:else}
        <div class="space-y-3">
          <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
            {#each paginated as item (item.key)}
              <Card interactive class="relative min-h-28 gap-0 p-4">
                <a href={item.href} class="absolute inset-0 rounded-lg" aria-label="Open {item.name}"></a>
                <div class="flex items-start gap-3">
                  <ProjectTile size="lg" icon={typeIcons[item.type]} />
                  <div class="min-w-0 flex-1">
                    <h2 class="truncate text-sm font-medium" title={item.name}>{item.name}</h2>
                    <p class="mt-0.5 truncate text-xs text-muted-foreground">{item.typeLabel} · {item.server}</p>
                  </div>
                  {@render status(item)}
                </div>
                <div class="mt-auto flex min-w-0 flex-col gap-0.5 pt-4">
                  <p class="min-h-4 truncate text-xs text-muted-foreground">{item.description}</p>
                  {#if item.fqdn}{@render domain(item, 'max-w-full self-start')}{/if}
                </div>
              </Card>
            {/each}
          </div>
          <Card class={listCard}>
            <ClientPagination bind:page bind:pageSize total={filtered.length} storageKey={pageSizeKey} />
          </Card>
        </div>
      {/if}
    {/if}
  </div>
{/if}
