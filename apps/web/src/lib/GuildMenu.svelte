<script lang="ts" module>
  import { go } from './router.svelte'
  import { session } from './session.svelte'
  import { sidebar } from './sidebar.svelte'
  import { toast } from './ui/toast.svelte'

  /** Makes the Guild the Current guild and opens its Dashboard; the guild rail switches the same way. */
  export async function switchTo(id: number) {
    sidebar.closeDrawer()
    if (id === session.guild?.id) return
    // Leave the page first: the switch remounts the current page in the new
    // Guild, where a Resource of the old one answers 404.
    go('/')
    try {
      await session.switchGuild(id)
    } catch (err) {
      toast.error('Guild not switched', err instanceof Error ? err.message : String(err))
    }
  }
</script>

<script lang="ts">
  // Paperclip's company menu (ui/src/components/SidebarCompanyMenu.tsx; MIT,
  // see NOTICE) as the Guild menu at the top of the sidebar: the Current
  // guild's pattern icon and name, opening the Guilds to switch to (the
  // current one checked), New guild, Invite and Log out. Left out: dragging
  // the Guilds into an order (nothing stores one). Switching opens the other
  // Guild's Dashboard.
  import { Check, ChevronsUpDown, LogOut, Plus, UserPlus } from '@lucide/svelte'
  import { buttonVariants } from '@bakery/ui/components/ui/button'
  import * as DropdownMenu from '@bakery/ui/components/ui/dropdown-menu'
  import GuildIcon from '@bakery/ui/GuildIcon.svelte'
  import { guildPath, href } from './router.svelte'
  import { RAIL_HIDDEN_LABEL } from './sidebar.svelte'
  import Modal from './ui/Modal.svelte'
  import { cn } from '@bakery/ui/utils'
  import Create from '../pages/guild/Create.svelte'

  let open = $state(false)
  let creating = $state(false)

  const ROW =
    'h-(--organization-popover-company-row-height) min-w-0 gap-(--organization-popover-row-gap) rounded-lg px-2.5 py-0 text-(length:--text-compact) focus:bg-accent/50 focus:text-foreground'
  const ACTION =
    'h-(--organization-popover-action-row-height) gap-(--organization-popover-row-gap) rounded-lg px-2.5 py-0 text-(length:--text-compact) font-medium leading-(--organization-popover-action-line-height) text-foreground focus:bg-accent/50 focus:text-foreground'

  const name = $derived(session.guild?.name ?? null)
</script>

<DropdownMenu.Root bind:open>
  <DropdownMenu.Trigger
    class={cn(
      buttonVariants({ variant: 'ghost' }),
      // The nav icons sit at nav px-3 + item mx-2 + item px-2; the wrapper's
      // px-3 and this px-4 put the guild icon on the same line.
      'h-9 min-w-0 flex-1 justify-start gap-2 px-4 text-left hover:bg-sidebar-accent hover:text-sidebar-accent-foreground has-[>svg]:px-4 dark:hover:bg-sidebar-accent dark:hover:text-sidebar-accent-foreground',
      sidebar.rail && 'justify-center gap-0 px-0 has-[>svg]:px-0',
    )}
    aria-label={name ? `Open ${name} guild switcher` : 'Open guild switcher'}
    data-testid="guild-menu"
  >
    <span class={cn('flex min-w-0 flex-1 items-center gap-2', sidebar.rail && 'justify-center gap-0')}>
      {#if session.guild}
        <GuildIcon name={session.guild.name} class="size-5 shrink-0 rounded-md text-(length:--text-micro)" />
      {/if}
      <span class={cn('min-w-0 truncate text-sm font-bold text-foreground', sidebar.rail && RAIL_HIDDEN_LABEL)} title={name ?? undefined}>
        {name ?? 'Select guild'}
      </span>
    </span>
    {#if !sidebar.rail}<ChevronsUpDown class="size-3.5 shrink-0 text-muted-foreground" />{/if}
  </DropdownMenu.Trigger>
  <DropdownMenu.Content
    align="start"
    sideOffset={8}
    class="ml-2 w-(--organization-popover-width) max-w-(--sz-calc-24) overflow-hidden rounded-xl border-border bg-popover p-0 shadow-(--shadow-profile-popover)"
  >
    <div class="flex h-(--organization-popover-header-height) items-center px-3.5">
      <DropdownMenu.Label class="p-0 text-(length:--text-compact) font-semibold text-foreground">Guilds</DropdownMenu.Label>
    </div>
    <div class="flex max-h-96 flex-col gap-0.5 overflow-y-auto px-2.5 pt-1 pb-2">
      {#each session.guilds as g (g.id)}
        <DropdownMenu.Item class={ROW} onSelect={() => switchTo(g.id)} data-testid="guild-option" aria-current={g.id === session.guild?.id ? 'true' : undefined}>
          <GuildIcon name={g.name} class="size-(--organization-popover-avatar-size) shrink-0 rounded-lg text-(length:--text-micro)" />
          <span class="block min-w-0 flex-1 truncate leading-(--organization-popover-name-line-height) font-medium">{g.name}</span>
          <span class="flex size-5 shrink-0 items-center justify-center">
            {#if g.id === session.guild?.id}<Check class="size-4 text-foreground" />{/if}
          </span>
        </DropdownMenu.Item>
      {:else}
        <DropdownMenu.Item disabled>No guilds</DropdownMenu.Item>
      {/each}
    </div>
    <div class="flex flex-col gap-0.5 border-t border-border px-2.5 pt-2 pb-2.5">
      <DropdownMenu.Item
        class={ACTION}
        onSelect={() => {
          sidebar.closeDrawer()
          creating = true
        }}
      >
        <span class="flex size-5 shrink-0 items-center justify-center text-muted-foreground"><Plus class="size-4" /></span>
        <span class="min-w-0 flex-1 truncate">New guild</span>
      </DropdownMenu.Item>
      {#if session.can('manage_members')}
        <DropdownMenu.Item class={ACTION} onSelect={() => sidebar.closeDrawer()}>
          {#snippet child({ props })}
            <a {...props} href={href(guildPath('members'))}>
              <span class="flex size-5 shrink-0 items-center justify-center text-muted-foreground"><UserPlus class="size-4" /></span>
              <span class="min-w-0 flex-1 truncate">{name ? `Invite people to ${name}` : 'Invite people'}</span>
            </a>
          {/snippet}
        </DropdownMenu.Item>
      {/if}
      <DropdownMenu.Item class={ACTION} onSelect={() => session.logout()}>
        <span class="flex size-5 shrink-0 items-center justify-center text-muted-foreground"><LogOut class="size-4" /></span>
        <span class="min-w-0 flex-1 truncate">Log out</span>
      </DropdownMenu.Item>
    </div>
  </DropdownMenu.Content>
</DropdownMenu.Root>

<Modal title="New Guild" variant="none" bind:open={creating}>
  <Create oncreated={() => (creating = false)} />
</Modal>
