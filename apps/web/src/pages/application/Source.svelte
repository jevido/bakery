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
  import Icon from '../../lib/Icon.svelte'
  import { session } from '../../lib/session.svelte'
  import type { Application } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import Input from '../../lib/ui/Input.svelte'
  import SettingsSection from '../../lib/ui/SettingsSection.svelte'
  import Textarea from '../../lib/ui/Textarea.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import UnsavedBar from '../../lib/ui/UnsavedBar.svelte'
  import { applicationInput } from './applicationInput'

  let { application, onchange }: { application: Application; onchange: (a: Application) => void } = $props()

  const canUpdate = $derived(session.canWrite)

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

<div class="application-settings-form flex flex-col gap-6">
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

    <SettingsSection id="repository-section" title="Repository" helper="Configure the Git repository and branch The Bakery deploys.">
      {#snippet actions()}
        <div class="flex flex-wrap items-center gap-2">
          <a target="_blank" rel="noreferrer" class="button" href={branchLocation}>
            Repository
            <Icon name="external-link" class="size-3.5" />
          </a>
          <a target="_blank" rel="noreferrer" class="button" href={commits}>
            Commits
            <Icon name="external-link" class="size-3.5" />
          </a>
        </div>
      {/snippet}

      {#if !application.deploy_key_public}
        <div
          class="mb-4 flex items-center justify-between gap-3 rounded-lg bg-neutral-50 px-3 py-2.5 ring-1 ring-neutral-200 dark:bg-white/[0.05] dark:ring-white/[0.07]"
        >
          <span class="text-[12px] text-neutral-500 dark:text-fg-dim">Connected source</span>
          <span class="text-[12px] font-medium text-neutral-900 dark:text-fg">Public repository</span>
        </div>
      {/if}

      <div class="grid gap-4 lg:grid-cols-2">
        <Input
          label="Repository"
          bind:value={gitUrl}
          error={errors.git_url}
          required
          placeholder="https://github.com/coollabsio/coolify-example"
          helper="https:// for a public repository, SSH (git@host:owner/repo.git) for a private one, cloned with the deploy key below."
          disabled={!canUpdate}
        />
        <Input label="Branch" bind:value={gitBranch} error={errors.git_branch} placeholder="main" disabled={!canUpdate} />
      </div>
    </SettingsSection>
  </form>

  {#if application.deploy_key_public}
    <SettingsSection id="deploy-key-section" title="Deploy key" helper="The SSH key The Bakery uses to clone this private repository.">
      <div
        class="mb-4 flex items-center justify-between gap-3 rounded-lg bg-neutral-50 px-3 py-2.5 ring-1 ring-neutral-200 dark:bg-white/[0.05] dark:ring-white/[0.07]"
      >
        <span class="text-[12px] text-neutral-500 dark:text-fg-dim">Attached private key</span>
        <span class="text-[12px] font-medium text-neutral-900 dark:text-fg">Deploy key of {application.name}</span>
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
      <div class="mt-4 flex flex-wrap items-center gap-2">
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
    </SettingsSection>
  {/if}
</div>
