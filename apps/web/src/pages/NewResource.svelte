<script lang="ts">
  // Coolify's New Resource page (resources/views/livewire/project/new/select.blade.php
  // with its searchResources() script, app/Livewire/Project/New/Select.php, and
  // project/resource/create.blade.php with Resource/Create.php for ?type=…&server=…;
  // Apache-2.0, see NOTICE): search, the resource type Filter and the category
  // list over Applications, Databases and the Services catalog. A Database or
  // Service card creates it at once; an Application card asks for the Server
  // when more than one is ready, then opens its create page.
  //
  // Left out until they exist: the GitHub App, GitLab App and Dockerfile
  // cards, KeyDB, Dragonfly and ClickHouse, Destinations, build servers, the
  // PostgreSQL image picker and "Connect an existing PostgreSQL database".
  import { api, ApiError } from '../lib/api'
  import ApplicationForm from '../lib/ApplicationForm.svelte'
  import { breadcrumb } from '../lib/breadcrumb.svelte'
  import Icon from '../lib/Icon.svelte'
  import { go, href } from '../lib/router.svelte'
  import ServiceForm from '../lib/ServiceForm.svelte'
  import { session } from '../lib/session.svelte'
  import type {
    Application,
    ApplicationInput,
    BuildPack,
    Database,
    DatabaseType,
    Environment,
    Server,
    Service,
    ServiceInput,
    ServiceTemplate,
  } from '../lib/types'
  import Callout from '../lib/ui/Callout.svelte'
  import Empty from '../lib/ui/Empty.svelte'
  import Spinner from '../lib/ui/Spinner.svelte'
  import StatusBadge from '../lib/ui/StatusBadge.svelte'
  import TableDropdown from '../lib/ui/TableDropdown.svelte'
  import { toast } from '../lib/ui/toast.svelte'

  let {
    projectId,
    id,
    type,
    server,
  }: { projectId: number; id: number; type: string; server: number | null } = $props()

  type ResourceType = 'all' | 'applications' | 'databases' | 'services'
  type Card = { id: string; name: string; description: string; logo: string; docs: string; website?: string }
  type ApplicationCard = Card & { source: 'Git source' | 'Docker source'; buildPack: BuildPack }

  // Select.php's cards, in its order, with what The Bakery can create.
  const gitBasedApplications: ApplicationCard[] = [
    {
      id: 'public',
      name: 'Public Git Repository',
      description: 'Deploy any public Git repository. The Bakery builds it from source, no credentials required.',
      logo: 'svgs/resources/public-repo.svg',
      docs: 'https://git-scm.com/docs',
      source: 'Git source',
      buildPack: 'dockerfile',
    },
    {
      id: 'private-deploy-key',
      name: 'Private Git Repository (with Deploy Key)',
      description:
        'Deploy a private repository over SSH with a repository-scoped deploy key. Set the URL and branch manually, no automatic deploys.',
      logo: 'svgs/resources/deploy-key.svg',
      docs: 'https://docs.github.com/en/authentication/connecting-to-github-with-ssh/managing-deploy-keys',
      source: 'Git source',
      buildPack: 'dockerfile',
    },
  ]
  const dockerBasedApplications: ApplicationCard[] = [
    {
      id: 'docker-compose-empty',
      name: 'Docker Compose',
      description: 'Deploy a multi-container application using a Docker Compose file, without a Git repository.',
      logo: 'svgs/resources/docker-compose.svg',
      docs: 'https://docs.podman.io/en/latest/markdown/podman-compose.1.html',
      source: 'Docker source',
      buildPack: 'dockerimage',
    },
    {
      id: 'docker-image',
      name: 'Docker Image',
      description: 'Deploy an application using a prebuilt image from any Docker registry, without a Git repository.',
      logo: 'svgs/resources/docker-image.svg',
      docs: 'https://docs.podman.io/en/latest/markdown/podman-pull.1.html',
      source: 'Docker source',
      buildPack: 'dockerimage',
    },
  ]
  const databases: (Card & { id: DatabaseType })[] = [
    {
      id: 'postgresql',
      name: 'PostgreSQL',
      description: 'A relational database with strong SQL standards support and extensibility.',
      logo: 'svgs/resources/postgres.svg',
      docs: 'https://www.postgresql.org/docs/',
      website: 'https://www.postgresql.org/',
    },
    {
      id: 'mysql',
      name: 'MySQL',
      description: 'A relational database for web and general-purpose applications.',
      logo: 'svgs/resources/mysql.svg',
      docs: 'https://dev.mysql.com/doc/',
      website: 'https://www.mysql.com/',
    },
    {
      id: 'mariadb',
      name: 'MariaDB',
      description: 'A relational database and drop-in replacement for MySQL.',
      logo: 'svgs/resources/mariadb.svg',
      docs: 'https://mariadb.com/kb/en/documentation/',
      website: 'https://mariadb.org/',
    },
    {
      id: 'redis',
      name: 'Redis',
      description: 'An in-memory key-value store used as a database, cache, and message broker.',
      logo: 'svgs/resources/redis.svg',
      docs: 'https://redis.io/docs/latest/',
      website: 'https://redis.io/',
    },
    {
      id: 'valkey',
      name: 'Valkey',
      description: 'An open source, Redis-compatible in-memory key-value store.',
      logo: 'svgs/resources/valkey.svg',
      docs: 'https://valkey.io/docs/',
      website: 'https://valkey.io/',
    },
    {
      id: 'mongodb',
      name: 'MongoDB',
      description: 'A document-oriented NoSQL database that stores JSON-like documents.',
      logo: 'svgs/resources/mongodb.svg',
      docs: 'https://www.mongodb.com/docs/',
      website: 'https://www.mongodb.com/',
    },
  ]
  const resourceTypeOptions: { value: ResourceType; label: string }[] = [
    { value: 'all', label: 'All resources' },
    { value: 'applications', label: 'Applications' },
    { value: 'databases', label: 'Databases' },
    { value: 'services', label: 'Services' },
  ]
  // Select.php upper-cases these when they are a category.
  const acronyms = new Set(['ai', 'api', 'ci', 'cd', 'cms', 'crm', 'erp', 'iot', 'vpn', 'dns', 'ssl', 'sql', 'ui', 'ux', 'cdn', 'cli'])
  const applicationCards = [...gitBasedApplications, ...dockerBasedApplications]

  let environment = $state.raw<Environment | null>(null)
  let servers = $state.raw<Server[]>([])
  let templates = $state.raw<ServiceTemplate[]>([])
  let loadError = $state('')
  let loading = $state(true)

  let search = $state('')
  let resourceType = $state<ResourceType>('all')
  let selectedCategory = $state('')
  let categorySearch = $state('')
  // The card being created; every card waits while one is.
  let selecting = $state('')
  let searchInput = $state<HTMLInputElement>()

  async function loadResources() {
    loading = true
    try {
      templates = (await api<{ templates: ServiceTemplate[] }>('GET', '/service-templates')).templates
    } catch (e) {
      toast.error('Cannot load the Services catalog.', e instanceof Error ? e.message : String(e))
    } finally {
      loading = false
    }
  }

  $effect(() => {
    environment = null
    loadError = ''
    // A viewer has no create rights: the Environment page instead.
    if (!session.canWrite) {
      location.replace(href(`/project/${projectId}/environment/${id}`))
      return
    }
    Promise.all([
      api<{ environment: Environment }>('GET', `/environments/${id}`),
      api<{ servers: Server[] }>('GET', '/servers'),
    ])
      .then(([e, s]) => {
        if (e.environment.project_id !== projectId) throw new Error('Environment not found.')
        servers = s.servers
        environment = e.environment
      })
      .catch((e) => (loadError = e.message))
    loadResources()
  })

  // The Local server always counts; a remote one once it validated.
  const ready = $derived(servers.filter((s) => s.kind === 'local' || s.status === 'reachable'))
  const notReady = $derived(servers.filter((s) => !ready.includes(s)))
  const card = $derived(applicationCards.find((c) => c.id === type))
  // Docker Compose becomes a Service, and Services run on the Local server.
  const needsServer = $derived(!!card && card.id !== 'docker-compose-empty')
  const step = $derived(!card ? 'type' : needsServer && server === null ? 'servers' : 'create')

  const base = $derived(`/project/${projectId}/environment/${id}/new`)

  // With one ready Server there is nothing to ask, as setType() does.
  $effect(() => {
    if (environment && step === 'servers' && ready.length === 1) location.replace(href(`${base}?type=${type}&server=${ready[0].id}`))
  })

  function matches(item: { name: string; description: string }) {
    const q = search.trim().toLowerCase()
    return !q || item.name.toLowerCase().includes(q) || item.description.toLowerCase().includes(q)
  }

  function categoryLabel(c: string) {
    return acronyms.has(c.toLowerCase()) ? c.toUpperCase() : c
  }

  const categories = $derived(
    [...new Set(templates.flatMap((t) => t.category.split(',').map((c) => categoryLabel(c.trim()))).filter(Boolean))].sort((a, b) =>
      a.localeCompare(b, undefined, { numeric: true, sensitivity: 'base' }),
    ),
  )
  const shownCategories = $derived(categories.filter((c) => !categorySearch || c.toLowerCase().includes(categorySearch.toLowerCase())))

  const filteredGit = $derived(gitBasedApplications.filter(matches))
  const filteredDocker = $derived(dockerBasedApplications.filter(matches))
  const filteredDatabases = $derived(databases.filter(matches))
  const filteredServices = $derived(
    templates
      .filter((t) => matches(t))
      .filter(
        (t) =>
          selectedCategory === '' ||
          t.category
            .split(',')
            .map((c) => c.trim().toLowerCase())
            .includes(selectedCategory.toLowerCase()),
      )
      .toSorted((a, b) => a.name.localeCompare(b.name)),
  )
  const showApplications = $derived((resourceType === 'all' || resourceType === 'applications') && filteredGit.length + filteredDocker.length > 0)
  const showDatabases = $derived((resourceType === 'all' || resourceType === 'databases') && filteredDatabases.length > 0)
  const showServices = $derived((resourceType === 'all' || resourceType === 'services') && filteredServices.length > 0)

  function websiteOf(url: string) {
    try {
      return new URL(url).origin
    } catch {
      return ''
    }
  }

  async function create(key: string, run: () => Promise<string>) {
    if (selecting) return
    selecting = key
    try {
      go(await run())
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      toast.error(err.message)
    } finally {
      selecting = ''
    }
  }

  function pickApplication(c: ApplicationCard) {
    if (selecting) return
    if (c.id === 'docker-compose-empty' || ready.length !== 1) go(`${base}?type=${c.id}`)
    else go(`${base}?type=${c.id}&server=${ready[0].id}`)
  }

  function pickDatabase(t: DatabaseType) {
    create(`database-${t}`, async () => {
      const { database } = await api<{ database: Database }>('POST', `/environments/${id}/databases`, { type: t })
      toast.success('Database created.')
      return `/databases/${database.id}`
    })
  }

  function pickService(t: ServiceTemplate) {
    create(`service-${t.key}`, async () => {
      const { service } = await api<{ service: Service }>('POST', `/environments/${id}/services`, { name: '', template: t.key })
      toast.success('Service created.')
      return `/services/${service.id}`
    })
  }

  async function addApplication(input: ApplicationInput) {
    const { application } = await api<{ application: Application }>('POST', `/environments/${id}/applications`, input)
    go(`/applications/${application.id}`)
  }

  async function addService(input: ServiceInput) {
    const { service } = await api<{ service: Service }>('POST', `/environments/${id}/services`, input)
    go(`/services/${service.id}`)
  }

  function onkeydown(e: KeyboardEvent) {
    // "/" jumps to the search box, as the Blade's @keydown.window.slash.
    const target = e.target as HTMLElement
    if (e.key !== '/' || step !== 'type' || target.closest('input, textarea, select, [contenteditable]')) return
    e.preventDefault()
    searchInput?.focus()
  }

  const crumbs = $derived(environment ? { project: environment.project_name ?? '', environment: environment.name } : null)
  $effect(() => {
    if (crumbs)
      breadcrumb.set(
        { label: 'Projects', href: href('/projects') },
        { label: crumbs.project, href: href(`/project/${projectId}`) },
        { label: crumbs.environment, href: href(`/project/${projectId}/environment/${id}`) },
        { label: 'New resource' },
      )
  })

  const cardClass =
    'group flex min-h-48 cursor-pointer flex-col rounded-xl border border-neutral-200 bg-white p-4 text-left transition-colors hover:border-neutral-300 focus-visible:ring-1 focus-visible:ring-accent focus-visible:outline-none dark:border-white/[0.08] dark:bg-white/[0.05] dark:hover:border-white/[0.14]'
  const logoBox =
    'flex size-11 shrink-0 items-center justify-center overflow-hidden rounded-xl border border-neutral-200 bg-neutral-50 dark:border-white/[0.08] dark:bg-white/[0.04]'
  const serverIcon =
    'flex size-8 shrink-0 items-center justify-center rounded-lg border border-neutral-200 bg-neutral-50 text-neutral-500 dark:border-white/[0.08] dark:bg-white/[0.035] dark:text-fg-dim'
