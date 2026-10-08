<script lang="ts">
  // Coolify's Project page (resources/views/livewire/project/show.blade.php
  // with its projectEnvironments() script, and app/Livewire/Project/Show.php;
  // Apache-2.0, see NOTICE): the Project's Environments with search, Sort, a
  // remembered table/grid switch and client-side pages, and "New environment"
  // in a modal.
  //
  // Laid out as Paperclip's ProjectDetail (ui/src/pages/ProjectDetail.tsx,
  // MIT): its header with the tile, name and actions, then the Environments
  // as on the Projects page, EntityRows in one card or cards in the grid.
  // Its Issues tab is a link to the Issues list filtered to the Project, and
  // New issue opens the dialog with the Project preset.
  import { buttonVariants } from '@bakery/ui/components/ui/button'
  import { Card } from '@bakery/ui/components/ui/card'
  import { api, ApiError } from '../lib/api'
  import { breadcrumb } from '../lib/breadcrumb.svelte'
  import CollectionToolbar from '../lib/CollectionToolbar.svelte'
  import EntityRow from '@bakery/ui/EntityRow.svelte'
  import Icon from '../lib/Icon.svelte'
  import NewIssueDialog from '../lib/NewIssueDialog.svelte'
  import PageHeader from '../lib/PageHeader.svelte'
  import PageSkeleton from '../lib/PageSkeleton.svelte'
  import ProjectTile from '../lib/ProjectTile.svelte'
  import { environmentResourceCount, projectResources, type ProjectResources } from '../lib/projectCounts'
  import { go, href } from '../lib/router.svelte'
  import { projectAccess } from '../lib/projectAccess.svelte'
  import SearchField from '../lib/SearchField.svelte'
  import { session } from '../lib/session.svelte'
  import SortPopover from '../lib/SortPopover.svelte'
  import type { Environment } from '../lib/types'
  import Button from '../lib/ui/Button.svelte'
  import ClientPagination from '../lib/ui/ClientPagination.svelte'
  import Empty from '../lib/ui/Empty.svelte'
  import Input from '../lib/ui/Input.svelte'
  import Modal from '../lib/ui/Modal.svelte'
  import ViewToggle from '../lib/ViewToggle.svelte'

  let { id }: { id: number } = $props()

  type SortKey = 'name-asc' | 'name-desc' | 'resources'
  type ViewMode = 'table' | 'grid'

  const viewKey = 'bakery.project-environments-view'
  const pageSizeKey = 'bakery.page-size.project-environments'
  const sortOptions: { value: SortKey; label: string }[] = [
    { value: 'name-asc', label: 'Name A–Z' },
    { value: 'name-desc', label: 'Name Z–A' },
    { value: 'resources', label: 'Most resources' },
  ]

  let resources = $state.raw<ProjectResources | null>(null)
  let loadError = $state('')

  let searchText = $state('')
  let sortBy = $state<SortKey>('name-asc')
  let viewMode = $state<ViewMode>(localStorage.getItem(viewKey) === 'table' ? 'table' : 'grid')
  let page = $state(1)
  let pageSize = $state(12)

  // The search box filters 150 ms after the last key, as Coolify's debounce does.
  let query = $state('')
  $effect(() => {
    const value = searchText
    const timer = setTimeout(() => (query = value.trim().toLowerCase()), 150)
    return () => clearTimeout(timer)
  })

  $effect(() => {
    resources = null
    loadError = ''
    projectResources(id)
      .then((r) => (resources = r))
      .catch((e) => (loadError = e.message))
  })

  const project = $derived(resources?.project)
  const environments = $derived(project?.environments ?? [])
  const resourceCount = (env: Environment) => (resources ? environmentResourceCount(resources, env.id) : 0)

  const filtered = $derived.by(() => {
    const list = environments.filter(
      (env) => !query || [env.name, env.description].filter(Boolean).join(' ').toLowerCase().includes(query),
    )
    return list.sort((a, b) => {
      if (sortBy === 'name-desc') return b.name.localeCompare(a.name)
      if (sortBy === 'resources') return resourceCount(b) - resourceCount(a) || a.name.localeCompare(b.name)
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

  const environmentHref = (env: Environment) => href(`/project/${id}/environment/${env.id}`)
  const addResourceHref = (env: Environment) => (projectAccess.can('manage_applications') ? href(`/project/${id}/environment/${env.id}/new`) : null)
  const settingsHref = (env: Environment) => (projectAccess.can('manage_applications') ? href(`/project/${id}/environment/${env.id}/edit`) : null)

  let creatingIssue = $state(false)

  // New Environment.
  let creating = $state(false)
  let name = $state('')
  let errors = $state<Record<string, string>>({})
  let busy = $state(false)

  async function create(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    errors = {}
    try {
      const { environment } = await api<{ environment: Environment }>('POST', `/projects/${id}/environments`, { name })
      // Coolify opens the new Environment.
      go(`/project/${id}/environment/${environment.id}`)
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      errors = Object.keys(err.errors).length ? err.errors : { name: err.message }
    } finally {
      busy = false
    }
  }

  const plural = (n: number, word: string) => `${n} ${word}${n === 1 ? '' : 's'}`
  const listCard = 'block gap-0 overflow-hidden py-0'
  const rowAction =
    'flex size-6.5 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground'

  const crumbName = $derived(project?.name)
  $effect(() => {
    if (crumbName !== undefined) breadcrumb.set({ label: 'Projects', href: href('/projects') }, { label: crumbName })
  })
</script>

{#snippet rowActions(environment: Environment)}
  {@const addHref = addResourceHref(environment)}
  {@const editHref = settingsHref(environment)}
  {#if addHref}
    <a href={addHref} class={rowAction} title="Add resource" aria-label="Add resource to {environment.name}">
      <Icon name="plus" class="size-3.5" />
    </a>
  {/if}
  {#if editHref}
    <a href={editHref} class={rowAction} title="Environment settings" aria-label="Open settings for {environment.name}">
      <Icon name="settings" class="size-3.5" />
    </a>
  {/if}
{/snippet}

{#if loadError}
  <p class="text-sm text-destructive">{loadError}</p>
{:else if !project}
  <PageSkeleton />
{:else}
  <div class="chrome w-full space-y-4">
    <PageHeader title={project.name} description={project.description || undefined}>
      {#snippet leading()}<ProjectTile size="lg" />{/snippet}
      {#snippet meta()}<span>{plural(environments.length, 'environment')} in this project</span>{/snippet}
      {#snippet actions()}
        {#if session.can('manage_roles')}
          <a
            href={href(`/project/${id}/permissions`)}
            class={buttonVariants({ variant: 'outline', size: 'sm' })}
            title="Project permissions"
            aria-label="Open permissions for {project.name}"
          >
            <Icon name="lock" class="size-3.5" />
            Permissions
          </a>
        {/if}
        <a
          href={href(`/issues?project=${id}`)}
          class={buttonVariants({ variant: 'outline', size: 'sm' })}
          title="This project's issues"
          aria-label="Open the issues of {project.name}"
        >
          <Icon name="issues" class="size-3.5" />
          Issues
        </a>
        {#if session.can('manage_work')}
          <button type="button" class={buttonVariants({ variant: 'outline', size: 'sm' })} onclick={() => (creatingIssue = true)}>
            <Icon name="plus" class="size-3.5" />
            New issue
          </button>
        {/if}
        {#if projectAccess.can('manage_applications')}
          <a
            href={href(`/project/${id}/edit`)}
            class={buttonVariants({ variant: 'outline', size: 'sm' })}
            title="Project settings"
            aria-label="Open settings for {project.name}"
          >
            <Icon name="settings" class="size-3.5" />
            Settings
          </a>
          <Modal title="New Environment" variant="none" bind:open={creating}>
            {#snippet trigger(show)}
              <button type="button" class={buttonVariants({ size: 'sm' })} onclick={show}>
                <Icon name="plus" class="size-3.5" />
                New environment
              </button>
            {/snippet}
            <form class="space-y-4" onsubmit={create}>
              <Input placeholder="staging" label="Name" required bind:value={name} error={errors.name} />

              <footer class="flex justify-end border-t border-border pt-4">
                <Button type="submit" variant="highlighted" loading={busy}>Create environment</Button>
              </footer>
            </form>
          </Modal>
        {/if}
      {/snippet}
    </PageHeader>

    {#if environments.length === 0}
      <Empty title="No environments yet" description="Add an environment to start organizing this project's resources." icon="layers" />
    {:else}
      <CollectionToolbar ariaLabel="Environments controls">
        {#snippet search()}
          <SearchField bind:value={searchText} label="Search environments" oninput={() => (page = 1)} />
        {/snippet}
        {#snippet controls()}
          <SortPopover bind:value={sortBy} options={sortOptions} label="Sort environments" onchange={() => (page = 1)} />
          <ViewToggle value={viewMode} onchange={setViewMode} />
        {/snippet}
      </CollectionToolbar>

      {#if filtered.length === 0}
        <Card class="block py-0">
          <Empty title="No matching environments" description="Try a different search." icon="search" size="sm" />
        </Card>
      {:else if viewMode === 'grid'}
        <div class="space-y-3">
          <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
            {#each paginated as environment (environment.id)}
              <Card interactive class="relative min-h-28 gap-0 p-4">
                <a href={environmentHref(environment)} class="absolute inset-0 rounded-lg" aria-label="Open {environment.name}"></a>
                <div class="flex items-start gap-3">
                  <ProjectTile size="lg" icon="layers" />
                  <div class="min-w-0 flex-1">
                    <h2 class="truncate text-sm font-medium" title={environment.name}>{environment.name}</h2>
                    <p class="mt-0.5 min-h-4 truncate text-xs text-muted-foreground">{environment.description || ''}</p>
                  </div>
                </div>
                <div class="mt-auto flex items-center justify-between gap-3 pt-4">
                  <p class="min-w-0 truncate text-xs text-muted-foreground tabular-nums">{plural(resourceCount(environment), 'resource')}</p>
                  <div class="relative z-10 flex shrink-0 items-center gap-0.5">{@render rowActions(environment)}</div>
                </div>
              </Card>
            {/each}
          </div>
          <Card class={listCard}>
            <ClientPagination bind:page bind:pageSize total={filtered.length} options={[12, 24, 48, 96]} storageKey={pageSizeKey} />
          </Card>
        </div>
      {:else}
        <Card class={listCard}>
          {#each paginated as environment (environment.id)}
            <EntityRow
              title={environment.name}
              subtitle={environment.description || undefined}
              reserveSubtitleSpace
              href={environmentHref(environment)}
              class="group"
            >
              {#snippet leading()}<ProjectTile size="sm" icon="layers" />{/snippet}
              {#snippet trailing()}
                <span class="hidden text-xs text-muted-foreground tabular-nums sm:inline">{plural(resourceCount(environment), 'resource')}</span>
                <div class="flex items-center gap-0.5">{@render rowActions(environment)}</div>
              {/snippet}
            </EntityRow>
          {/each}
          <ClientPagination bind:page bind:pageSize total={filtered.length} options={[12, 24, 48, 96]} storageKey={pageSizeKey} />
        </Card>
      {/if}
    {/if}
  </div>
{/if}

<NewIssueDialog bind:open={creatingIssue} projectId={id} />
