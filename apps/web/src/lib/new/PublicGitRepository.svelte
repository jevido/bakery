<script lang="ts">
  // Coolify's public-git-repository page (resources/views/livewire/project/new/
  // public-git-repository.blade.php, app/Livewire/Project/New/PublicGitRepository.php;
  // Apache-2.0, see NOTICE). "Check repository" checks the URL only: The
  // Bakery has no Git provider API to look the branch up, so the build
  // configuration opens with the branch of a `/tree/<branch>` link, else main.
  // The helper's examples and the "Need a sample?" link pointed at Coolify's
  // own example repositories; the examples are generic here and the link is
  // left out.
  import { api } from '../api'
  import { go } from '../router.svelte'
  import type { Application, BuildPack } from '../types'
  import Button from '../ui/Button.svelte'
  import Input from '../ui/Input.svelte'
  import BuildConfiguration from './BuildConfiguration.svelte'
  import { attempt } from './create'
  import { publicRepository } from './parse'

  let { environmentId, server }: { environmentId: number; server: number } = $props()

  let repositoryUrl = $state('')
  let branchFound = $state(false)
  let gitBranch = $state('main')
  let buildPack = $state<BuildPack>('nixpacks')
  let port = $state(3000)
  let publishDirectory = $state('.')
  let errors = $state<Record<string, string>>({})
  let busy = $state(false)

  function loadBranch(e: SubmitEvent) {
    e.preventDefault()
    errors = {}
    try {
      const repository = publicRepository(repositoryUrl)
      repositoryUrl = repository.url
      gitBranch = repository.branch
      branchFound = true
    } catch (err) {
      errors = { git_url: (err as Error).message }
    }
  }

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    const created = await attempt(
      () =>
        api<{ application: Application }>('POST', `/environments/${environmentId}/applications`, {
          git_url: repositoryUrl,
          git_branch: gitBranch,
          build_pack: buildPack,
          port: Number(port),
          publish_directory: publishDirectory,
          server_id: server,
        }),
      ['git_url', 'git_branch', 'build_pack', 'port', 'publish_directory'],
      (e) => (errors = e),
    )
    busy = false
    if (created) go(`/applications/${created.application.id}`)
  }
</script>

{#snippet repositoryHelper()}
  <span class="text-helper">Examples</span><br />For Public repositories, use <span class="text-helper">https://...</span>.<br />For
  Private repositories, use <span class="text-helper">git@...</span>.<br /><br />https://github.com/owner/repository
  <span class="text-helper">main</span> branch will be selected<br />https://github.com/owner/repository/tree/develop
  <span class="text-helper">develop</span> branch will be selected.<br />https://gitea.com/owner/repository.git
  <span class="text-helper">main</span> branch will be selected.
{/snippet}

<div class="chrome mt-8 flex w-full max-w-none flex-col gap-6 lg:mt-3">
  <form onsubmit={loadBranch}>
    <section class="application-settings-section">
      <div class="application-settings-section-header">
        <div>
          <h2>Public Git repository</h2>
          <p>Connect a public repository over HTTPS and inspect its default branch.</p>
        </div>
      </div>
      <div class="application-settings-section-body">
        <div class="flex flex-col gap-2 sm:flex-row sm:items-end">
          <div class="min-w-0 flex-1">
            <!-- svelte-ignore a11y_autofocus -->
            <Input
              required
              label="Repository URL"
              helper={repositoryHelper}
              placeholder="https://github.com/owner/repository"
              autofocus
              bind:value={repositoryUrl}
              error={errors.git_url}
            />
          </div>
          <Button type="submit" class="w-full justify-center sm:w-auto">Check repository</Button>
        </div>
      </div>
    </section>
  </form>

  {#if branchFound}
    <form onsubmit={submit}>
      <section class="application-settings-section">
        <div class="application-settings-section-header">
          <div>
            <h2>Build configuration</h2>
            <p>Choose how The Bakery builds and runs this repository.</p>
          </div>
          <Button type="submit" variant="highlighted" loading={busy}>Continue</Button>
        </div>
        <div class="application-settings-section-body space-y-5">
          <BuildConfiguration
            bind:branch={gitBranch}
            bind:buildPack
            bind:port
            bind:publishDirectory
            {errors}
            branchHelper="You can choose another branch after the application is created."
          />
        </div>
      </section>
    </form>
  {/if}
</div>
