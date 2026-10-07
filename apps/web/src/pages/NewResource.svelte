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
  //
  // In Paperclip's look (MIT, see NOTICE): the ProjectDetail header, the list
  // toolbar of Projects.tsx, Cards for the types, EntityRows for the Servers,
  // and a settings page (CompanySettings.tsx) around each create form.
  import { Plus } from '@lucide/svelte'
  import { buttonVariants } from '$lib/components/ui/button'
  import { Card } from '$lib/components/ui/card'
  import * as Popover from '$lib/components/ui/popover'
  import { api, ApiError } from '../lib/api'
  import { breadcrumb } from '../lib/breadcrumb.svelte'
  import CollectionToolbar from '../lib/CollectionToolbar.svelte'
  import EntityRow from '../lib/EntityRow.svelte'
  import Icon from '../lib/Icon.svelte'
  import DockerCompose from '../lib/new/DockerCompose.svelte'
  import DockerImage from '../lib/new/DockerImage.svelte'
  import PrivateGitRepository from '../lib/new/PrivateGitRepository.svelte'
  import PublicGitRepository from '../lib/new/PublicGitRepository.svelte'
  import { databasePath, go, href, serverPath, servicePath } from '../lib/router.svelte'
  import PageHeader from '../lib/PageHeader.svelte'
  import PageSkeleton from '../lib/PageSkeleton.svelte'
  import { projectAccess } from '../lib/projectAccess.svelte'
  import ProjectTile from '../lib/ProjectTile.svelte'
  import SearchField from '../lib/SearchField.svelte'
  import SettingsPage from '../lib/settings/SettingsPage.svelte'
  import type { Database, DatabaseType, Environment, Server, Service, ServiceTemplate } from '../lib/types'
  import Callout from '../lib/ui/Callout.svelte'
  import Empty from '../lib/ui/Empty.svelte'
  import Spinner from '../lib/ui/Spinner.svelte'
  import StatusBadge from '../lib/ui/StatusBadge.svelte'
  import { toast } from '../lib/ui/toast.svelte'

  let {
    projectId,
    id,
    type,
    server,
  }: { projectId: number; id: number; type: string; server: number | null } = $props()

  type ResourceType = 'all' | 'applications' | 'databases' | 'services'
  type ResourceCard = { id: string; name: string; description: string; logo: string; docs: string; website?: string }
  type ApplicationCard = ResourceCard & { source: 'Git source' | 'Docker source' }

  // Select.php's cards, in its order, with what The Bakery can create.
  const gitBasedApplications: ApplicationCard[] = [
    {
      id: 'public',
      name: 'Public Git Repository',
      description: 'Deploy any public Git repository. The Bakery builds it from source, no credentials required.',
      logo: 'svgs/resources/public-repo.svg',
      docs: 'https://git-scm.com/docs',
      source: 'Git source',
    },
    {
      id: 'private-deploy-key',
      name: 'Private Git Repository (with Deploy Key)',
      description:
        'Deploy a private repository over SSH with a repository-scoped deploy key. Set the URL and branch manually, no automatic deploys.',
      logo: 'svgs/resources/deploy-key.svg',
      docs: 'https://docs.github.com/en/authentication/connecting-to-github-with-ssh/managing-deploy-keys',
      source: 'Git source',
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
    },
    {
      id: 'docker-image',
      name: 'Docker Image',
      description: 'Deploy an application using a prebuilt image from any Docker registry, without a Git repository.',
      logo: 'svgs/resources/docker-image.svg',
      docs: 'https://docs.podman.io/en/latest/markdown/podman-pull.1.html',
      source: 'Docker source',
    },
  ]
  const databases: (ResourceCard & { id: DatabaseType })[] = [
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

  let searchText = $state('')
  let resourceType = $state<ResourceType>('all')
  let selectedCategory = $state('')
  let categorySearch = $state('')
  // The card being created; every card waits while one is.
  let selecting = $state('')
  let searchInput = $state<HTMLInputElement | null>(null)

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
    // A viewer has no create rights: the Environment page instead. Opened
    // straight from a link, the Project's Permissions are still on their way.
    if (!projectAccess.ready) return
    if (!projectAccess.can('manage_applications')) {
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
    const q = searchText.trim().toLowerCase()
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
      return databasePath(database)
    })
  }

  function pickService(t: ServiceTemplate) {
    create(`service-${t.key}`, async () => {
      const { service } = await api<{ service: Service }>('POST', `/environments/${id}/services`, { name: '', template: t.key })
      toast.success('Service created.')
      return servicePath(service)
    })
  }

  function onkeydown(e: KeyboardEvent) {
    // "/" jumps to the search box, as the Blade's @keydown.window.slash.
    const target = e.target as HTMLElement
    if (e.key !== '/' || step !== 'type' || target.closest('input, textarea, select, [contenteditable]')) return
    e.preventDefault()
    searchInput?.focus()
  }

  // On a create page "New resource" leads back to the chooser.
  const crumbs = $derived(
    environment
      ? { project: environment.project_name ?? '', environment: environment.name, card: step === 'create' ? card?.name : undefined, base }
      : null,
  )
  $effect(() => {
    if (crumbs)
      breadcrumb.set(
        { label: 'Projects', href: href('/projects') },
        { label: crumbs.project, href: href(`/project/${projectId}`) },
        { label: crumbs.environment, href: href(`/project/${projectId}/environment/${id}`) },
        ...(crumbs.card ? [{ label: 'New resource', href: href(crumbs.base) }, { label: crumbs.card }] : [{ label: 'New resource' }]),
      )
  })

  const deployButton = buttonVariants({ size: 'xs', class: 'ml-auto' })
  const linkButton = buttonVariants({ variant: 'ghost', size: 'xs', class: 'relative z-10 text-muted-foreground' })
  const option = (selected: boolean) => [
    'flex w-full items-center justify-between gap-2 rounded-sm px-2 py-1.5 text-sm',
    selected ? 'bg-accent/50 text-foreground' : 'text-muted-foreground hover:bg-accent/50',
  ]
  const cardGrid = 'grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3'
  let typeOpen = $state(false)
  let categoryOpen = $state(false)
  const typeLabel = $derived(resourceTypeOptions.find((o) => o.value === resourceType)?.label ?? 'All resources')
