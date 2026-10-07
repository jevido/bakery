<script lang="ts">
  // The Current guild's settings pages, under the settings sidebar (which
  // lists them, as Paperclip's CompanySettingsSidebar does). General and the
  // Danger Zone share the Guild loaded here; the others load their own.
  import { api } from '../../lib/api'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import type { GuildPage } from '../../lib/router.svelte'
  import type { GuildDetails } from '../../lib/types'
  import Spinner from '../../lib/ui/Spinner.svelte'
  import Danger from './Danger.svelte'
  import General from './General.svelte'
  import Members from './Members.svelte'
  import Role from './Role.svelte'
  import Roles from './Roles.svelte'

  let { page, roleId }: { page: GuildPage; roleId?: number } = $props()

  const titles: Record<GuildPage, string> = { '': 'General', members: 'Members', roles: 'Roles', danger: 'Danger Zone' }

  let guild = $state.raw<GuildDetails | null>(null)
  let loadError = $state('')

  async function load() {
    try {
      guild = (await api<{ guild: GuildDetails }>('GET', '/guilds/current')).guild
    } catch (err) {
      loadError = err instanceof Error ? err.message : String(err)
    }
  }
  // Again on every page: the Danger Zone shows what blocks deleting it now.
  $effect(() => {
    void page
    load()
  })

  $effect(() => {
    breadcrumb.set({ label: titles[page] })
  })
</script>

{#if page === 'members'}
  <Members />
{:else if page === 'roles' && roleId !== undefined}
  <Role id={roleId} />
{:else if page === 'roles'}
  <Roles />
{:else if loadError}
  <p class="text-sm text-destructive">{loadError}</p>
{:else if guild === null}
  <Spinner text="Loading…" />
{:else if page === 'danger'}
  <Danger {guild} />
{:else}
  <General {guild} onchange={(g) => (guild = g)} />
{/if}
