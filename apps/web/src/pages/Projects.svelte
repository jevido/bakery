<script lang="ts">
  // Coolify's Projects page (resources/views/livewire/project/index.blade.php
  // with its projectsIndex() script, app/Livewire/Project/Index.php, and the
  // New Project form of add-empty.blade.php and AddEmpty.php; Apache-2.0, see
  // NOTICE): search, Sort, a remembered table/grid switch and client-side
  // pages, with "New project" in a modal.
  //
  // Left out: project icons (Projects have none here) and the empty state's
  // "Open onboarding" link, until onboarding exists.
  import { api, ApiError } from '../lib/api'
  import { breadcrumb } from '../lib/breadcrumb.svelte'
  import Icon from '../lib/Icon.svelte'
  import { projectCounts, type ProjectCounts } from '../lib/projectCounts'
  import { go, href } from '../lib/router.svelte'
  import { session } from '../lib/session.svelte'
  import type { Project } from '../lib/types'
  import Button from '../lib/ui/Button.svelte'
  import ClientPagination from '../lib/ui/ClientPagination.svelte'
  import Empty from '../lib/ui/Empty.svelte'
  import Input from '../lib/ui/Input.svelte'
  import Modal from '../lib/ui/Modal.svelte'
  import Spinner from '../lib/ui/Spinner.svelte'
  import TableDropdown from '../lib/ui/TableDropdown.svelte'

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

  let search = $state('')
  let sortBy = $state<SortKey>('name-asc')
  let viewMode = $state<ViewMode>(localStorage.getItem(viewKey) === 'table' ? 'table' : 'grid')
  let page = $state(1)
  let pageSize = $state(12)

  // The search box filters 150 ms after the last key, as Coolify's debounce does.
  let query = $state('')
  $effect(() => {
    const value = search
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

  const addResourceHref = (p: Project) => {
    const first = counts[p.id]?.firstEnvironment
    return first && session.can('manage_applications') ? href(`/project/${p.id}/environment/${first}/new`) : null
  }
  const settingsHref = (p: Project) => (session.can('manage_applications') ? href(`/project/${p.id}/edit`) : null)

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

  const rowAction =
    'flex items-center justify-center rounded-md text-neutral-400 transition-colors hover:bg-neutral-100 hover:text-black dark:text-fg-faint dark:hover:bg-white/[0.06] dark:hover:text-fg'
  const toggleIdle =
    'text-neutral-400 hover:bg-neutral-100 hover:text-black dark:text-fg-faint dark:hover:bg-white/[0.06] dark:hover:text-fg'

  $effect(() => breadcrumb.set({ label: 'Projects' }))
</script>

<div class="chrome application-settings-form w-full">
  <header class="mb-5 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
    <h1 class="min-w-0 truncate text-[24px]! leading-7! font-semibold! tracking-tight!">Projects</h1>
    {#if session.can('manage_applications')}
      <div class="w-fit shrink-0">
        <Modal title="New Project" bind:open={creating}>
          {#snippet trigger(show)}
            <button type="button" class="button button-highlighted" onclick={show}>
              <Icon name="plus" class="size-3.5" />
              New project
            </button>
          {/snippet}
          <form class="space-y-4" onsubmit={create}>
            <div class="grid gap-4 md:grid-cols-2">
              <Input placeholder="Your project name" label="Name" required bind:value={name} error={errors.name} />
              <Input placeholder="A short project description" label="Description" bind:value={description} error={errors.description} />
            </div>

            <p
              class="rounded-lg border border-neutral-200 bg-neutral-50 px-3 py-2.5 text-[12px] text-neutral-500 dark:border-white/[0.08] dark:bg-white/[0.05] dark:text-fg-dim"
            >
              A production environment will be created automatically.
            </p>

            <footer class="flex justify-end border-t border-neutral-200 pt-4 dark:border-white/[0.08]">
              <Button type="submit" variant="highlighted" loading={busy}>Create project</Button>
            </footer>
          </form>
        </Modal>
      </div>
    {/if}
  </header>

  {#if loadError}
    <p class="text-sm text-error">{loadError}</p>
  {:else if projects === null}
    <Spinner text="Loading…" />
  {:else if projects.length === 0}
    <Empty title="No projects yet" description="Create a project to organize your environments and resources." icon="projects" />
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
          placeholder="Search projects"
          aria-label="Search projects"
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
        <TableDropdown panelClass="w-52!">
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

    {#if filtered.length === 0}
      <div
        class="flex min-h-52 flex-col items-center justify-center rounded-xl border border-neutral-200 bg-white px-6 text-center dark:border-white/[0.08] dark:bg-white/[0.05]"
      >
        <Icon name="search" class="mb-3 size-6 text-neutral-300 dark:text-fg-faint" />
        <p class="text-[13px] font-medium">No matching projects</p>
        <p class="mt-1 text-[12px] text-neutral-500 dark:text-fg-dim">Try a different search.</p>
      </div>
    {:else if viewMode === 'grid'}
      <div>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {#each paginated as project (project.id)}
            {@const addHref = addResourceHref(project)}
            {@const editHref = settingsHref(project)}
            {@const envs = environmentCount(project)}
            {@const resources = resourceCount(project)}
            <article
              class="group relative flex min-h-28 flex-col rounded-xl border border-neutral-200 bg-white p-3 shadow-sm transition-all hover:-translate-y-px hover:border-neutral-300 hover:shadow-md dark:border-white/[0.08] dark:bg-white/[0.05] dark:hover:border-white/[0.14]"
            >
              <a href={href(`/project/${project.id}`)} class="absolute inset-0 rounded-xl" aria-label="Open {project.name}"></a>
              <div class="flex items-start gap-3">
                <div
                  class="flex size-8 shrink-0 items-center justify-center rounded-lg border border-neutral-200 bg-neutral-50 text-neutral-500 dark:border-white/[0.08] dark:bg-white/[0.04] dark:text-fg-dim"
                >
                  <Icon name="projects" class="size-4" />
                </div>
                <div class="min-w-0 flex-1">
                  <h2 class="truncate text-[13px]! leading-4! font-semibold! text-black dark:text-fg">{project.name}</h2>
                  <p class="mt-0.5 truncate text-[11px] text-neutral-500 dark:text-fg-faint">{project.description || ''}</p>
                </div>
              </div>

              <div class="mt-auto flex items-center justify-between gap-3 pt-4">
                <p class="min-w-0 truncate text-[11px] text-neutral-500 dark:text-fg-dim">
                  <span>{envs} {envs === 1 ? 'env' : 'envs'}</span>
                  <span class="px-1 text-neutral-300 dark:text-white/15">·</span>
                  <span>{resources} {resources === 1 ? 'resource' : 'resources'}</span>
                </p>

                <div class="relative z-10 flex shrink-0 items-center gap-0.5">
                  {#if addHref}
                    <a href={addHref} class={['size-7.5', rowAction]} title="Add resource" aria-label="Add resource to {project.name}">
                      <Icon name="plus" class="size-3" />
                    </a>
                  {/if}
                  {#if editHref}
                    <a href={editHref} class={['size-7.5', rowAction]} title="Project settings" aria-label="Open settings for {project.name}">
                      <Icon name="settings" class="size-3" />
                    </a>
                  {/if}
                </div>
              </div>
            </article>
          {/each}
        </div>
        <ClientPagination
          bind:page
          bind:pageSize
          total={filtered.length}
          options={[12, 24, 48, 96]}
          storageKey={pageSizeKey}
          class="mt-3 rounded-xl border border-neutral-200 bg-white shadow-sm dark:border-white/[0.08] dark:bg-white/[0.05]"
        />
      </div>
    {:else}
      <div class="overflow-hidden rounded-xl border border-neutral-200 bg-white shadow-sm dark:border-white/[0.08] dark:bg-white/[0.05]">
        <div
          class="projects-table-grid border-b border-neutral-200 bg-neutral-50 px-4 py-2.5 text-[11px] font-medium text-neutral-500 dark:border-white/[0.08] dark:bg-white/[0.05] dark:text-fg-faint"
        >
          <div>Project</div>
          <div>Environments</div>
          <div>Resources</div>
          <div class="project-description">Description</div>
          <div></div>
        </div>

        {#each paginated as project (project.id)}
          {@const addHref = addResourceHref(project)}
          {@const editHref = settingsHref(project)}
          <div
            class="projects-table-grid group min-h-14 items-center border-b border-neutral-200 px-4 py-2.5 transition-colors last:border-b-0 hover:bg-neutral-50 dark:border-white/[0.07] dark:hover:bg-white/[0.025]"
          >
            <div class="flex min-w-0 items-center gap-3">
              <div
                class="flex size-8 shrink-0 items-center justify-center rounded-lg border border-neutral-200 bg-neutral-50 text-neutral-500 dark:border-white/[0.08] dark:bg-white/[0.035] dark:text-fg-dim"
              >
                <Icon name="projects" class="size-4" />
              </div>
              <a href={href(`/project/${project.id}`)} class="truncate text-[13px] font-semibold text-black hover:underline dark:text-fg">{project.name}</a>
            </div>

            <div class="text-[12px] text-neutral-600 dark:text-fg-dim">{environmentCount(project)}</div>
            <div class="text-[12px] text-neutral-600 dark:text-fg-dim">{resourceCount(project)}</div>
            <p class="project-description truncate text-[12px] text-neutral-500 dark:text-fg-dim">{project.description || '-'}</p>

            <div class="flex items-center justify-end gap-0.5">
              {#if addHref}
                <a href={addHref} class={['size-7', rowAction]} title="Add resource" aria-label="Add resource to {project.name}">
                  <Icon name="plus" class="size-3.5" />
                </a>
              {/if}
              {#if editHref}
                <a href={editHref} class={['size-7', rowAction]} title="Project settings" aria-label="Open settings for {project.name}">
                  <Icon name="settings" class="size-3.5" />
                </a>
              {/if}
            </div>
          </div>
        {/each}
        <ClientPagination bind:page bind:pageSize total={filtered.length} options={[12, 24, 48, 96]} storageKey={pageSizeKey} />
      </div>
    {/if}
  {/if}
</div>
