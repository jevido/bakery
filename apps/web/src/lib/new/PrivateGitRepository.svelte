<script lang="ts">
  // Coolify's github-private-repository-deploy-key page (resources/views/
  // livewire/project/new/github-private-repository-deploy-key.blade.php,
  // app/Livewire/Project/New/GithubPrivateRepositoryDeployKey.php; Apache-2.0,
  // see NOTICE). Coolify first asks for one of the team's Private Keys; The
  // Bakery generates a Deploy key per Application instead, so the page opens
  // on the repository step and the Application page shows the key to add.
  import { api } from '../api'
  import { applicationPath, go } from '../router.svelte'
  import type { Application, BuildPack } from '../types'
  import SettingsGroup from '../settings/SettingsGroup.svelte'
  import Button from '../ui/Button.svelte'
  import Input from '../ui/Input.svelte'
  import { toast } from '../ui/toast.svelte'
  import BuildConfiguration from './BuildConfiguration.svelte'
  import { attempt } from './create'

  let { environmentId, server }: { environmentId: number; server: number } = $props()

  let repositoryUrl = $state('')
  let branch = $state('main')
  let buildPack = $state<BuildPack>('nixpacks')
  let port = $state(3000)
  let publishDirectory = $state('.')
  let errors = $state<Record<string, string>>({})
  let busy = $state(false)

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    const created = await attempt(
      () =>
        api<{ application: Application }>('POST', `/environments/${environmentId}/applications`, {
          git_url: repositoryUrl.trim(),
          git_branch: branch,
          build_pack: buildPack,
          port: Number(port),
          publish_directory: publishDirectory,
          server_id: server,
        }),
      ['git_url', 'git_branch', 'build_pack', 'port', 'publish_directory'],
      (e) => (errors = e),
    )
    busy = false
    if (!created) return
    if (created.application.deploy_key_public) toast.info('Add the deploy key to the repository, then deploy.')
    go(applicationPath(created.application))
  }
</script>

<form onsubmit={submit}>
  <SettingsGroup label="Repository configuration" hint="Enter the repository location and choose how The Bakery should build it.">
    <!-- svelte-ignore a11y_autofocus -->
    <Input required label="Repository URL" placeholder="git@github.com:owner/repository.git" autofocus bind:value={repositoryUrl} error={errors.git_url} />
    <BuildConfiguration bind:branch bind:buildPack bind:port bind:publishDirectory {errors} />
    <div class="flex justify-end">
      <Button type="submit" variant="highlighted" loading={busy}>Continue</Button>
    </div>
  </SettingsGroup>
</form>
