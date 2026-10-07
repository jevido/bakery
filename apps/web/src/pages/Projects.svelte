<script lang="ts">
  // Coolify's Projects page (resources/views/livewire/project/index.blade.php
  // with its projectsIndex() script, app/Livewire/Project/Index.php, and the
  // New Project form of add-empty.blade.php and AddEmpty.php; Apache-2.0, see
  // NOTICE): search, Sort, a remembered table/grid switch and client-side
  // pages, with "New project" in a modal.
  //
  // Laid out as Paperclip's Projects page (ui/src/pages/Projects.tsx, MIT):
  // a CollectionToolbar, a "Sort: <label>" popover, and the Projects as
  // EntityRows in one bordered card, or as cards in the grid view.
  //
  // Left out: project icons (Projects have none here) and the empty state's
  // "Open onboarding" link, until onboarding exists. Paperclip's "My
  // Projects" / "Other Projects" split follows Project membership, which The
  // Bakery does not have, so this is one list.
  import { buttonVariants } from '$lib/components/ui/button'
  import { Card } from '$lib/components/ui/card'
  import { Input as UiInput } from '$lib/components/ui/input'
  import * as Popover from '$lib/components/ui/popover'
  import { api, ApiError } from '../lib/api'
  import { breadcrumb } from '../lib/breadcrumb.svelte'
  import CollectionToolbar from '../lib/CollectionToolbar.svelte'
  import EntityRow from '../lib/EntityRow.svelte'
  import Icon from '../lib/Icon.svelte'
  import PageSkeleton from '../lib/PageSkeleton.svelte'
  import ProjectTile from '../lib/ProjectTile.svelte'
  import { projectCounts, type ProjectCounts } from '../lib/projectCounts'
  import { go, href } from '../lib/router.svelte'
  import { canIn, session } from '../lib/session.svelte'
  import type { Project } from '../lib/types'
  import Button from '../lib/ui/Button.svelte'
  import ClientPagination from '../lib/ui/ClientPagination.svelte'
  import Empty from '../lib/ui/Empty.svelte'
  import Input from '../lib/ui/Input.svelte'
  import Modal from '../lib/ui/Modal.svelte'

  type SortKey = 'name-asc' | 'name-desc' | 'resources' | 'environments'
  type ViewMode = 'table' | 'grid'

  const viewKey = 'bakery.projects-view'
  const pageSizeKey = 'bakery.page-size.projects'
  const sortOptions: { value: SortKey; label: string }[] = [
    { value: 'name-asc', label: 'Name A–Z' },
    { value: 'name-desc', label: 'Name Z–A' },
    { value: 'resources', label: 'Most resources' },
    { value: 'environments', label: 'Most environments' },
  ]

  let projects = $state.raw<Project[] | null>(null)
  let counts = $state.raw<Record<number, ProjectCounts>>({})
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

  api<{ projects: Project[] }>('GET', '/projects')
    .then((r) => {
      projects = r.projects
      for (const p of r.projects) {
        projectCounts(p.id).then((c) => {
          if (c) counts = { ...counts, [p.id]: c }
        })
      }
    })
    .catch((e) => (loadError = e.message))

  const environmentCount = (p: Project) => counts[p.id]?.environments ?? p.environments?.length ?? 0
  const resourceCount = (p: Project) => counts[p.id]?.resources ?? 0

  const filtered = $derived.by(() => {
    const list = (projects ?? []).filter(
      (p) => !query || [p.name, p.description].filter(Boolean).join(' ').toLowerCase().includes(query),
    )
    return list.sort((a, b) => {
      if (sortBy === 'name-desc') return b.name.localeCompare(a.name)
      if (sortBy === 'resources') return resourceCount(b) - resourceCount(a) || a.name.localeCompare(b.name)
      if (sortBy === 'environments') return environmentCount(b) - environmentCount(a) || a.name.localeCompare(b.name)
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

  // A Project may override manage_applications, so its own buttons ask the Project.
  const canManage = (p: Project) => !!counts[p.id] && canIn(counts[p.id].permissions, 'manage_applications')
  const addResourceHref = (p: Project) => {
    const first = counts[p.id]?.firstEnvironment
    return first && canManage(p) ? href(`/project/${p.id}/environment/${first}/new`) : null
  }
  const settingsHref = (p: Project) => (canManage(p) ? href(`/project/${p.id}/edit`) : null)

  // New Project.
  let creating = $state(false)
  let name = $state('')
  let description = $state('')
  let errors = $state<Record<string, string>>({})
  let busy = $state(false)

  async function create(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    errors = {}
    try {
      const { project } = await api<{ project: Project }>('POST', '/projects', { name, description })
      // Coolify opens the new Project's production Environment.
      const environments =
        project.environments ?? (await api<{ project: Project }>('GET', `/projects/${project.id}`)).project.environments ?? []
      const production = environments.find((env) => env.name === 'production') ?? environments[0]
      go(production ? `/project/${project.id}/environment/${production.id}` : `/project/${project.id}`)
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      errors = Object.keys(err.errors).length ? err.errors : { name: err.message }
    } finally {
      busy = false
    }
  }

  const plural = (n: number, word: string) => `${n} ${word}${n === 1 ? '' : 's'}`
  const countsLine = (p: Project) => `${plural(environmentCount(p), 'env')} · ${plural(resourceCount(p), 'resource')}`
  const sortLabel = $derived(sortOptions.find((o) => o.value === sortBy)?.label ?? 'Name A–Z')
  const listCard = 'block gap-0 overflow-hidden py-0'
  const rowAction =
    'flex size-6.5 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground'

  $effect(() => breadcrumb.set({ label: 'Projects' }))
</script>

{#snippet rowActions(project: Project)}
  {@const addHref = addResourceHref(project)}
  {@const editHref = settingsHref(project)}
  {#if addHref}
    <a href={addHref} class={rowAction} title="Add resource" aria-label="Add resource to {project.name}">
      <Icon name="plus" class="size-3.5" />
    </a>
  {/if}
  {#if editHref}
    <a href={editHref} class={rowAction} title="Project settings" aria-label="Open settings for {project.name}">
      <Icon name="settings" class="size-3.5" />
    </a>
  {/if}
{/snippet}

<div class="chrome w-full space-y-4">
  <h1 class="sr-only">Projects</h1>

  {#if session.can('manage_applications')}
    <Modal title="New Project" variant="none" bind:open={creating}>
      <form class="space-y-4" onsubmit={create}>
        <div class="grid gap-4 md:grid-cols-2">
          <Input placeholder="Your project name" label="Name" required bind:value={name} error={errors.name} />
          <Input placeholder="A short project description" label="Description" bind:value={description} error={errors.description} />
        </div>

        <p class="rounded-md border border-border bg-muted/50 px-3 py-2.5 text-xs text-muted-foreground">
          A production environment will be created automatically.
        </p>

        <footer class="flex justify-end border-t border-border pt-4">
          <Button type="submit" variant="highlighted" loading={busy}>Create project</Button>
        </footer>
      </form>
    </Modal>
  {/if}

  {#if loadError}
    <p class="text-sm text-destructive">{loadError}</p>
  {:else if projects === null}
    <PageSkeleton />
  {:else}
    <CollectionToolbar ariaLabel="Projects controls">
      {#snippet search()}
        {#if projects?.length}
          <div class="relative w-full sm:max-w-sm">
            <Icon name="search" class="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
            <UiInput
              bind:value={searchText}
              oninput={() => (page = 1)}
              type="search"
              placeholder="Search projects"
              aria-label="Search projects"
              class="h-8 pr-8 pl-8 text-sm [&::-webkit-search-cancel-button]:hidden"
            />
            {#if searchText}
              <button
                type="button"
                onclick={() => {
                  searchText = ''
                  page = 1
                }}
                class="absolute top-1/2 right-1.5 flex size-5 -translate-y-1/2 items-center justify-center rounded-sm text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                aria-label="Clear search"
              >
                <Icon name="x" class="size-3" />
              </button>
            {/if}
          </div>
        {/if}
      {/snippet}
      {#snippet controls()}
        {#if projects?.length}
          <Popover.Root>
            <Popover.Trigger class={buttonVariants({ variant: 'ghost', size: 'sm', class: 'w-fit text-xs' })} title="Sort">
              <Icon name="sort-direction" class="size-3.5 sm:size-3" />
              <span>Sort: {sortLabel}</span>
            </Popover.Trigger>
            <Popover.Content align="start" class="w-48 p-2">
              <div class="space-y-0.5" role="listbox" aria-label="Sort projects">
                {#each sortOptions as option (option.value)}
                  <button
                    type="button"
                    role="option"
                    aria-selected={sortBy === option.value}
                    class={[
                      'flex w-full items-center justify-between rounded-sm px-2 py-1.5 text-sm',
                      sortBy === option.value ? 'bg-accent/50 text-foreground' : 'text-muted-foreground hover:bg-accent/50',
                    ]}
                    onclick={() => {
                      sortBy = option.value
                      page = 1
                    }}
                  >
                    <span>{option.label}</span>
                    {#if sortBy === option.value}<Icon name="check" class="size-3 text-muted-foreground" />{/if}
                  </button>
                {/each}
              </div>
            </Popover.Content>
          </Popover.Root>
          <div class="flex items-center" role="group" aria-label="View">
            {#each [{ mode: 'table', icon: 'unordered-list', label: 'Table view' }, { mode: 'grid', icon: 'grid', label: 'Grid view' }] as const as v (v.mode)}
              <button
                type="button"
                onclick={() => setViewMode(v.mode)}
                class={buttonVariants({
                  variant: 'ghost',
                  size: 'icon-sm',
                  class: ['size-8', viewMode === v.mode ? 'bg-accent text-foreground' : 'text-muted-foreground'].join(' '),
                })}
                aria-label={v.label}
                aria-pressed={viewMode === v.mode}
                title={v.label}
              >
                <Icon name={v.icon} class="size-3.5" />
              </button>
            {/each}
          </div>
        {/if}
      {/snippet}
      {#snippet actions()}
        {#if session.can('manage_applications')}
          <button type="button" class={buttonVariants({ variant: 'outline', size: 'sm' })} onclick={() => (creating = true)}>
            <Icon name="plus" class="size-4" />
            New project
          </button>
        {/if}
      {/snippet}
    </CollectionToolbar>

    {#if projects.length === 0}
      <Empty title="No projects yet" description="Create a project to organize your environments and resources." icon="projects">
        {#if session.can('manage_applications')}
          <Button variant="highlighted" onclick={() => (creating = true)}>
            <Icon name="plus" class="size-4" />
            New project
          </Button>
        {/if}
      </Empty>
    {:else if filtered.length === 0}
      <Card class="block py-0">
        <Empty title="No matching projects" description="Try a different search." icon="search" size="sm" />
      </Card>
    {:else if viewMode === 'grid'}
      <div class="space-y-3">
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {#each paginated as project (project.id)}
            <Card interactive class="relative min-h-28 gap-0 p-4">
              <a href={href(`/project/${project.id}`)} class="absolute inset-0 rounded-lg" aria-label="Open {project.name}"></a>
              <div class="flex items-start gap-3">
                <ProjectTile size="lg" />
                <div class="min-w-0 flex-1">
                  <h2 class="truncate text-sm font-medium" title={project.name}>{project.name}</h2>
                  <p class="mt-0.5 min-h-4 truncate text-xs text-muted-foreground">{project.description || ''}</p>
                </div>
              </div>
              <div class="mt-auto flex items-center justify-between gap-3 pt-4">
                <p class="min-w-0 truncate text-xs text-muted-foreground tabular-nums">{countsLine(project)}</p>
                <div class="relative z-10 flex shrink-0 items-center gap-0.5">{@render rowActions(project)}</div>
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
        {#each paginated as project (project.id)}
          <EntityRow title={project.name} subtitle={project.description || undefined} reserveSubtitleSpace href={href(`/project/${project.id}`)} class="group">
            {#snippet leading()}<ProjectTile size="sm" />{/snippet}
            {#snippet trailing()}
              <span class="hidden text-xs text-muted-foreground tabular-nums sm:inline">{countsLine(project)}</span>
              <div class="flex items-center gap-0.5">{@render rowActions(project)}</div>
            {/snippet}
          </EntityRow>
        {/each}
        <ClientPagination bind:page bind:pageSize total={filtered.length} options={[12, 24, 48, 96]} storageKey={pageSizeKey} />
      </Card>
    {/if}
  {/if}
</div>
