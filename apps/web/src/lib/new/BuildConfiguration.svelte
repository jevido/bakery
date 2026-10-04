<script lang="ts">
  // The build fields the public-git-repository and
  // github-private-repository-deploy-key pages share (Apache-2.0, see NOTICE),
  // with what The Bakery builds: Branch, Build pack, Port, and Publish
  // directory for Static. updatedBuildPack() sets the port as Coolify does:
  // 3000 for Nixpacks, 80 for Static.
  //
  // Left out: Railpack and Docker Compose build packs, the Output type of
  // Nixpacks and the Base directory, which The Bakery does not have.
  import type { BuildPack } from '../types'
  import Input from '../ui/Input.svelte'
  import Select from '../ui/Select.svelte'

  let {
    branch = $bindable(),
    buildPack = $bindable(),
    port = $bindable(),
    publishDirectory = $bindable(),
    errors,
    branchHelper,
  }: {
    branch: string
    buildPack: BuildPack
    port: number
    publishDirectory: string
    errors: Record<string, string>
    branchHelper?: string
  } = $props()

  const packs: { value: BuildPack; label: string }[] = [
    { value: 'nixpacks', label: 'Nixpacks' },
    { value: 'static', label: 'Static' },
    { value: 'dockerfile', label: 'Dockerfile' },
  ]

  function updatedBuildPack() {
    if (buildPack === 'nixpacks') port = 3000
    else if (buildPack === 'static') port = 80
  }
</script>

<div class="grid gap-4 sm:grid-cols-2">
  <Input label="Branch" bind:value={branch} required helper={branchHelper} error={errors.git_branch} />
  <Select label="Build pack" bind:value={buildPack} required onchange={updatedBuildPack} error={errors.build_pack}>
    {#each packs as p (p.value)}
      <option value={p.value}>{p.label}</option>
    {/each}
  </Select>
  {#if buildPack === 'static'}
    <Input
      label="Publish directory"
      bind:value={publishDirectory}
      required
      helper="Directory containing the generated static assets."
      error={errors.publish_directory}
    />
  {:else}
    <Input type="number" label="Port" bind:value={port} required helper="Port the application listens on." error={errors.port} />
  {/if}
</div>
