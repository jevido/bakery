<script lang="ts">
  // Coolify's Resource Limits page (resources/views/livewire/project/shared/resource-limits.blade.php,
  // app/Livewire/Project/Shared/ResourceLimits.php; Apache-2.0, see NOTICE),
  // with the two limits The Bakery has: CPU limit and Memory limit, behind
  // the unsaved bar. Memory is typed as Coolify's 512m-style value and stored
  // in whole megabytes; 0 (or empty) is unlimited for both. Left out: CPU
  // set, CPU weight, memory reservation, memory and swap limit, swappiness.
  // Shared by the Application and the Database page, which pass their limits
  // and how to save them.
  import { untrack } from 'svelte'
  import { ApiError } from '../../lib/api'
  import { buttonVariants } from '$lib/components/ui/button'
  import Icon from '../../lib/Icon.svelte'
  import { projectAccess } from '../../lib/projectAccess.svelte'
  import type { ResourceLimits } from '../../lib/types'
  import Input from '../../lib/ui/Input.svelte'
  import SettingsGroup from '../../lib/settings/SettingsGroup.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import UnsavedBar from '../../lib/ui/UnsavedBar.svelte'

  let {
    limits,
    onsave,
    applied,
  }: {
    limits: ResourceLimits
    /** Saves the limits; throws ApiError when the API refuses them. */
    onsave: (limits: ResourceLimits) => Promise<void>
    /** The toast's description after a save: when the limits take effect. */
    applied: string
  } = $props()

  const canUpdate = $derived(projectAccess.can('manage_applications'))
  const saved = $derived(limits)
  const savedCpus = $derived(saved.cpus == null ? '0' : String(saved.cpus))
  const savedMemory = $derived(saved.memory_mb == null ? '0' : `${saved.memory_mb}m`)

  let cpus = $state('')
  let memory = $state('')
  let errors = $state<Record<string, string>>({})
  let saving = $state(false)

  function reset() {
    cpus = savedCpus
    memory = savedMemory
    errors = {}
  }

  // What was saved changed (a save, or another resource): start from it.
  $effect(() => {
    void savedCpus
    void savedMemory
    untrack(reset)
  })

  const dirty = $derived(canUpdate && (cpus.trim() !== savedCpus || memory.trim().toLowerCase() !== savedMemory))

  const units: Record<string, number> = { b: 1 / (1024 * 1024), k: 1 / 1024, m: 1, g: 1024 }

  // Megabytes for a Docker-style size (a number in bytes, or with b, k, m or
  // g), null for unlimited, or an error message.
  function megabytes(value: string): number | null | string {
    const v = value.trim().toLowerCase()
    if (v === '' || v === '0') return null
    const m = /^(\d+(?:\.\d+)?)\s*([bkmg])?$/.exec(v)
    if (!m) return 'Memory limit must be a size such as 256m or 1g.'
    const mb = Number(m[1]) * units[m[2] ?? 'b']
    if (Math.abs(mb - Math.round(mb)) > 1e-9) return 'Memory limit must be a whole number of megabytes, such as 256m or 1g.'
    return Math.round(mb)
  }

  async function save() {
    if (saving || !dirty) return
    errors = {}
    const memoryMB = megabytes(memory)
    if (typeof memoryMB === 'string') {
      errors = { 'resource_limits.memory_mb': memoryMB }
      return
    }
    const c = cpus.trim()
    if (c !== '' && !/^\d+(\.\d+)?$/.test(c)) {
      errors = { 'resource_limits.cpus': 'CPU limit must be a number such as 1.5.' }
      return
    }
    saving = true
    try {
      await onsave({ memory_mb: memoryMB, cpus: c === '' || Number(c) === 0 ? null : Number(c) })
      reset()
      toast.success('Resource limits updated.', applied)
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      errors = Object.keys(err.errors).length ? err.errors : { 'resource_limits.memory_mb': err.message }
    } finally {
      saving = false
    }
  }
</script>

<form
  class="space-y-8"
  onsubmit={(e) => {
    e.preventDefault()
    save()
  }}
>
  {#if canUpdate}
    <UnsavedBar {dirty} {saving} onsave={save} onreset={reset} />
  {/if}

  <SettingsGroup id="cpu-limits-section" label="CPU" hint="Limit the CPU capacity of this container.">
    {#snippet actions()}
      <a class={buttonVariants({ variant: 'outline', size: 'sm' })} target="_blank" rel="noopener noreferrer" href="https://docs.podman.io/en/latest/markdown/podman-run.1.html#cpus">
        Podman CPU constraints
        <Icon name="external-link" class="size-3.5" />
      </a>
    {/snippet}
    <div class="max-w-xs">
      <Input
        label="CPU limit"
        bind:value={cpus}
        error={errors['resource_limits.cpus']}
        placeholder="1.5"
        disabled={!canUpdate}
        helper="Set to 0 to use all available CPUs. Decimal values such as 0.5 are supported."
      />
    </div>
  </SettingsGroup>

  <SettingsGroup id="memory-limits-section" label="Memory" hint="Set a hard memory limit for this container.">
    <div class="max-w-xs">
      <Input
        label="Memory limit"
        bind:value={memory}
        error={errors['resource_limits.memory_mb']}
        placeholder="512m"
        disabled={!canUpdate}
        helper="Maximum memory available to the container. Set to 0 for unlimited."
      />
    </div>
    <p class="text-xs text-muted-foreground">
      Accepted units are <code class="font-mono">b</code>, <code class="font-mono">k</code>,
      <code class="font-mono">m</code>, and <code class="font-mono">g</code>; the limit is kept in whole megabytes, between 16m and 64g.
    </p>
  </SettingsGroup>
</form>
