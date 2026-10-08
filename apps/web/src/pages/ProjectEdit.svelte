<script lang="ts">
  // Coolify's Project settings (resources/views/livewire/project/edit.blade.php
  // and delete-project.blade.php, app/Livewire/Project/Edit.php and
  // DeleteProject.php; Apache-2.0, see NOTICE): the Project's name and
  // description behind the unsaved-changes bar, and Delete project once the
  // Project is empty.
  //
  // Left out: the Project icon section (Projects have no icons here). Added:
  // the Project's shared variables, until the Shared Variables pages arrive.
  //
  // Laid out as a Paperclip settings page (ui/src/pages/CompanySettings.tsx;
  // MIT, see NOTICE): a General group and a Danger Zone group. It stays in
  // the primary sidebar under its Project, not among the settings pages.
  import { FolderCog } from '@lucide/svelte'
  import { buttonVariants } from '@bakery/ui/components/ui/button'
  import { api, ApiError } from '../lib/api'
  import { breadcrumb } from '../lib/breadcrumb.svelte'
  import EnvironmentVariables from '../lib/EnvironmentVariables.svelte'
  import Icon from '../lib/Icon.svelte'
  import PageSkeleton from '../lib/PageSkeleton.svelte'
  import { projectResources, type ProjectResources } from '../lib/projectCounts'
  import { go, href } from '../lib/router.svelte'
  import { projectAccess } from '../lib/projectAccess.svelte'
  import { session } from '../lib/session.svelte'
  import SettingsGroup from '../lib/settings/SettingsGroup.svelte'
  import SettingsPage from '../lib/settings/SettingsPage.svelte'
  import type { Project } from '../lib/types'
  import ConfirmationModal from '../lib/ui/ConfirmationModal.svelte'
  import Input from '../lib/ui/Input.svelte'
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
  const dirty = $derived(!!project && projectAccess.can('manage_applications') && (name !== project.name || description !== (project.description ?? '')))
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
  <p class="text-sm text-destructive">{loadError}</p>
{:else if !project}
  <PageSkeleton />
{:else}
  <SettingsPage icon={FolderCog} title="Project settings">
    {#snippet actions()}
      {#if session.can('manage_roles')}
        <a href={href(`/project/${id}/permissions`)} class={buttonVariants({ variant: 'outline', size: 'sm' })}>
          <Icon name="lock" class="size-3.5" />
          Permissions
        </a>
      {/if}
    {/snippet}
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
      <SettingsGroup label="General" hint="Name and describe this project across the dashboard." data-testid="project-general">
        <Input label="Name" bind:value={name} error={errors.name} disabled={!projectAccess.can('manage_applications')} />
        <Input label="Description" bind:value={description} error={errors.description} disabled={!projectAccess.can('manage_applications')} />
      </SettingsGroup>
    </form>

    <EnvironmentVariables
      path={`/projects/${id}/variables`}
      title="Shared variables"
      helper="Every application in this project gets these, unless its environment or the application sets the same name."
    />

    {#if projectAccess.can('manage_applications')}
      <SettingsGroup label="Danger Zone" destructive data-testid="project-delete">
        <p class="text-sm text-muted-foreground">
          Permanently delete <strong class="font-medium text-foreground">{project.name}</strong> and its environments. Empty the project before
          deleting it.
        </p>
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
      </SettingsGroup>
    {/if}
  </SettingsPage>
{/if}
