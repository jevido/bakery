<script lang="ts">
  // Coolify's Team General page (resources/views/livewire/team/index.blade.php,
  // app/Livewire/Team/Index.php; Apache-2.0, see NOTICE): Name, Description
  // and "New guild", in Paperclip's CompanySettings General group
  // (ui/src/pages/CompanySettings.tsx; MIT, see NOTICE). Coolify's MCP server
  // setting is left out until agents reach The Bakery's API.
  import { SlidersHorizontal } from '@lucide/svelte'
  import { untrack } from 'svelte'
  import { api, ApiError } from '../../lib/api'
  import { session } from '../../lib/session.svelte'
  import type { GuildDetails } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import Input from '../../lib/ui/Input.svelte'
  import Modal from '../../lib/ui/Modal.svelte'
  import SettingsGroup from '../../lib/settings/SettingsGroup.svelte'
  import SettingsPage from '../../lib/settings/SettingsPage.svelte'
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

<SettingsPage icon={SlidersHorizontal} title="General">
  {#snippet actions()}
    <Modal title="New Guild" bind:open={creating}>
      {#snippet trigger(show)}
        <Button onclick={show}>New guild</Button>
      {/snippet}
      <Create oncreated={() => (creating = false)} />
    </Modal>
  {/snippet}
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
    <SettingsGroup label="General" data-testid="guild-general">
      <Input label="Guild name" helper="The display name of this guild." bind:value={name} error={errors.name} required disabled={!canUpdate} />
      <Input
        label="Description"
        helper="Optional description shown with this guild."
        placeholder="Optional guild description"
        bind:value={description}
        error={errors.description}
        disabled={!canUpdate}
      />
    </SettingsGroup>
  </form>
</SettingsPage>
