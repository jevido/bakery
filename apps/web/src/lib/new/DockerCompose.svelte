<script lang="ts">
  // Coolify's docker-compose page (resources/views/livewire/project/new/
  // docker-compose.blade.php, app/Livewire/Project/New/DockerCompose.php;
  // Apache-2.0, see NOTICE): a Service from a pasted compose file, with a
  // generated name. A plain textarea stands in for Coolify's Monaco editor;
  // the API's refusal lists each problem with its line. The placeholder gives
  // the Component a Domain instead of Coolify's `ports:`, which The Bakery
  // refuses.
  import { api } from '../api'
  import { go, servicePath } from '../router.svelte'
  import type { Service } from '../types'
  import SettingsGroup from '../settings/SettingsGroup.svelte'
  import Button from '../ui/Button.svelte'
  import Textarea from '../ui/Textarea.svelte'
  import { toast } from '../ui/toast.svelte'
  import { attempt } from './create'

  let { environmentId }: { environmentId: number } = $props()

  let dockerComposeRaw = $state('')
  let errors = $state<Record<string, string>>({})
  let busy = $state(false)

  const problems = $derived([errors.compose, errors.domains].filter(Boolean).flatMap((e) => e.split('\n')))

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    const created = await attempt(
      () => api<{ service: Service }>('POST', `/environments/${environmentId}/services`, { compose: dockerComposeRaw }),
      ['compose', 'domains'],
      (e) => (errors = e),
    )
    busy = false
    if (!created) return
    toast.success('Service created.')
    go(servicePath(created.service))
  }
</script>

<form onsubmit={submit}>
  <SettingsGroup label="Docker Compose" hint="Create a multi-container service directly from a Compose file." wide>
    <!-- svelte-ignore a11y_autofocus -->
    <Textarea
      label="Docker Compose file"
      rows={20}
      monospace
      allowTab
      required
      autofocus
      bind:value={dockerComposeRaw}
      aria-invalid={problems.length > 0 ? 'true' : undefined}
      placeholder={'services:\n  app:\n    image: nginx:alpine\n    environment:\n      - SERVICE_FQDN_APP_80\n'}
    />
    {#if problems.length > 0}
      <ul class="mt-1 list-none space-y-0.5 p-0 font-mono text-xs text-destructive">
        {#each problems as problem, i (i)}<li>{problem}</li>{/each}
      </ul>
    {/if}
    <div class="flex justify-end">
      <Button type="submit" variant="highlighted" loading={busy}>Create service</Button>
    </div>
  </SettingsGroup>
</form>
