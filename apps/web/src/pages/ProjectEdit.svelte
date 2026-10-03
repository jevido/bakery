<script lang="ts">
  // Coolify's Project settings (resources/views/livewire/project/edit.blade.php
  // and delete-project.blade.php, app/Livewire/Project/Edit.php and
  // DeleteProject.php; Apache-2.0, see NOTICE): the Project's name and
  // description behind the unsaved-changes bar, and Delete project once the
  // Project is empty.
  //
  // Left out: the Project icon section (Projects have no icons here). Added:
  // the Project's shared variables, until the Shared Variables pages arrive.
  import { api, ApiError } from '../lib/api'
  import { breadcrumb } from '../lib/breadcrumb.svelte'
  import EnvironmentVariables from '../lib/EnvironmentVariables.svelte'
  import { projectResources, type ProjectResources } from '../lib/projectCounts'
  import { go, href } from '../lib/router.svelte'
  import { session } from '../lib/session.svelte'
  import type { Project } from '../lib/types'
  import ConfirmationModal from '../lib/ui/ConfirmationModal.svelte'
  import Input from '../lib/ui/Input.svelte'
  import Spinner from '../lib/ui/Spinner.svelte'
  import { toast } from '../lib/ui/toast.svelte'
  import UnsavedBar from '../lib/ui/UnsavedBar.svelte'

  let { id }: { id: number } = $props()

  let resources = $state.raw<ProjectResources | null>(null)
  let loadError = $state('')
  let name = $state('')
  let description = $state('')
  let errors = $state<Record<string, string>>({})
  let saving = $state(false)

  $effect(() => {
    resources = null
    loadError = ''
    projectResources(id)
      .then((r) => {
        resources = r
        reset()
      })
      .catch((e) => (loadError = e.message))
  })

  const project = $derived(resources?.project)
  const dirty = $derived(!!project && session.canWrite && (name !== project.name || description !== (project.description ?? '')))
  const empty = $derived(
    !!resources &&
      resources.databases.length === 0 &&
      resources.services.length === 0 &&
      (resources.project.environments ?? []).every((e) => e.applications.length === 0),
  )

  function reset() {
    if (!resources) return
    name = resources.project.name
    description = resources.project.description ?? ''
    errors = {}
  }

  async function save() {
    if (!resources || saving) return
    saving = true
    errors = {}
    try {
      const { project: saved } = await api<{ project: Project }>('PATCH', `/projects/${id}`, { name, description })
      resources = { ...resources, project: { ...resources.project, name: saved.name, description: saved.description } }
      reset()
      toast.success('Project updated.')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      errors = Object.keys(err.errors).length ? err.errors : { name: err.message }
    } finally {
      saving = false
    }
  }

  async function remove() {
    try {
      await api('DELETE', `/projects/${id}`)
      go('/projects')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      toast.error(err.message)
    }
  }

  const crumbName = $derived(project?.name)
  $effect(() => {
    if (crumbName !== undefined)
      breadcrumb.set({ label: 'Projects', href: href('/projects') }, { label: crumbName, href: href(`/project/${id}`) }, { label: 'Settings' })
  })
</script>

{#if loadError}
  <p class="text-sm text-error">{loadError}</p>
{:else if !project}
  <Spinner text="Loading…" />
{:else}
  <div class="chrome w-full max-w-none">
    <header class="mb-5">
      <h1 class="truncate text-[24px]! leading-7! font-semibold! tracking-tight!">{project.name}</h1>
      <p class="mt-1 text-[13px] text-neutral-500 dark:text-fg-dim">Project settings</p>
    </header>

    <div class="flex flex-col gap-6">
      <form
        onsubmit={(e) => {
          e.preventDefault()
          save()
        }}
      >
        {#if session.canWrite}
          <UnsavedBar {dirty} {saving} onsave={save} onreset={reset} />
        {/if}
        <section class="application-settings-section">
          <div class="application-settings-section-header">
            <div>
              <h2>Project details</h2>
              <p>Name and describe this project across the dashboard.</p>
            </div>
          </div>
          <div class="application-settings-section-body grid gap-4 sm:grid-cols-2">
            <Input label="Name" bind:value={name} error={errors.name} disabled={!session.canWrite} />
            <Input label="Description" bind:value={description} error={errors.description} disabled={!session.canWrite} />
          </div>
        </section>
      </form>

      <section class="application-settings-section">
        <div class="application-settings-section-header">
          <div>
            <h2>Shared variables</h2>
            <p>Every application in this project gets these, unless its environment or the application sets the same name.</p>
          </div>
        </div>
        <div class="application-settings-section-body">
          <EnvironmentVariables path={`/projects/${id}/variables`} description="Values are stored encrypted and apply on the next deploy." />
        </div>
      </section>

      {#if session.canWrite}
        <section class="overflow-hidden rounded-[10px] border border-red-300 bg-red-50/80 dark:border-red-500/25 dark:bg-red-500/[0.06]">
          <div class="flex items-start justify-between gap-4 px-5 py-4">
            <div>
              <h2 class="text-sm font-semibold text-red-800 dark:text-red-300">Delete project</h2>
              <p class="mt-1 max-w-2xl text-sm text-red-700/80 dark:text-red-200/70">Empty the project before permanently deleting it.</p>
            </div>
            <ConfirmationModal
              title="Confirm Project Deletion?"
              buttonTitle="Delete Project"
              variant="error"
              disabled={!empty}
              actions={['This will delete the selected project', 'All Environments inside the project will be deleted as well.']}
              confirmationText={project.name}
              confirmationLabel="Please confirm the execution of the actions by entering the Project Name below"
              shortConfirmationLabel="Project Name"
              step2ButtonText="Permanently Delete"
              onconfirm={remove}
            />
          </div>
        </section>
      {/if}
    </div>
  </div>
{/if}