</script>

<svelte:window {onkeydown} />

{#snippet resourceCard(key: string, c: ResourceCard, subtitle: string, onpick: () => void, imgClass = 'size-full object-contain')}
  <Card
    interactive
    role="button"
    tabindex={0}
    aria-label="Deploy {c.name}"
    aria-busy={selecting === key}
    aria-disabled={!!selecting}
    onclick={onpick}
    onkeydown={(e: KeyboardEvent) => {
      if (e.target === e.currentTarget && (e.key === 'Enter' || e.key === ' ')) {
        e.preventDefault()
        onpick()
      }
    }}
    class={['min-h-44 gap-0 p-4 text-left hover:bg-accent/50', selecting && selecting !== key && 'opacity-60', selecting && 'cursor-wait'].filter(Boolean).join(' ')}
  >
    <div class="flex min-w-0 items-start gap-3">
      <div class="flex size-10 shrink-0 items-center justify-center overflow-hidden rounded-lg border border-border bg-muted/40">
        <img class={imgClass} src={c.logo} alt="" onerror={(e) => ((e.currentTarget as HTMLImageElement).src = 'svgs/default.webp')} />
      </div>
      <div class="min-w-0 flex-1">
        <h3 class="truncate text-sm font-medium" title={c.name}>{c.name}</h3>
        {#if subtitle}<p class="mt-0.5 truncate text-xs text-muted-foreground">{subtitle}</p>{/if}
      </div>
    </div>
    <p class="mt-3 line-clamp-2 text-xs leading-5 text-muted-foreground">{c.description}</p>
    <div class="mt-auto flex items-center gap-1 border-t border-border pt-3">
      <a class={linkButton} href={c.docs} target="_blank" rel="noopener noreferrer" onclick={(e) => e.stopPropagation()}>Docs</a>
      {#if c.website}
        <a class={linkButton} href={c.website} target="_blank" rel="noopener noreferrer" onclick={(e) => e.stopPropagation()}>Website</a>
      {/if}
      <span class={deployButton}>
        {#if selecting === key}
          <Spinner text="Deploying" />
        {:else}
          Deploy
          <Icon name="arrow-right" class="size-3" />
        {/if}
      </span>
    </div>
  </Card>
{/snippet}

{#snippet groupLabel(icon: 'globe' | 'database' | 'layers', title: string)}
  <div class="flex items-center gap-2 text-xs font-medium tracking-wide text-muted-foreground uppercase">
    <Icon name={icon} class="size-3.5" />
    <h2 class="text-xs font-medium">{title}</h2>
  </div>
{/snippet}

{#if loadError}
  <p class="text-sm text-destructive">{loadError}</p>
{:else if !environment}
  <PageSkeleton />
{:else if step === 'type'}
  <div class="chrome w-full space-y-6">
    <PageHeader title="New resource" description="Add an application, database or service to {environment.name}.">
      {#snippet leading()}<ProjectTile size="lg" icon="plus" />{/snippet}
      {#snippet actions()}
        <button type="button" class={buttonVariants({ variant: 'outline', size: 'sm' })} disabled={loading} onclick={loadResources}>
          <Icon name="refresh" class="size-3.5" />
          Reload
        </button>
      {/snippet}
    </PageHeader>

    <CollectionToolbar ariaLabel="Resource controls">
      {#snippet search()}
        <SearchField bind:value={searchText} bind:ref={searchInput} autofocus label="Search resources" />
      {/snippet}
      {#snippet controls()}
        <Popover.Root bind:open={typeOpen}>
          <Popover.Trigger
            class={buttonVariants({ variant: 'ghost', size: 'sm', class: ['text-xs', resourceType !== 'all' && 'bg-accent'].filter(Boolean).join(' ') })}
            title="Resource type"
          >
            <Icon name="filter" class="size-3.5 sm:size-3" />
            <span>{resourceType === 'all' ? 'Filter' : typeLabel}</span>
          </Popover.Trigger>
          <Popover.Content align="start" class="w-48 p-2">
            <div class="px-2 pt-1 pb-1 text-[10px] font-semibold tracking-wide text-muted-foreground uppercase">Resource type</div>
            <div class="space-y-0.5" role="listbox" aria-label="Resource type">
              {#each resourceTypeOptions as o (o.value)}
                <button type="button" role="option" aria-selected={resourceType === o.value} class={option(resourceType === o.value)} onclick={() => {
                    resourceType = o.value
                    typeOpen = false
                  }}>
                  <span>{o.label}</span>
                  {#if resourceType === o.value}<Icon name="check" class="size-3 text-muted-foreground" />{/if}
                </button>
              {/each}
            </div>
          </Popover.Content>
        </Popover.Root>
        <Popover.Root bind:open={categoryOpen} onOpenChange={(open) => open && (categorySearch = '')}>
          <Popover.Trigger
            class={buttonVariants({ variant: 'ghost', size: 'sm', class: ['max-w-56 text-xs', selectedCategory && 'bg-accent'].filter(Boolean).join(' ') })}
            disabled={loading || categories.length === 0}
            title={selectedCategory || 'All categories'}
          >
            <Icon name="layers" class="size-3.5 sm:size-3" />
            <span class={['truncate', selectedCategory && 'capitalize']}>{selectedCategory || 'All categories'}</span>
            <Icon name="chevron-down" class="size-3 opacity-60" />
          </Popover.Trigger>
          <Popover.Content align="start" class="w-60 p-0">
            <div class="border-b border-border p-2">
              <!-- svelte-ignore a11y_autofocus -->
              <input
                type="search"
                bind:value={categorySearch}
                autofocus
                placeholder="Search categories"
                aria-label="Search categories"
                class="h-8 w-full rounded-md border border-input bg-transparent px-2.5 text-sm outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
              />
            </div>
            <div class="max-h-60 space-y-0.5 overflow-auto p-2" role="listbox" aria-label="Service category">
              <button type="button" role="option" aria-selected={selectedCategory === ''} class={option(selectedCategory === '')} onclick={() => {
                  selectedCategory = ''
                  categoryOpen = false
                }}>
                <span>All categories</span>
                {#if selectedCategory === ''}<Icon name="check" class="size-3 text-muted-foreground" />{/if}
              </button>
              {#each shownCategories as category (category)}
                <button
                  type="button"
                  role="option"
                  aria-selected={selectedCategory === category}
                  class={[...option(selectedCategory === category), 'capitalize']}
                  onclick={() => {
                    selectedCategory = category
                    categoryOpen = false
                  }}
                >
                  <span class="truncate">{category}</span>
                  {#if selectedCategory === category}<Icon name="check" class="size-3 text-muted-foreground" />{/if}
                </button>
              {/each}
            </div>
          </Popover.Content>
        </Popover.Root>
      {/snippet}
    </CollectionToolbar>

    {#if loading}
      <div class="flex items-center justify-center py-8"><Spinner text="Loading resources..." /></div>
    {:else}
      <div class="space-y-8">
        {#if showApplications}
          <section class="space-y-3">
            {@render groupLabel('globe', 'Applications')}
            <div class={cardGrid}>
              {#each [...filteredGit, ...filteredDocker] as c (c.id)}
                {@render resourceCard(`application-${c.id}`, c, c.source, () => pickApplication(c))}
              {/each}
            </div>
          </section>
        {/if}
        {#if showDatabases}
          <section class="space-y-3">
            {@render groupLabel('database', 'Databases')}
            <div class={cardGrid}>
              {#each filteredDatabases as c (c.id)}
                {@render resourceCard(`database-${c.id}`, c, '', () => pickDatabase(c.id))}
              {/each}
            </div>
          </section>
        {/if}
        {#if showServices}
          <section class="space-y-3">
            {@render groupLabel('layers', 'Services')}
            <Callout type="info" title="Trademarks policy">
              The respective trademarks mentioned here are owned by the respective companies, and use of them does not imply any affiliation or
              endorsement.
            </Callout>
            <div class={cardGrid}>
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
          </section>
        {/if}
        {#if !showApplications && !showDatabases && !showServices}
          <Card class="block py-0">
            <Empty title="No resources found" description="Try a different search or resource type." icon="search" size="sm" />
          </Card>
        {/if}
      </div>
    {/if}
  </div>
{:else if step === 'servers'}
  <div class="chrome w-full space-y-6">
    <PageHeader title="Select a server" description="Choose the machine that will host this resource.">
      {#snippet leading()}<ProjectTile size="lg" icon="servers" />{/snippet}
      {#snippet actions()}
        <a class={buttonVariants({ variant: 'outline', size: 'sm' })} href={href(base)}>Back</a>
      {/snippet}
    </PageHeader>
    {#if ready.length === 0}
      <Callout type="warning" title="No deployment server">
        No server is ready. Add or validate a server before continuing.
        <a class="font-medium underline" href={href('/servers')}>Open servers</a>
      </Callout>
    {/if}
    {#if servers.length > 0}
      <Card class="block gap-0 overflow-hidden py-0">
        {#each ready as s (s.id)}
          <EntityRow title={s.name} subtitle={s.kind === 'local' ? 'This machine' : s.host} href={href(`${base}?type=${type}&server=${s.id}`)}>
            {#snippet leading()}<ProjectTile size="sm" icon="servers" />{/snippet}
            {#snippet trailing()}<StatusBadge status="Ready" type="success" />{/snippet}
          </EntityRow>
        {/each}
        {#each notReady as s (s.id)}
          <EntityRow title={s.name} subtitle="Validate this server before it can host resources." class="opacity-60">
            {#snippet leading()}<ProjectTile size="sm" icon="servers" />{/snippet}
            {#snippet trailing()}
              <StatusBadge status={s.status === 'unreachable' ? 'Unreachable' : 'Not validated'} type="neutral" />
              <a href={href(serverPath(s.id))} class={buttonVariants({ variant: 'outline', size: 'xs' })}>Settings</a>
            {/snippet}
          </EntityRow>
        {/each}
      </Card>
    {/if}
  </div>
{:else if step === 'create' && card}
  <SettingsPage icon={Plus} title={card.name}>
    {#snippet actions()}
      <a class={buttonVariants({ variant: 'outline', size: 'sm' })} href={href(base)}>Back</a>
    {/snippet}
    {#if card.id === 'public'}
      <PublicGitRepository environmentId={id} server={server!} />
    {:else if card.id === 'private-deploy-key'}
      <PrivateGitRepository environmentId={id} server={server!} />
    {:else if card.id === 'docker-image'}
      <DockerImage environmentId={id} server={server!} />
    {:else}
      <DockerCompose environmentId={id} />
    {/if}
  </SettingsPage>
{/if}
