<script lang="ts">
  // Coolify's Server Danger page (resources/views/livewire/server/delete.blade.php,
  // components/danger-zone.blade.php and app/Livewire/Server/Delete.php;
  // Apache-2.0, see NOTICE). The Local server is Coolify's is_coolify_host
  // and shows nothing. Coolify's force-deletion checkbox is left out: the API
  // refuses to delete a Server Applications still run on, and that refusal
  // is shown as the error toast.
  import { untrack } from 'svelte'
  import { api } from '../../lib/api'
  import { go } from '../../lib/router.svelte'
  import { session } from '../../lib/session.svelte'
  import type { Server } from '../../lib/types'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import SettingsSection from '../../lib/ui/SettingsSection.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import { serverResources } from './resources'

  let { server }: { server: Server } = $props()

  // Coolify's definedResources()->count() > 0; unknown until loaded.
  let hasResources = $state(false)
  const serverId = $derived(server.id)
  $effect(() => {
    void serverId
    hasResources = false
    // Not again on every reload of the Server, only when it is another one.
    serverResources(untrack(() => server))
      .then((rows) => (hasResources = rows.length > 0))
      .catch(() => {})
  })

  async function remove() {
    try {
      await api('DELETE', `/servers/${server.id}`)
    } catch (err) {
      toast.error('Server not deleted', err instanceof Error ? err.message : String(err))
      return
    }
    toast.success('Server deleted.')
    go('/servers')
  }
</script>

<div class="chrome application-settings-form w-full">
  {#if server.kind === 'remote'}
    <SettingsSection
      id="server-danger-section"
      title="Delete server"
      helper="Permanently remove this server and its configuration from The Bakery."
    >
      <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div class="min-w-0">
          <h4 class="text-sm font-semibold text-red-700 dark:text-red-300">This action cannot be undone</h4>
          <div class="mt-2 max-w-2xl space-y-2 text-[13px] leading-5 text-red-700/80 dark:text-red-300/80">
            <p>
              The server will be removed from The Bakery. Nothing on the server itself is touched.
              {#if hasResources}
                <span data-testid="server-has-resources">
                  It currently contains managed resources. Delete them first; The Bakery does not delete a server that still runs
                  applications.
                </span>
              {/if}
            </p>
            <p>Type the server name in the confirmation dialog to continue.</p>
          </div>
        </div>
        {#if session.can('manage_servers')}
          <div class="shrink-0">
            <ConfirmationModal
              title="Confirm Server Deletion?"
              buttonTitle="Delete server"
              variant="error"
              actions={['This server will be permanently deleted from The Bakery.']}
              confirmationText={server.name}
              confirmationLabel="Please confirm by entering the Server Name below"
              shortConfirmationLabel="Server Name"
              onconfirm={remove}
            />
          </div>
        {/if}
      </div>
    </SettingsSection>
  {/if}
</div>
