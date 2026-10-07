<script lang="ts">
  // Coolify's Team Danger Zone (resources/views/livewire/team/danger-zone.blade.php,
  // app/Livewire/Team/DangerZone.php; Apache-2.0, see NOTICE) in Paperclip's
  // Danger Zone group (ui/src/pages/CompanySettings.tsx; MIT, see NOTICE):
  // transferring the Guild Master, and Delete guild, disabled with what the
  // Guild still owns. Coolify counts each kind and lists its Projects and
  // Servers; The Bakery names the kinds, with a link to each one's page. The
  // first Guild is not special, and an admin may delete their last Guild: an
  // account may be in none.
  import { ShieldAlert } from '@lucide/svelte'
  import { untrack } from 'svelte'
  import { api } from '../../lib/api'
  import { go, href } from '../../lib/router.svelte'
  import { session } from '../../lib/session.svelte'
  import SettingsGroup from '../../lib/settings/SettingsGroup.svelte'
  import SettingsPage from '../../lib/settings/SettingsPage.svelte'
  import type { GuildDetails, Member } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import Select from '../../lib/ui/Select.svelte'
  import { toast } from '../../lib/ui/toast.svelte'

  let { guild }: { guild: GuildDetails } = $props()

  const blockers: Record<string, { label: string; path: string }> = {
    projects: { label: 'Projects', path: '/projects' },
    servers: { label: 'Servers', path: '/servers' },
    's3 storages': { label: 'S3 storages', path: '/storages' },
    'notification channels': { label: 'Notification channels', path: '/notifications' },
  }

  // The Guild's open Transfer offer, here so withdrawing it needs no reload.
  let offer = $state.raw(untrack(() => guild.offer))
  // The other Members the Guild Master can offer it to; null while loading.
  let candidates = $state.raw<Member[] | null>(null)
  let to = $state<number | null>(null)
  let offerError = $state('')
  const when = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' })

  $effect(() => {
    if (!session.guildMaster) return
    api<{ members: Member[] }>('GET', '/members').then((r) => {
      candidates = r.members.filter((m) => !m.guild_master)
      to ??= candidates[0]?.id ?? null
    })
  })

  const target = $derived(candidates?.find((m) => m.id === to) ?? null)

  async function refreshOffer() {
    offer = (await api<{ guild: GuildDetails }>('GET', '/guilds/current')).guild.offer
  }

  async function offerGuildMaster() {
    if (!target) return
    offerError = ''
    try {
      await api('POST', '/guilds/current/guild-master-offer', { member_id: target.id })
    } catch (err) {
      offerError = err instanceof Error ? err.message : String(err)
    }
    await refreshOffer()
  }

  async function withdrawOffer() {
    offerError = ''
    try {
      await api('DELETE', '/guilds/current/guild-master-offer')
    } catch (err) {
      offerError = err instanceof Error ? err.message : String(err)
    }
    await refreshOffer()
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

<SettingsPage icon={ShieldAlert} title="Danger Zone">
  <SettingsGroup label="Guild Master" hint="The one person who holds every permission in this guild and cannot be locked out." data-testid="guild-master-transfer">
    {#if offer}
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between" data-testid="guild-master-offer">
        <p class="text-sm text-muted-foreground">
          The Guild Master is offered to <strong class="font-medium text-foreground">{offer.to?.name}</strong> ({offer.to?.email}). Nothing
          changes until they accept; the offer expires on {when.format(new Date(offer.expires_at))}.
        </p>
        {#if session.guildMaster}<Button onclick={withdrawOffer}>Withdraw</Button>{/if}
      </div>
    {:else if !session.guildMaster}
      <p class="text-sm text-muted-foreground">
        {guild.guild_master ? `${guild.guild_master.name} is the Guild Master.` : ''} Only the Guild Master can transfer it.
      </p>
    {:else if candidates === null}
      <p class="text-sm text-muted-foreground">Loading members…</p>
    {:else if candidates.length === 0}
      <p class="text-sm text-muted-foreground">Invite someone first: the Guild Master can only be offered to another member of this guild.</p>
    {:else}
      <div class="flex flex-col gap-3 sm:flex-row sm:items-end">
        <div class="min-w-0 flex-1">
          <Select label="Offer it to" bind:value={to} data-testid="guild-master-candidate">
            {#each candidates as m (m.id)}
              <option value={m.id}>{m.name} ({m.email})</option>
            {/each}
          </Select>
        </div>
        <ConfirmationModal
          title="Transfer Guild Master?"
          buttonTitle="Transfer Guild Master"
          actions={[
            `${target?.name} is offered the Guild Master of ${guild.name}.`,
            'Nothing changes until they accept. Until then you can withdraw the offer, and it expires after 7 days.',
            'Once they accept, they are the Guild Master and you are not. You both keep your other Roles.',
          ]}
          warningMessage="Only the Guild Master can transfer it, so you cannot take it back yourself once it is accepted."
          confirmWithText={false}
          step2ButtonText="Offer"
          onconfirm={offerGuildMaster}
        />
      </div>
    {/if}
    {#if offerError}<p class="text-sm text-destructive">{offerError}</p>{/if}
  </SettingsGroup>

  <SettingsGroup label="Danger Zone" destructive data-testid="guild-delete">
    <div class="space-y-2 text-sm text-muted-foreground" data-testid="guild-danger">
      {#if !session.can('administrator')}
        <p>Only guild admins can delete this guild.</p>
      {:else if guild.blocking.length === 0}
        <p>
          Permanently delete <strong class="font-medium text-foreground">{guild.name}</strong> from The Bakery. This action cannot be undone.
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
              <a class="font-medium text-foreground underline underline-offset-4" href={href(blockers[kind]?.path ?? '/')}>
                {blockers[kind]?.label ?? kind}
              </a>
            </li>
          {/each}
        </ul>
        <p>Remove these resources before deleting the guild.</p>
      {/if}
    </div>
    <div class="flex items-center gap-2">
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
  </SettingsGroup>
</SettingsPage>
