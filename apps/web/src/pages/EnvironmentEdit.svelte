<script lang="ts">
  // Coolify's Environment settings (resources/views/livewire/project/environment-edit.blade.php
  // and delete-environment.blade.php, app/Livewire/Project/EnvironmentEdit.php
  // and DeleteEnvironment.php; Apache-2.0, see NOTICE): the Environment's name
  // and description behind the unsaved-changes bar, and Delete environment
  // once it holds no Resources.
  //
  // Left out: Clone environment (no cloning yet). Added: the Environment's
  // shared variables, until the Shared Variables pages arrive.
  //
  // Laid out as a Paperclip settings page (ui/src/pages/CompanySettings.tsx;
  // MIT, see NOTICE), as Project settings is: a General group and a Danger
  // Zone group, in the primary sidebar under its Project.
  import { Layers } from '@lucide/svelte'
  import { api, ApiError } from '../lib/api'
  import { breadcrumb } from '../lib/breadcrumb.svelte'
  import EnvironmentVariables from '../lib/EnvironmentVariables.svelte'
  import { environmentResourceCount, projectResources, type ProjectResources } from '../lib/projectCounts'
  import { go, href } from '../lib/router.svelte'
  import PageSkeleton from '../lib/PageSkeleton.svelte'
  import { projectAccess } from '../lib/projectAccess.svelte'
  import SettingsGroup from '../lib/settings/SettingsGroup.svelte'
  import SettingsPage from '../lib/settings/SettingsPage.svelte'
  import type { Environment } from '../lib/types'
  import ConfirmationModal from '../lib/ui/ConfirmationModal.svelte'
  import Input from '../lib/ui/Input.svelte'
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
  <p class="text-sm text-destructive">{loadError}</p>
{:else if !project || !environment}
  <PageSkeleton />
{:else}
  <SettingsPage icon={Layers} title="Environment settings">
    <form
      class="space-y-8"
      onsubmit={(e) => {
        e.preventDefault()
        save()
      }}
    >
      {#if projectAccess.can('manage_applications')}
        <UnsavedBar {dirty} {saving} onsave={save} onreset={reset} />
      {/if}
      <SettingsGroup label="General" hint="Name and describe this environment inside {project.name}." data-testid="environment-general">
        <Input label="Name" bind:value={name} error={errors.name} disabled={!projectAccess.can('manage_applications')} />
        <Input label="Description" bind:value={description} error={errors.description} disabled={!projectAccess.can('manage_applications')} />
      </SettingsGroup>
    </form>

    <EnvironmentVariables
      path={`/environments/${id}/variables`}
      title="Shared variables"
      helper={`Every application in ${environment.name} gets these, unless it sets the same name. They win over the project's.`}
    />

    {#if projectAccess.can('manage_applications')}
      <SettingsGroup label="Danger Zone" destructive data-testid="environment-delete">
        <p class="text-sm text-muted-foreground">
          Permanently delete <strong class="font-medium text-foreground">{environment.name}</strong>. Remove every resource before deleting
          this environment.
        </p>
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
      </SettingsGroup>
    {/if}
  </SettingsPage>
{/if}