</script>

<svelte:window {onkeydown} />

{#snippet resourceCard(key: string, c: Card, subtitle: string, onpick: () => void, imgClass = 'size-full object-contain')}
  <div
    role="button"
    tabindex="0"
    aria-label="Deploy {c.name}"
    aria-busy={selecting === key}
    aria-disabled={!!selecting}
    onclick={onpick}
    onkeydown={(e) => {
      if (e.target === e.currentTarget && (e.key === 'Enter' || e.key === ' ')) {
        e.preventDefault()
        onpick()
      }
    }}
    class={[cardClass, selecting && selecting !== key && 'opacity-60', selecting && 'cursor-wait']}
  >
    <div class="flex min-w-0 items-start gap-3">
      <div class={logoBox}>
        <img class={imgClass} src={c.logo} alt="" onerror={(e) => ((e.currentTarget as HTMLImageElement).src = 'svgs/default.webp')} />
      </div>
      <div class="min-w-0 flex-1">
        <h3 class="truncate text-[13px] font-semibold text-black dark:text-fg">{c.name}</h3>
        {#if subtitle}<p class="mt-0.5 truncate text-[11px] text-neutral-500 dark:text-fg-faint">{subtitle}</p>{/if}
      </div>
    </div>
    <p class="mt-3 line-clamp-2 text-[12px] leading-5 text-neutral-600 dark:text-fg-dim">{c.description}</p>
    <div class="mt-auto flex items-center gap-1.5 border-t border-neutral-200 pt-3 dark:border-white/[0.07]">
      <a class="button" href={c.docs} target="_blank" rel="noopener noreferrer" onclick={(e) => e.stopPropagation()}>Docs</a>
      {#if c.website}
        <a class="button" href={c.website} target="_blank" rel="noopener noreferrer" onclick={(e) => e.stopPropagation()}>Website</a>
      {/if}
      <span class="button button-highlighted ml-auto">
        {#if selecting === key}
          <Spinner text="Deploying" />
        {:else}
          Deploy
          <Icon name="arrow-right" class="size-3.5" />
        {/if}
      </span>
    </div>
  </div>
{/snippet}

{#snippet sectionHeader(icon: 'globe' | 'database' | 'layers', title: string)}
  <div class="application-settings-section-header">
    <div class="flex items-center gap-2">
      <Icon name={icon} class="size-4 text-neutral-400 dark:text-fg-faint" />
      <h2>{title}</h2>
    </div>
  </div>
{/snippet}

{#if loadError}
  <p class="text-sm text-error">{loadError}</p>
{:else if !environment}
  <Spinner text="Loading…" />
{:else}
  <div class="chrome application-settings-form w-full">
    <h1 class="sr-only">New resource in {environment.name}</h1>

    {#if step === 'type'}
      <section class="application-settings-section">
        <header>
          <div class="min-w-0 py-0.5"><h3>Choose a resource</h3></div>
          <div class="flex min-w-0 max-w-full flex-wrap items-center gap-2">
            <button type="button" class="button" disabled={loading} onclick={loadResources}>
              <Icon name="refresh" class="size-3.5" />
              Reload
            </button>
          </div>
        </header>
        <div class="application-settings-section-body is-flush">
          <div class="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between">
            <div class="relative min-w-0 flex-1">
              <Icon
                name="search"
                class="pointer-events-none absolute top-1/2 left-2.5 z-10 size-3.5 -translate-y-1/2 text-neutral-400 dark:text-fg-faint"
              />
              <!-- svelte-ignore a11y_autofocus -->
              <input
                bind:this={searchInput}
                bind:value={search}
                autocomplete="off"
                autofocus
                type="search"
                placeholder="Search resources"
                aria-label="Search resources"
                class="input h-8! w-full rounded-lg! border-neutral-200! bg-white! py-0! pr-8! pl-8! text-[12px]! shadow-none! placeholder:text-neutral-400 focus:border-accent! focus:ring-0! dark:border-white/[0.08]! dark:bg-white/[0.035]! dark:text-fg! dark:placeholder:text-fg-faint"
              />
            </div>

            <div class="flex shrink-0 items-center gap-2">
              <TableDropdown panelClass="w-48!">
                {#snippet trigger({ open, toggle })}
                  <button type="button" class="button" aria-haspopup="listbox" aria-expanded={open} onclick={toggle}>
                    <Icon name="filter" class="size-3.5" />
                    Filter
                  </button>
                {/snippet}
                {#snippet children(close)}
                  <div class="px-2 py-1 text-[10px] font-semibold tracking-wide text-neutral-400 uppercase dark:text-fg-faint">
                    Resource type
                  </div>
                  {#each resourceTypeOptions as option (option.value)}
                    <button
                      type="button"
                      class="listbox-option"
                      role="option"
                      aria-selected={resourceType === option.value}
                      onclick={() => {
                        resourceType = option.value
                        close()
                      }}
                    >
                      <span>{option.label}</span>
                      {#if resourceType === option.value}<Icon name="check-circle" class="size-3.5 text-accent" />{/if}
                    </button>
                  {/each}
                {/snippet}
              </TableDropdown>

              <div class="w-48">
                <TableDropdown panelClass="min-w-56! p-0!">
                  {#snippet trigger({ open, toggle })}
                    <button
                      type="button"
                      class="listbox-trigger"
                      disabled={loading || categories.length === 0}
                      aria-haspopup="listbox"
                      aria-expanded={open}
                      title={selectedCategory || 'All categories'}
                      onclick={() => {
                        categorySearch = ''
                        toggle()
                      }}
                    >
                      <span class="listbox-trigger-label capitalize">{selectedCategory || 'All categories'}</span>
                      <svg class="size-3.5 shrink-0 opacity-60" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="m8 9 4-4 4 4m0 6-4 4-4-4" />
                      </svg>
                    </button>
                  {/snippet}
                  {#snippet children(close)}
                    <div class="border-b border-neutral-200 p-2 dark:border-white/[0.08]">
                      <!-- svelte-ignore a11y_autofocus -->
                      <input
                        type="search"
                        bind:value={categorySearch}
                        autofocus
                        placeholder="Search categories"
                        aria-label="Search categories"
                        class="input h-8! w-full rounded-md! border-neutral-200! bg-neutral-50! px-2.5! py-0! text-[12px]! shadow-none! focus:ring-0! dark:border-white/[0.08]! dark:bg-white/[0.04]! dark:text-fg!"
                      />
                    </div>
                    <div class="max-h-60 overflow-auto p-1" role="listbox" aria-label="Service category">
                      <button
                        type="button"
                        class="listbox-option"
                        role="option"
                        aria-selected={selectedCategory === ''}
                        onclick={() => {
                          selectedCategory = ''
                          close()
                        }}
                      >
                        <span>All categories</span>
                        {#if selectedCategory === ''}<Icon name="check-circle" class="size-3.5 text-accent" />{/if}
                      </button>
                      {#each shownCategories as category (category)}
                        <button
                          type="button"
                          class="listbox-option capitalize"
                          role="option"
                          aria-selected={selectedCategory === category}
                          onclick={() => {
                            selectedCategory = category
                            close()
                          }}
                        >
                          <span class="truncate">{category}</span>
                          {#if selectedCategory === category}<Icon name="check-circle" class="size-3.5 text-accent" />{/if}
                        </button>
                      {/each}
                    </div>
                  {/snippet}
                </TableDropdown>
              </div>
            </div>
          </div>
        </div>
      </section>

      {#if loading}
        <div class="flex items-center justify-center py-8"><Spinner text="Loading resources..." /></div>
      {:else}
        <div class="mt-6 flex flex-col gap-6">
          {#if showApplications}
            <section class="application-settings-section">
              {@render sectionHeader('globe', 'Applications')}
              <div class="application-settings-section-body grid grid-cols-1 justify-start gap-3 text-left md:grid-cols-2 xl:grid-cols-3">
                {#each filteredGit as c (c.id)}
                  {@render resourceCard(`application-${c.id}`, c, c.source, () => pickApplication(c))}
                {/each}
                {#each filteredDocker as c (c.id)}
                  {@render resourceCard(`application-${c.id}`, c, c.source, () => pickApplication(c))}
                {/each}
              </div>
            </section>
          {/if}

          {#if showDatabases}
            <section class="application-settings-section">
              {@render sectionHeader('database', 'Databases')}
              <div class="application-settings-section-body grid grid-cols-1 justify-start gap-3 text-left md:grid-cols-2 xl:grid-cols-3">
                {#each filteredDatabases as c (c.id)}
                  {@render resourceCard(`database-${c.id}`, c, '', () => pickDatabase(c.id))}
                {/each}
              </div>
            </section>
          {/if}

          {#if showServices}
            <section class="application-settings-section">
              {@render sectionHeader('layers', 'Services')}
              <div class="application-settings-section-body">
                <Callout type="info" title="Trademarks policy" class="mb-4">
                  The respective trademarks mentioned here are owned by the respective companies, and use of them does not imply any
                  affiliation or endorsement.
                </Callout>
                <div class="grid grid-cols-1 justify-start gap-3 text-left md:grid-cols-2 xl:grid-cols-3">
                  {#each filteredServices as t (t.key)}
                    {@render resourceCard(
                      `service-${t.key}`,
                      { id: t.key, name: t.name, description: t.description, logo: t.logo, docs: t.docs_url, website: websiteOf(t.docs_url) },
                      'Template ready',
                      () => pickService(t),
                      'h-full w-full object-contain p-2',
                    )}
                  {/each}
                </div>
              </div>
            </section>
          {/if}

          {#if !showApplications && !showDatabases && !showServices}
            <Empty title="No resources found" description="Try a different search or resource type." icon="layers" size="sm" />
          {/if}
        </div>
      {/if}
    {:else if step === 'servers'}
      <section class="application-settings-section">
        <header>
          <div class="min-w-0 py-0.5">
            <h3>Select a server</h3>
            <p class="mt-0.5 text-[12px] text-neutral-500 dark:text-fg-dim">Choose the machine that will host this resource.</p>
          </div>
          <div class="flex min-w-0 max-w-full flex-wrap items-center gap-2">
            <a class="button" href={href(base)}>Back</a>
          </div>
        </header>
        <div class="application-settings-section-body is-flush">
          {#if ready.length === 0}
            <Callout type="warning" title="No deployment server" class="m-4">
              No server is ready. Add or validate a server before continuing.
              <a class="font-medium underline" href={href('/servers')}>Open servers</a>
            </Callout>
          {/if}
          <div class="divide-y divide-neutral-200 dark:divide-white/[0.07]">
            {#each ready as s (s.id)}
              <button
                type="button"
                onclick={() => go(`${base}?type=${type}&server=${s.id}`)}
                class="group flex min-h-14 w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-neutral-50 dark:hover:bg-white/[0.025]"
              >
                <span class={serverIcon}><Icon name="servers" class="size-4" /></span>
                <span class="min-w-0 flex-1">
                  <span class="block truncate text-[13px] font-semibold text-black dark:text-fg">{s.name}</span>
                  <span class="block truncate text-[11px] text-neutral-500 dark:text-fg-faint">{s.kind === 'local' ? 'This machine' : s.host}</span>
                </span>
                <StatusBadge status="Ready" type="success" />
              </button>
            {/each}
            {#each notReady as s (s.id)}
              <div class="flex min-h-14 items-center gap-3 px-4 py-3 opacity-55">
                <span class={serverIcon}><Icon name="servers" class="size-4" /></span>
                <span class="min-w-0 flex-1">
                  <span class="block truncate text-[13px] font-semibold text-black dark:text-fg">{s.name}</span>
                  <span class="block text-[11px] text-neutral-500 dark:text-fg-faint">Validate this server before it can host resources.</span>
                </span>
                <StatusBadge status={s.status === 'unreachable' ? 'Unreachable' : 'Not validated'} type="neutral" />
                <a href={href(`/servers/${s.id}`)} class="button">Settings</a>
              </div>
            {/each}
          </div>
        </div>
      </section>
    {/if}
  </div>
  {#if step === 'create' && card}
    <!-- The create pages of Coolify's Application cards come next; until then
         each card opens the form The Bakery had, outside .chrome so it keeps
         its legacy styles. -->
    <div class="chrome mb-4 flex items-center justify-between gap-3">
      <h2 class="text-[15px] font-semibold text-black dark:text-fg">{card.name}</h2>
      <a class="button" href={href(base)}>Back</a>
    </div>
    {#if card.id === 'docker-compose-empty'}
      <ServiceForm onsubmit={addService} />
    {:else}
      {#key `${card.id}-${server}`}
        <ApplicationForm submitLabel="Create application" server={server ?? undefined} buildPack={card.buildPack} onsubmit={addApplication} />
      {/key}
    {/if}
  {/if}
{/if}
