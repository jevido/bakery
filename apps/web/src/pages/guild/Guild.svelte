<script lang="ts">
  // Coolify's Team layout (resources/views/components/team/settings-layout.blade.php,
  // Apache-2.0, see NOTICE) as the Guild's: the header below xl, the Guild
  // sidebar (General, Members, Danger Zone) and the page. Coolify's Admin View
  // is left out; the Instance admin's list of every account comes with
  // instance Settings.
  import { api } from '../../lib/api'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import Icon, { type IconName } from '../../lib/Icon.svelte'
  import { guildPath, href, type GuildPage } from '../../lib/router.svelte'
  import type { GuildDetails } from '../../lib/types'
  import Spinner from '../../lib/ui/Spinner.svelte'
  import Danger from './Danger.svelte'
  import General from './General.svelte'
  import Members from './Members.svelte'
  import Role from './Role.svelte'
  import Roles from './Roles.svelte'

  let { page, roleId }: { page: GuildPage; roleId?: number } = $props()

  const items: { page: GuildPage; label: string; icon: IconName; sectionStart?: boolean }[] = [
    { page: '', label: 'General', icon: 'settings' },
    { page: 'members', label: 'Members', icon: 'teams' },
    { page: 'roles', label: 'Roles', icon: 'lock' },
    { page: 'danger', label: 'Danger Zone', icon: 'shield-alert', sectionStart: true },
  ]

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
    void page
    breadcrumb.set({ label: 'Guild' })
  })
</script>

<section class="chrome application-settings-workspace w-full max-w-none">
  <header class="settings-mobile-header xl:hidden">
    <h1 class="settings-mobile-title">Guild</h1>
    <p class="settings-mobile-description">Manage your guild, members, and access settings.</p>
  </header>
  <div class="grid min-w-0 gap-8 xl:grid-cols-[210px_minmax(0,1fr)] xl:gap-8">
    <aside class="application-settings-navigation min-w-0 xl:self-start">
      <nav
        aria-label="Guild settings"
        class="grid grid-cols-2 gap-0.5 border-y border-neutral-200 py-3 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-1 xl:border-y-0 xl:py-0 dark:border-white/[0.06]"
      >
        <div class="nav-section hidden xl:block">Guild</div>
        {#each items as item (item.page)}
          {#if item.sectionStart}
            <div class="col-span-full my-2 hidden border-t border-neutral-200 xl:block dark:border-white/[0.06]" aria-hidden="true"></div>
          {/if}
          <a
            class={['menu-item', item.page === page && 'menu-item-active']}
            href={href(guildPath(item.page))}
            aria-current={item.page === page ? 'page' : undefined}
            data-testid="guild-menu-item"
          >
            <Icon name={item.icon} class="menu-item-icon" />
            <span class="menu-item-label">{item.label}</span>
          </a>
        {/each}
      </nav>
    </aside>

    <div class="min-w-0">
      {#if page === 'members'}
        <Members />
      {:else if page === 'roles' && roleId !== undefined}
        <Role id={roleId} />
      {:else if page === 'roles'}
        <Roles />
      {:else if loadError}
        <p class="text-sm text-red-600 dark:text-red-400">{loadError}</p>
      {:else if guild === null}
        <Spinner text="Loading…" />
      {:else if page === 'danger'}
        <Danger {guild} />
      {:else}
        <General {guild} onchange={(g) => (guild = g)} />
      {/if}
    </div>
  </div>
</section>
