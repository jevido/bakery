<script lang="ts">
  // Coolify's Team General page (resources/views/livewire/team/index.blade.php,
  // app/Livewire/Team/Index.php; Apache-2.0, see NOTICE): Name, Description
  // and "New guild". Coolify's MCP server setting is left out until agents
  // reach The Bakery's API.
  import { untrack } from 'svelte'
  import { api, ApiError } from '../../lib/api'
  import { session } from '../../lib/session.svelte'
  import type { GuildDetails } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import Input from '../../lib/ui/Input.svelte'
  import Modal from '../../lib/ui/Modal.svelte'
  import SettingsSection from '../../lib/ui/SettingsSection.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import UnsavedBar from '../../lib/ui/UnsavedBar.svelte'
  import Create from './Create.svelte'

  let { guild, onchange }: { guild: GuildDetails; onchange: (g: GuildDetails) => void } = $props()

  const canUpdate = $derived(session.can('manage_guild'))

  let name = $state(untrack(() => guild.name))
  let description = $state(untrack(() => guild.description))
  let errors = $state<Record<string, string>>({})
  let saving = $state(false)
  let creating = $state(false)

  function reset() {
    name = guild.name
    description = guild.description
    errors = {}
  }

  const dirty = $derived(canUpdate && (name !== guild.name || description !== guild.description))

  async function save() {
    if (saving || !dirty) return
    saving = true
    errors = {}
    try {
      const r = await api<{ guild: GuildDetails }>('PATCH', '/guilds/current', { name, description })
      onchange({ ...r.guild, blocking: guild.blocking })
      untrack(reset)
      // The switcher shows the new name.
      await session.refresh()
      toast.success('Guild updated.')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      errors = Object.keys(err.errors).length ? err.errors : { name: err.message }
    } finally {
      saving = false
    }
  }
</script>

<form
  class="application-settings-form flex flex-col gap-6"
  onsubmit={(e) => {
    e.preventDefault()
    save()
  }}
>
  {#if canUpdate}
    <UnsavedBar {dirty} {saving} onsave={save} onreset={reset} />
  {/if}
  <SettingsSection id="guild-general-section" title="General" helper="Manage this guild's identity.">
    {#snippet actions()}
      <Modal title="New Guild" bind:open={creating}>
        {#snippet trigger(show)}
          <Button onclick={show}>New guild</Button>
        {/snippet}
        <Create oncreated={() => (creating = false)} />
      </Modal>
    {/snippet}
    <div class="grid gap-4 lg:grid-cols-2">
      <Input label="Name" bind:value={name} error={errors.name} required disabled={!canUpdate} />
      <Input label="Description" bind:value={description} error={errors.description} disabled={!canUpdate} />
    </div>
  </SettingsSection>
</form>
