<script lang="ts">
  // Coolify's Git Source (resources/views/livewire/project/application/source.blade.php
  // and app/Livewire/Project/Application/Source.php, Apache-2.0, see NOTICE):
  // the repository and branch with links to them, and the deploy key a
  // private repository is cloned with. Coolify attaches one of the team's
  // Private Keys; The Bakery generates a key for each Application, so this
  // shows its public half and Regenerate instead of a list to switch to.
  // Coolify's Commit SHA and "Git source" switch (GitHub and GitLab Apps)
  // wait for Sources.
  import { untrack } from 'svelte'
  import { api, ApiError } from '../../lib/api'
  import { buttonVariants } from '$lib/components/ui/button'
  import Icon from '../../lib/Icon.svelte'
  import { projectAccess } from '../../lib/projectAccess.svelte'
  import type { Application } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import Input from '../../lib/ui/Input.svelte'
  import Textarea from '../../lib/ui/Textarea.svelte'
  import SettingsGroup from '../../lib/settings/SettingsGroup.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import UnsavedBar from '../../lib/ui/UnsavedBar.svelte'
  import { applicationInput } from './applicationInput'

  let { application, onchange }: { application: Application; onchange: (a: Application) => void } = $props()

  const canUpdate = $derived(projectAccess.can('manage_applications'))

  let gitUrl = $state('')
  let gitBranch = $state('')
  let errors = $state<Record<string, string>>({})
  let saving = $state(false)

  function reset() {
    gitUrl = application.git_url
    gitBranch = application.git_branch
    errors = {}
  }

  $effect(() => {
    void application.id
    untrack(reset)
  })

  const dirty = $derived(canUpdate && (gitUrl !== application.git_url || gitBranch !== application.git_branch))

  async function save() {
    if (saving || !dirty) return
    saving = true
    errors = {}
    try {
      const r = await api<{ application: Application }>('PATCH', `/applications/${application.id}`, {
        ...applicationInput(application),
        git_url: gitUrl,
        git_branch: gitBranch,
      })
      onchange(r.application)
      reset()
      toast.success('Application source updated!')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      errors = Object.keys(err.errors).length ? err.errors : { git_url: err.message }
    } finally {
      saving = false
    }
  }

  // Coolify's gitBranchLocation and gitCommits: the repository's web page
  // from an https or scp-style URL (git@host:owner/repo.git).
  const web = $derived.by(() => {
    const url = application.git_url
    const m = url.match(/^https?:\/\/([^/]+)\/(.+?)(?:\.git)?\/?$/) ?? url.match(/^(?:ssh:\/\/)?[^@]+@([^:/]+)[:/](.+?)(?:\.git)?$/)
    return m ? `https://${m[1]}/${m[2]}` : null
  })
  const bitbucket = $derived(application.git_url.includes('bitbucket'))
  const branchLocation = $derived(web ? `${web}/${bitbucket ? 'src' : 'tree'}/${application.git_branch}` : application.git_url)
  const commits = $derived(web ? `${web}/commits/${application.git_branch}` : application.git_url)

  async function regenerate() {
    try {
      const r = await api<{ application: Application }>('POST', `/applications/${application.id}/deploy-key`)
      onchange(r.application)
      toast.success('Private key updated!', 'Add the new public key to the repository.')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      toast.error('Deploy key not regenerated', err.message)
    }
  }

  async function copyKey() {
    if (!navigator.clipboard?.writeText) {
      toast.error('Clipboard is not available. Please use HTTPS or localhost.')
      return
    }
    await navigator.clipboard.writeText(application.deploy_key_public)
    toast.success('Public key copied to clipboard.')
  }
</script>

<div class="space-y-8">
  <form
    class="flex flex-col gap-6"
    onsubmit={(e) => {
      e.preventDefault()
      save()
    }}
  >
    {#if canUpdate}
      <UnsavedBar {dirty} {saving} onsave={save} onreset={reset} />
    {/if}

    <SettingsGroup id="repository-section" label="Repository" hint="Configure the Git repository and branch The Bakery deploys.">
      {#snippet actions()}
        <a target="_blank" rel="noreferrer" class={buttonVariants({ variant: 'outline', size: 'sm' })} href={branchLocation}>
          Repository
          <Icon name="external-link" class="size-3.5" />
        </a>
        <a target="_blank" rel="noreferrer" class={buttonVariants({ variant: 'outline', size: 'sm' })} href={commits}>
          Commits
          <Icon name="external-link" class="size-3.5" />
        </a>
      {/snippet}

      {#if !application.deploy_key_public}
        <div
          class="flex items-center justify-between gap-3 rounded-md border border-border bg-muted/30 px-3 py-2.5"
        >
          <span class="text-xs text-muted-foreground">Connected source</span>
          <span class="text-xs font-medium text-foreground">Public repository</span>
        </div>
      {/if}

      <div class="grid gap-3 sm:grid-cols-2">
        <Input
          label="Repository"
          bind:value={gitUrl}
          error={errors.git_url}
          required
          placeholder="https://github.com/owner/repository"
          helper="https:// for a public repository, SSH (git@host:owner/repo.git) for a private one, cloned with the deploy key below."
          disabled={!canUpdate}
        />
        <Input label="Branch" bind:value={gitBranch} error={errors.git_branch} placeholder="main" disabled={!canUpdate} />
      </div>
    </SettingsGroup>
  </form>

  {#if application.deploy_key_public}
    <SettingsGroup id="deploy-key-section" label="Deploy key" hint="The SSH key The Bakery uses to clone this private repository.">
      <div
        class="flex items-center justify-between gap-3 rounded-md border border-border bg-muted/30 px-3 py-2.5"
      >
        <span class="text-xs text-muted-foreground">Attached private key</span>
        <span class="text-xs font-medium text-foreground">Deploy key of {application.name}</span>
      </div>
      <Textarea
        label="Public key"
        value={application.deploy_key_public}
        readonly
        monospace
        rows={3}
        helper="Add it to the repository as a read-only deploy key (GitHub: Settings → Deploy keys; GitLab: Settings → Repository → Deploy keys; Gitea and Forgejo: Settings → Deploy keys)."
        data-testid="deploy-key"
      />
      <div class="flex flex-wrap items-center gap-2">
        <Button onclick={copyKey}>Copy public key</Button>
        {#if canUpdate}
          <ConfirmationModal
            title="Regenerate deploy key?"
            buttonTitle="Regenerate"
            actions={['A new key pair is generated for this application.', 'The current key stops working; deploys fail until the new public key is added to the repository.']}
            confirmWithText={false}
            onconfirm={regenerate}
          >
            {#snippet trigger(show)}
              <Button onclick={show}>Regenerate</Button>
            {/snippet}
          </ConfirmationModal>
        {/if}
      </div>
    </SettingsGroup>
  {/if}
</div>
