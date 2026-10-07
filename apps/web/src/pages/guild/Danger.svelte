<script lang="ts">
  // Coolify's Team Danger Zone (resources/views/livewire/team/danger-zone.blade.php,
  // app/Livewire/Team/DangerZone.php; Apache-2.0, see NOTICE): Delete guild,
  // disabled with what the Guild still owns. Coolify counts each kind and
  // lists its Projects and Servers; The Bakery names the kinds, with a link
  // to each one's page. The first Guild is not special, and an admin may
  // delete their last Guild: an account may be in none.
  import { api } from '../../lib/api'
  import { go, href } from '../../lib/router.svelte'
  import { session } from '../../lib/session.svelte'
  import type { GuildDetails } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import SettingsSection from '../../lib/ui/SettingsSection.svelte'
  import { toast } from '../../lib/ui/toast.svelte'

  let { guild }: { guild: GuildDetails } = $props()

  const blockers: Record<string, { label: string; path: string }> = {
    projects: { label: 'Projects', path: '/projects' },
    servers: { label: 'Servers', path: '/servers' },
    's3 storages': { label: 'S3 storages', path: '/storages' },
    'notification channels': { label: 'Notification channels', path: '/notifications' },
  }

  async function remove() {
    try {
      await api('DELETE', '/guilds/current')
    } catch (err) {
      toast.error('Guild not deleted', err instanceof Error ? err.message : String(err))
      return
    }
    await session.refresh()
    toast.success('Guild deleted.')
    go('/')
  }
</script>

<div class="application-settings-form w-full">
  <SettingsSection id="guild-danger-zone" title="Danger zone" helper="Destructive actions for this guild cannot be undone.">
    <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
      <div class="min-w-0">
        <h4 class="text-sm font-semibold text-red-700 dark:text-red-300">Delete guild</h4>
        <div class="mt-2 max-w-2xl space-y-2 text-[13px] leading-5 text-red-700/80 dark:text-red-300/80" data-testid="guild-danger">
          {#if !session.can('administrator')}
            <p>Only guild admins can delete this guild.</p>
          {:else if guild.blocking.length === 0}
            <p>
              Permanently delete <strong class="font-semibold text-black dark:text-fg">{guild.name}</strong> from The Bakery. This
              action cannot be undone.
            </p>
            <ul class="space-y-1 text-xs">
              <li>• All members will lose access to this guild.</li>
              <li>• Its open invitations and API tokens stop working.</li>
            </ul>
          {:else}
            <p>This guild still owns:</p>
            <ul class="space-y-1" data-testid="guild-blocking">
              {#each guild.blocking as kind (kind)}
                <li>
                  <a class="font-medium text-coollabs hover:underline" href={href(blockers[kind]?.path ?? '/')}>
                    {blockers[kind]?.label ?? kind}
                  </a>
                </li>
              {/each}
            </ul>
            <p>Remove these resources before deleting the guild.</p>
          {/if}
        </div>
      </div>
      <div class="shrink-0">
        {#if session.can('administrator') && guild.blocking.length === 0}
          <ConfirmationModal
            title="Confirm Guild Deletion?"
            buttonTitle="Delete guild"
            variant="error"
            actions={['The current guild will be permanently deleted from The Bakery.']}
            confirmationText={guild.name}
            confirmationLabel="Enter the guild name to confirm permanent deletion"
            shortConfirmationLabel="Guild name"
            step2ButtonText="Permanently Delete"
            onconfirm={remove}
          />
        {:else}
          <Button variant="error" disabled title="Resolve the requirements shown before deleting this guild.">Delete guild</Button>
        {/if}
      </div>
    </div>
  </SettingsSection>
</div>
