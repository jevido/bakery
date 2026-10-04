<script lang="ts">
  // Coolify's Environment page (resources/views/livewire/project/resource/index.blade.php
  // with its resourceIndex() script, and app/Livewire/Project/Resource/Index.php;
  // Apache-2.0, see NOTICE): every Application, Database and Service of one
  // Environment with search, Filter, Sort, a remembered table/grid switch and
  // client-side pages. Tags are left out until Tags exist.
  import { api } from '../lib/api'
  import { breadcrumb } from '../lib/breadcrumb.svelte'
  import Icon from '../lib/Icon.svelte'
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
  import { applicationPath, href } from '../lib/router.svelte'
  import { session } from '../lib/session.svelte'
  import type { Database, Deployment, Environment, Server, Service } from '../lib/types'
  import ClientPagination from '../lib/ui/ClientPagination.svelte'
  import Empty from '../lib/ui/Empty.svelte'
  import Spinner from '../lib/ui/Spinner.svelte'
  import StatusBadge from '../lib/ui/StatusBadge.svelte'
  import TableDropdown from '../lib/ui/TableDropdown.svelte'

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

  let search = $state('')
  let filters = $state<Record<FilterKey, string[]>>({ typeFilters: [], serverFilters: [], statusFilters: [] })
  let sortBy = $state<SortKey>('name-asc')
  let viewMode = $state<ViewMode>(localStorage.getItem(viewKey) === 'grid' ? 'grid' : 'table')
  let page = $state(1)
  let pageSize = $state(10)

  // The search box filters 150 ms after the last key, as Coolify's debounce does.
  let query = $state('')
  $effect(() => {
    const value = search
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
              href: href(`/databases/${x.id}`),
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
              href: href(`/services/${x.id}`),
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

  const toggleIdle =
    'text-neutral-400 hover:bg-neutral-100 hover:text-black dark:text-fg-faint dark:hover:bg-white/[0.06] dark:hover:text-fg'
  const iconBox =
    'flex size-8 shrink-0 items-center justify-center rounded-lg border border-neutral-200 bg-neutral-50 text-neutral-500 dark:border-white/[0.08] dark:text-fg-dim'
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

{#snippet noMatches(boxed: boolean)}
  <div
    class={[
      'flex min-h-52 flex-col items-center justify-center px-6 text-center',
      boxed && 'rounded-xl border border-neutral-200 bg-white dark:border-white/[0.08] dark:bg-white/[0.05]',
    ]}
  >
    <Icon name="search" class="mb-3 size-6 text-neutral-300 dark:text-fg-faint" />
    <p class="text-[13px] font-medium">No matching resources</p>
    <p class="mt-1 text-[12px] text-neutral-500 dark:text-fg-dim">Try a different search or filter.</p>
  </div>
{/snippet}

{#if loadError}
  <p class="text-sm text-error">{loadError}</p>
{:else if !environment}
  <Spinner text="Loading…" />
{:else}
  <div class="chrome w-full">
    <header class="mb-5 flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div class="min-w-0">
        <h1 class="truncate text-[24px]! leading-7! font-semibold! tracking-tight!">{environment.name}</h1>
        <p class="mt-1 text-[13px] text-neutral-500 dark:text-fg-dim">
          <span>{resources.length} {resources.length === 1 ? 'resource' : 'resources'}</span>
          in {environment.project_name}
        </p>
      </div>
      {#if session.canWrite}
        <div class="flex w-fit shrink-0 items-center gap-2">
          <a
            href={href(`/project/${projectId}/environment/${id}/edit`)}
            class="button whitespace-nowrap"
            title="Environment settings"
            aria-label="Open settings for {environment.name}"
          >
            <Icon name="settings" class="size-3.5" />
            Settings
          </a>
          <a href={newHref} class="button button-highlighted whitespace-nowrap">
            <Icon name="plus" class="size-3.5" />
            New resource
          </a>
        </div>
      {/if}
    </header>

    {#if resources.length === 0}
      <Empty title="No resources yet" description="Add an application, database, or service to this environment." icon="layers">
        {#if session.canWrite}
          <a href={newHref} class="button">
            <Icon name="plus" class="size-3.5" />
            Add resource
          </a>
        {/if}
      </Empty>
    {:else}
      <div class="mb-3 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div class="relative w-full sm:max-w-sm">
          <Icon
            name="search"
            class="pointer-events-none absolute top-1/2 left-2.5 z-10 size-3.5 -translate-y-1/2 text-neutral-400 dark:text-fg-faint"
          />
          <input
            bind:value={search}
            oninput={() => (page = 1)}
            type="search"
            placeholder="Search resources"
            aria-label="Search resources"
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
              <span class="text-sm leading-none">×</span>
            </button>
          {/if}
        </div>

        <div class="flex items-center gap-2">
          <TableDropdown panelClass="w-64! overflow-hidden! p-0!">
            {#snippet trigger({ open, toggle })}
              <button
                type="button"
                class={['button max-w-64', activeFilterCount > 0 && 'button-highlighted']}
                title={activeFilterCount > 0 ? filterButtonText : 'Filter'}
                aria-haspopup="listbox"
                aria-expanded={open}
                onclick={toggle}
              >
                <svg class="size-3.5 opacity-65" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                  <path d="M4 6h16M7 12h10M10 18h4" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" />
                </svg>
                <span class="truncate">{activeFilterCount > 0 ? filterButtonText : 'Filter'}</span>
                {#if activeFilterCount > 0}
                  <span
                    class="shrink-0 rounded-full bg-neutral-100 px-1.5 py-0.5 text-[10px] font-medium text-neutral-500 dark:bg-white/[0.07] dark:text-fg-dim"
                    >{activeFilterCount}</span
                  >
                {/if}
              </button>
            {/snippet}
            {#snippet children()}
              <div class="max-h-80 overflow-y-auto p-1" aria-multiselectable="true">
                {#each filterGroups as group (group.key)}
                  {#if group.options.length > 0}
                    <div>
                      <div class="px-2 pt-2 pb-1 text-[10px] font-semibold tracking-wide text-neutral-400 uppercase dark:text-fg-faint">
                        {group.label}
                      </div>
                      {#each group.options as option (`${group.key}-${option.value}`)}
                        {@const selected = filters[group.key].includes(option.value)}
                        <button
                          type="button"
                          role="option"
                          aria-selected={selected}
                          class="listbox-option"
                          onclick={() => toggleFilter(group.key, option.value)}
                        >
                          <span class="min-w-0 flex-1 truncate">{option.label}</span>
                          <span
                            class={[
                              'flex size-4 shrink-0 items-center justify-center rounded-[5px] border',
                              selected
                                ? 'border-coollabs bg-coollabs text-white dark:border-warning dark:bg-warning dark:text-black'
                                : 'border-neutral-300 bg-white dark:border-white/[0.14] dark:bg-white/[0.045]',
                            ]}
                          >
                            {#if selected}
                              <svg class="size-3" viewBox="0 0 12 12" fill="none" aria-hidden="true">
                                <path
                                  d="m2.25 6.15 2.35 2.3 5.15-5"
                                  stroke="currentColor"
                                  stroke-width="1.8"
                                  stroke-linecap="round"
                                  stroke-linejoin="round"
                                />
                              </svg>
                            {/if}
                          </span>
                        </button>
                      {/each}
                    </div>
                  {/if}
                {/each}
              </div>
              <div class="border-t border-neutral-200 bg-white p-1 dark:border-white/10 dark:bg-raised">
                <button type="button" class="listbox-option justify-center! text-center!" onclick={clearFilters}>Clear filters</button>
              </div>
            {/snippet}
          </TableDropdown>

          <TableDropdown panelClass="w-48!">
            {#snippet trigger({ open, toggle })}
              <button type="button" class="button" aria-haspopup="listbox" aria-expanded={open} onclick={toggle}>
                <svg class="size-3.5 opacity-65" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                  <path
                    d="M8 5v14m0 0-3-3m3 3 3-3M16 19V5m0 0-3 3m3-3 3 3"
                    stroke="currentColor"
                    stroke-width="1.7"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  />
                </svg>
                Sort
              </button>
            {/snippet}
            {#snippet children(close)}
              {#each sortOptions as option (option.value)}
                <button
                  type="button"
                  role="option"
                  aria-selected={sortBy === option.value}
                  class="flex h-9 w-full items-center rounded-md px-2 text-left text-[12px] text-neutral-600 transition-colors hover:bg-neutral-100 hover:text-black dark:text-fg-dim dark:hover:bg-white/[0.06] dark:hover:text-fg"
                  onclick={() => {
                    sortBy = option.value
                    close()
                    page = 1
                  }}
                >
                  <span class="flex-1">{option.label}</span>
                  {#if sortBy === option.value}
                    <svg class="size-3.5 text-warning" viewBox="0 0 12 12" fill="none" aria-hidden="true">
                      <path d="m2.5 6.25 2.1 2.1 4.9-5" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round" />
                    </svg>
                  {/if}
                </button>
              {/each}
            {/snippet}
          </TableDropdown>

          <div class="view-toggle">
            <button
              type="button"
              onclick={() => setViewMode('table')}
              class={['flex size-7.5 items-center justify-center rounded-md transition-colors', viewMode === 'table' ? 'control-selected' : toggleIdle]}
              aria-label="Table view"
              aria-pressed={viewMode === 'table'}
              title="Table view"
            >
              <Icon name="unordered-list" class="size-3.5" />
            </button>
            <button
              type="button"
              onclick={() => setViewMode('grid')}
              class={['flex size-7.5 items-center justify-center rounded-md transition-colors', viewMode === 'grid' ? 'control-selected' : toggleIdle]}
              aria-label="Grid view"
              aria-pressed={viewMode === 'grid'}
              title="Grid view"
            >
              <Icon name="grid" class="size-3.5" />
            </button>
          </div>
        </div>
      </div>

      {#if viewMode === 'table'}
        <div class="overflow-hidden rounded-xl border border-neutral-200 bg-white shadow-sm dark:border-white/[0.08] dark:bg-white/[0.05]">
          <div
            class="environment-resource-grid border-b border-neutral-200 bg-neutral-50 px-4 py-2.5 text-[11px] font-medium text-neutral-500 dark:border-white/[0.08] dark:bg-white/[0.05] dark:text-fg-faint"
          >
            <div>Resource</div>
            <div class="resource-type">Type</div>
            <div>Status</div>
            <div class="resource-domain">Domain</div>
            <div class="resource-server">Server</div>
          </div>

          {#each paginated as item (item.key)}
            <div
              class="environment-resource-grid group relative min-h-14 items-center border-b border-neutral-200 px-4 py-2.5 transition-colors last:border-b-0 hover:bg-neutral-50 dark:border-white/[0.07] dark:hover:bg-white/[0.025]"
            >
              <a href={item.href} class="absolute inset-0" aria-label="Open {item.name}"></a>
              <div class="flex min-w-0 items-center gap-3">
                <div class={[iconBox, 'dark:bg-white/[0.035]']}>
                  <Icon name={typeIcons[item.type]} class="size-4" />
                </div>
                <div class="min-w-0">
                  <div class="flex min-w-0 items-center gap-1.5">
                    <a href={item.href} class="relative block truncate text-[13px] font-semibold text-black hover:underline dark:text-fg">{item.name}</a>
                  </div>
                  <p class="min-h-4 truncate text-[11px] text-neutral-500 dark:text-fg-faint">
                    {#if item.description}<span>{item.description}</span>{/if}
                  </p>
                  <div class="mobile-resource-domain min-w-0">
                    {#if item.fqdn}
                      <a
                        href={firstDomain(item.fqdn)}
                        target="_blank"
                        rel="noopener noreferrer"
                        class="relative z-10 block truncate text-[11px] text-neutral-500 hover:underline dark:text-fg-dim">{displayDomain(item.fqdn)}</a
                      >
                    {/if}
                  </div>
                </div>
              </div>

              <div class="resource-type truncate text-[12px] text-neutral-600 dark:text-fg-dim">{item.typeLabel}</div>

              <div>{@render status(item)}</div>

              <div class="resource-domain min-w-0">
                {#if item.fqdn}
                  <a
                    href={firstDomain(item.fqdn)}
                    target="_blank"
                    rel="noopener noreferrer"
                    class="relative inline-block max-w-full truncate align-middle text-[12px] text-neutral-600 hover:underline dark:text-fg-dim"
                    >{displayDomain(item.fqdn)}</a
                  >
                {:else}
                  <span class="text-[12px] text-neutral-400 dark:text-fg-faint">-</span>
                {/if}
              </div>

              <div class="resource-server truncate text-[12px] text-neutral-600 dark:text-fg-dim">{item.server}</div>
            </div>
          {/each}

          {#if filtered.length === 0}
            {@render noMatches(false)}
          {:else}
            <ClientPagination bind:page bind:pageSize total={filtered.length} storageKey={pageSizeKey} />
          {/if}
        </div>
      {:else}
        <div>
          <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
            {#each paginated as item (item.key)}
              <article
                class="group relative flex min-h-28 flex-col rounded-xl border border-neutral-200 bg-white p-3 shadow-sm transition-all hover:-translate-y-px hover:border-neutral-300 hover:shadow-md dark:border-white/[0.08] dark:bg-white/[0.05] dark:hover:border-white/[0.14]"
              >
                <a href={item.href} class="absolute inset-0 rounded-xl" aria-label="Open {item.name}"></a>

                <div class="flex items-start gap-3">
                  <div class={[iconBox, 'dark:bg-white/[0.04]']}>
                    <Icon name={typeIcons[item.type]} class="size-4" />
                  </div>
                  <div class="min-w-0 flex-1">
                    <div class="flex min-w-0 items-center gap-1.5">
                      <h2 class="truncate text-[13px]! leading-4! font-semibold! text-black dark:text-fg">{item.name}</h2>
                    </div>
                    <p class="mt-0.5 text-[11px] text-neutral-500 dark:text-fg-faint">{item.typeLabel}</p>
                  </div>
                  {@render status(item)}
                </div>
                <div class="mt-auto flex min-w-0 flex-col gap-0.5 pt-4">
                  <p class="min-h-4 truncate text-[11px] text-neutral-500 dark:text-fg-faint">
                    {#if item.description}<span>{item.description}</span>{/if}
                  </p>
                  {#if item.fqdn}
                    <a
                      href={firstDomain(item.fqdn)}
                      target="_blank"
                      rel="noopener noreferrer"
                      class="relative z-10 max-w-full self-start truncate text-[11px] text-neutral-500 hover:underline dark:text-fg-dim"
                      title={displayDomain(item.fqdn)}>{displayDomain(item.fqdn)}</a
                    >
                  {/if}
                </div>
              </article>
            {/each}
          </div>

          {#if filtered.length === 0}
            {@render noMatches(true)}
          {:else}
            <ClientPagination
              bind:page
              bind:pageSize
              total={filtered.length}
              storageKey={pageSizeKey}
              class="mt-3 rounded-xl border border-neutral-200 bg-white shadow-sm dark:border-white/[0.08] dark:bg-white/[0.05]"
            />
          {/if}
        </div>
      {/if}
    {/if}
  </div>
{/if}
