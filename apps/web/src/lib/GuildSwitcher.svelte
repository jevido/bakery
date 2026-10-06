<script lang="ts">
  // Coolify's team switcher (resources/views/livewire/switch-team.blade.php,
  // app/Livewire/SwitchTeam.php; Apache-2.0, see NOTICE) as the Guild
  // switcher: the Current guild's name with up/down chevrons, a menu of the
  // Guilds with the current one marked, and "New guild". Switching shows
  // the Dashboard of the other Guild.
  import Icon from './Icon.svelte'
  import { go } from './router.svelte'
  import { session } from './session.svelte'
  import Modal from './ui/Modal.svelte'
  import { toast } from './ui/toast.svelte'
  import Create from '../pages/guild/Create.svelte'

  let open = $state(false)
  let creating = $state(false)
  let root: HTMLDivElement

  async function switchTo(id: number) {
    open = false
    if (id === session.guild?.id) return
    try {
      await session.switchGuild(id)
    } catch (err) {
      toast.error('Guild not switched', err instanceof Error ? err.message : String(err))
      return
    }
    go('/')
  }
</script>

<svelte:window
  onkeydown={(e) => open && e.key === 'Escape' && (open = false)}
  onclick={(e) => open && !root.contains(e.target as Node) && (open = false)}
/>

<div class="relative min-w-0" bind:this={root}>
  <button
    type="button"
    onclick={() => (open = !open)}
    title="Switch guild"
    aria-expanded={open}
    data-testid="guild-switcher"
    class="-ml-1 flex h-8 min-w-0 items-center gap-1.5 rounded-lg px-2 text-left opacity-70 transition-[background-color,opacity] hover:bg-neutral-100 hover:opacity-100 dark:hover:bg-white/[0.05]"
  >
    <span class="truncate text-[13px] font-semibold whitespace-nowrap text-black dark:text-fg">{session.guild?.name ?? 'No guild'}</span>
    <svg class="size-4 shrink-0 text-neutral-400 dark:text-fg-faint" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path d="M8 9l4-4 4 4M8 15l4 4 4-4" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" />
    </svg>
  </button>
  {#if open}
    <div class="listbox-panel left-0! z-[90]! max-h-72! min-w-56" role="menu">
      <div class="px-2 py-1 text-[10px] font-semibold tracking-wide text-neutral-400 uppercase dark:text-fg-faint">Guilds</div>
      {#each session.guilds as g (g.id)}
        <button
          type="button"
          role="menuitem"
          onclick={() => switchTo(g.id)}
          class={['listbox-option w-full', g.id === session.guild?.id && 'bg-neutral-100 font-medium dark:bg-white/[0.06]']}
          aria-current={g.id === session.guild?.id ? 'true' : undefined}
          data-testid="guild-option"
        >
          <span class="min-w-0 flex-1 truncate text-left">{g.name}</span>
          {#if g.id === session.guild?.id}<Icon name="check" class="size-3.5 shrink-0" />{/if}
        </button>
      {/each}
      <div class="mt-1 border-t border-neutral-200 pt-1 dark:border-white/[0.08]">
        <button
          type="button"
          role="menuitem"
          class="listbox-option w-full"
          onclick={() => {
            open = false
            creating = true
          }}
        >
          <Icon name="plus" class="size-3.5 shrink-0" />
          <span class="min-w-0 flex-1 text-left">New guild</span>
        </button>
      </div>
    </div>
  {/if}
</div>

<Modal title="New Guild" variant="none" bind:open={creating}>
  <Create oncreated={() => (creating = false)} />
</Modal>
