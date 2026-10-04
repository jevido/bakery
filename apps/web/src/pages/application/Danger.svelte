<script lang="ts">
  // Coolify's Danger Zone (resources/views/livewire/project/shared/danger.blade.php,
  // components/danger-zone.blade.php and app/Livewire/Project/Shared/Danger.php,
  // Apache-2.0, see NOTICE), shared by the Application and Database pages as
  // Coolify shares it. Each page offers only the checkboxes The Bakery can
  // honour, all ticked to start with, as Coolify's are; each is sent as a
  // query flag on DELETE.
  import { api } from '../../lib/api'
  import { go } from '../../lib/router.svelte'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import SettingsSection from '../../lib/ui/SettingsSection.svelte'
  import { toast } from '../../lib/ui/toast.svelte'

  let {
    label,
    name,
    url,
    checkboxes,
    notes = [],
    back,
  }: {
    // Coolify's $resourceLabel: "application", or "resource" for a Database.
    label: string
    name: string
    // The dashboard API path DELETE goes to.
    url: string
    checkboxes: { id: string; label: string; checked: boolean }[]
    // What else goes with it, under Coolify's three lines.
    notes?: string[]
    // The page to open once it is deleted.
    back: string
  } = $props()

  async function remove(selected: string[]) {
    const q = new URLSearchParams(checkboxes.map((c) => [c.id, String(selected.includes(c.id))]))
    try {
      await api('DELETE', `${url}?${q}`)
    } catch (err) {
      toast.error(`${label[0].toUpperCase()}${label.slice(1)} not deleted`, err instanceof Error ? err.message : String(err))
      throw err
    }
    go(back)
  }
</script>

<SettingsSection id="danger-zone-section" title="Danger zone" helper="Destructive resource actions cannot be undone.">
  <div class="chrome flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
    <div class="min-w-0">
      <h4 class="text-sm font-semibold text-red-700 dark:text-red-300">Delete {label}</h4>
      <div class="mt-2 max-w-2xl space-y-2 text-[13px] leading-5 text-red-700/80 dark:text-red-300/80">
        <p>
          Permanently delete
          <strong class="font-semibold text-black dark:text-fg">{name}</strong>, stop its containers, and remove the selected resources and
          configuration.
        </p>
        <ul class="space-y-1 text-xs">
          <li>• Active deployments will be stopped.</li>
          <li>• Selected volumes and stored data may be permanently removed.</li>
          <li>• This {label} cannot be restored from The Bakery after deletion.</li>
          {#each notes as note (note)}
            <li>• {note}</li>
          {/each}
        </ul>
      </div>
    </div>
    <div class="shrink-0">
      <ConfirmationModal
        title={`Delete ${label}?`}
        buttonTitle={`Delete ${label}`}
        variant="error"
        {checkboxes}
        actions={['Permanently delete this resource and its selected resources.']}
        confirmationText={name}
        confirmationLabel="Enter the resource name to confirm permanent deletion"
        shortConfirmationLabel="Resource name"
        onconfirm={remove}
      />
    </div>
  </div>
</SettingsSection>
