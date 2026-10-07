<script lang="ts">
  // Coolify's Environment settings (resources/views/livewire/project/environment-edit.blade.php
  // and delete-environment.blade.php, app/Livewire/Project/EnvironmentEdit.php
  // and DeleteEnvironment.php; Apache-2.0, see NOTICE): the Environment's name
  // and description behind the unsaved-changes bar, and Delete environment
  // once it holds no Resources.
  //
  // Left out: Clone environment (no cloning yet). Added: the Environment's
  // shared variables, until the Shared Variables pages arrive.
  import { api, ApiError } from '../lib/api'
  import { breadcrumb } from '../lib/breadcrumb.svelte'
  import EnvironmentVariables from '../lib/EnvironmentVariables.svelte'
  import { environmentResourceCount, projectResources, type ProjectResources } from '../lib/projectCounts'
  import { go, href } from '../lib/router.svelte'
  import { projectAccess } from '../lib/projectAccess.svelte'
  import type { Environment } from '../lib/types'
  import ConfirmationModal from '../lib/ui/ConfirmationModal.svelte'
  import Input from '../lib/ui/Input.svelte'
  import Spinner from '../lib/ui/Spinner.svelte'
  import { toast } from '../lib/ui/toast.svelte'
  import UnsavedBar from '../lib/ui/UnsavedBar.svelte'

  let { projectId, id }: { projectId: number; id: number } = $props()

  let resources = $state.raw<ProjectResources | null>(null)
  let loadError = $state('')
  let name = $state('')
  let description = $state('')
  let errors = $state<Record<string, string>>({})
  let saving = $state(false)

  $effect(() => {
    resources = null
    loadError = ''
    projectResources(projectId)
      .then((r) => {
        resources = r
        if (!r.project.environments?.some((e) => e.id === id)) loadError = 'Environment not found.'
        reset()
      })
      .catch((e) => (loadError = e.message))
  })

  const project = $derived(resources?.project)
  const environment = $derived(project?.environments?.find((e) => e.id === id))
  const dirty = $derived(
    !!environment && projectAccess.can('manage_applications') && (name !== environment.name || description !== (environment.description ?? '')),
  )
  const empty = $derived(!!resources && environmentResourceCount(resources, id) === 0)

  function reset() {
    const e = resources?.project.environments?.find((e) => e.id === id)
    if (!e) return
    name = e.name
    description = e.description ?? ''
    errors = {}
  }

  async function save() {
    if (!resources || saving) return
    saving = true
    errors = {}
    try {
      const { environment: saved } = await api<{ environment: Environment }>('PATCH', `/environments/${id}`, { name, description })
      const environments = (resources.project.environments ?? []).map((e) =>
        e.id === id ? { ...e, name: saved.name, description: saved.description } : e,
      )
      resources = { ...resources, project: { ...resources.project, environments } }
      reset()
      toast.success('Environment updated.')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      errors = Object.keys(err.errors).length ? err.errors : { name: err.message }
    } finally {
      saving = false
    }
  }

  async function remove() {
    try {
      await api('DELETE', `/environments/${id}`)
      go(`/project/${projectId}`)
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      toast.error(err.message)
    }
  }

  const crumbs = $derived(project && environment ? { project: project.name, environment: environment.name } : undefined)
  $effect(() => {
    if (crumbs)
      breadcrumb.set(
        { label: 'Projects', href: href('/projects') },
        { label: crumbs.project, href: href(`/project/${projectId}`) },
        { label: crumbs.environment, href: href(`/project/${projectId}/environment/${id}`) },
        { label: 'Settings' },
      )
  })
</script>

{#if loadError}
  <p class="text-sm text-error">{loadError}</p>
{:else if !project || !environment}
  <Spinner text="Loading…" />
{:else}
  <div class="chrome w-full max-w-none">
    <header class="mb-5">
      <h1 class="truncate text-[24px]! leading-7! font-semibold! tracking-tight!">{environment.name}</h1>
      <p class="mt-1 text-[13px] text-neutral-500 dark:text-fg-dim">Environment settings in {project.name}</p>
    </header>

    <div class="flex flex-col gap-6">
      <form
        onsubmit={(e) => {
          e.preventDefault()
          save()
        }}
      >
        {#if projectAccess.can('manage_applications')}
          <UnsavedBar {dirty} {saving} onsave={save} onreset={reset} />
        {/if}
        <section class="application-settings-section">
          <div class="application-settings-section-header">
            <div>
              <h2>Environment details</h2>
              <p>Name and describe this environment inside {project.name}.</p>
            </div>
          </div>
          <div class="application-settings-section-body grid gap-4 sm:grid-cols-2">
            <Input label="Name" bind:value={name} error={errors.name} disabled={!projectAccess.can('manage_applications')} />
            <Input label="Description" bind:value={description} error={errors.description} disabled={!projectAccess.can('manage_applications')} />
          </div>
        </section>
      </form>

      <EnvironmentVariables
        path={`/environments/${id}/variables`}
        title="Shared variables"
        helper={`Every application in ${environment.name} gets these, unless it sets the same name. They win over the project's.`}
      />

      {#if projectAccess.can('manage_applications')}
        <section class="overflow-hidden rounded-[10px] border border-red-300 bg-red-50/80 dark:border-red-500/25 dark:bg-red-500/[0.06]">
          <div class="flex flex-col gap-4 px-5 py-4 sm:flex-row sm:items-start sm:justify-between">
            <div class="min-w-0">
              <h2 class="text-sm font-semibold text-red-800 dark:text-red-300">Delete environment</h2>
              <p class="mt-1 max-w-2xl text-sm text-red-700/80 dark:text-red-200/70">
                Remove every resource before permanently deleting this environment.
              </p>
            </div>
            <div class="shrink-0 sm:pt-0.5">
              <ConfirmationModal
                title="Confirm Environment Deletion?"
                buttonTitle="Delete"
                variant="error"
                disabled={!empty}
                actions={['This will delete the selected environment.']}
                confirmationText={environment.name}
                confirmationLabel="Please confirm the execution of the actions by entering the Environment Name below"
                shortConfirmationLabel="Environment Name"
                step2ButtonText="Permanently Delete"
                onconfirm={remove}
              />
            </div>
          </div>
        </section>
      {/if}
    </div>
  </div>
{/if}
