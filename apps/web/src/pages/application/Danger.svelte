<script lang="ts">
  // Coolify's Danger Zone (resources/views/livewire/project/shared/danger.blade.php,
  // components/danger-zone.blade.php and app/Livewire/Project/Shared/Danger.php,
  // Apache-2.0, see NOTICE). Of Coolify's checkboxes only the two The Bakery
  // can honour are offered: volumes, and the Application's Images in place of
  // the Docker cleanup. It has no per-Application networks or configuration
  // files on the server to delete. Both start ticked, as Coolify's do.
  import { api } from '../../lib/api'
  import { go } from '../../lib/router.svelte'
  import type { Application } from '../../lib/types'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import SettingsSection from '../../lib/ui/SettingsSection.svelte'
  import { toast } from '../../lib/ui/toast.svelte'

  let { application }: { application: Application } = $props()

  const checkboxes = [
    { id: 'delete_volumes', label: 'Permanently delete all volumes associated with this resource.', checked: true },
    { id: 'delete_images', label: 'Remove the images its deployments built or pulled.', checked: true },
  ]

  async function remove(selected: string[]) {
    const q = new URLSearchParams(checkboxes.map((c) => [c.id, String(selected.includes(c.id))]))
    try {
      await api('DELETE', `/applications/${application.id}?${q}`)
    } catch (err) {
      toast.error('Application not deleted', err instanceof Error ? err.message : String(err))
      throw err
    }
    go(`/project/${application.project_id}/environment/${application.environment_id}`)
  }
</script>

<SettingsSection id="danger-zone-section" title="Danger zone" helper="Destructive resource actions cannot be undone.">
  <div class="chrome flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
    <div class="min-w-0">
      <h4 class="text-sm font-semibold text-red-700 dark:text-red-300">Delete application</h4>
      <div class="mt-2 max-w-2xl space-y-2 text-[13px] leading-5 text-red-700/80 dark:text-red-300/80">
        <p>
          Permanently delete
          <strong class="font-semibold text-black dark:text-fg">{application.name}</strong>, stop its containers, and remove the selected resources and
          configuration.
        </p>
        <ul class="space-y-1 text-xs">
          <li>• Active deployments will be stopped.</li>
          <li>• Selected volumes and stored data may be permanently removed.</li>
          <li>• This application cannot be restored from The Bakery after deletion.</li>
        </ul>
      </div>
    </div>
    <div class="shrink-0">
      <ConfirmationModal
        title="Delete application?"
        buttonTitle="Delete application"
        variant="error"
        {checkboxes}
        actions={['Permanently delete this resource and its selected resources.']}
        confirmationText={application.name}
        confirmationLabel="Enter the resource name to confirm permanent deletion"
        shortConfirmationLabel="Resource name"
        onconfirm={remove}
      />
    </div>
  </div>
</SettingsSection>
