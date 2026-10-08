<script lang="ts">
  // The guild rail, like Discord's server list: a column of the Guilds the
  // Member is in, left of the sidebar, the Current guild marked by a pill on
  // its left edge, and "+" for a New guild. Paperclip v2026.1005.0 removed its
  // company rail; the goal asks for this one beside the Guild menu.
  import { Plus } from '@lucide/svelte'
  import * as Tooltip from '@bakery/ui/components/ui/tooltip'
  import GuildIcon from '@bakery/ui/GuildIcon.svelte'
  import { switchTo } from './GuildMenu.svelte'
  import { session } from './session.svelte'
  import { sidebar } from './sidebar.svelte'
  import Modal from './ui/Modal.svelte'
  import { cn } from '@bakery/ui/utils'
  import Create from '../pages/guild/Create.svelte'

  let creating = $state(false)

  // Discord's marks: a rounded square instead of a circle on hover and for the
  // current one, and a pill on the left edge, short on hover, tall when current.
  const ITEM = 'group relative flex w-full justify-center outline-none'
  const PILL = 'absolute top-1/2 left-0 w-1 -translate-y-1/2 rounded-r-full bg-foreground transition-all duration-150'
  const ICON = 'size-12 transition-[border-radius] duration-150 group-focus-visible:ring-2 group-focus-visible:ring-ring'
</script>

<nav class="flex h-full w-[72px] shrink-0 flex-col items-center border-r border-border bg-background py-3" aria-label="Guilds" data-testid="guild-rail">
  <div class="flex min-h-0 w-full flex-1 flex-col items-center gap-2 overflow-y-auto [scrollbar-width:none]">
    {#each session.guilds as g (g.id)}
      {@const current = g.id === session.guild?.id}
      <Tooltip.Root>
        <Tooltip.Trigger>
          {#snippet child({ props })}
            <button
              {...props}
              type="button"
              class={ITEM}
              aria-label={g.name}
              aria-current={current ? 'true' : undefined}
              onclick={() => switchTo(g.id)}
              data-testid="guild-rail-item"
            >
              <span class={cn(PILL, current ? 'h-10' : 'h-0 group-hover:h-2')} data-testid="guild-rail-pill"></span>
              <GuildIcon name={g.name} class={cn(ICON, current ? 'rounded-2xl' : 'rounded-3xl group-hover:rounded-2xl')} />
            </button>
          {/snippet}
        </Tooltip.Trigger>
        <Tooltip.Content side="right" sideOffset={8}>{g.name}</Tooltip.Content>
      </Tooltip.Root>
    {/each}
  </div>
  <div class="my-2 h-0.5 w-8 shrink-0 rounded-full bg-border"></div>
  <Tooltip.Root>
    <Tooltip.Trigger>
      {#snippet child({ props })}
        <button
          {...props}
          type="button"
          class={ITEM}
          aria-label="New guild"
          onclick={() => {
            sidebar.closeDrawer()
            creating = true
          }}
          data-testid="guild-rail-new"
        >
          <span class={cn(PILL, 'h-0 group-hover:h-2')}></span>
          <span
            class={cn(
              ICON,
              'flex items-center justify-center rounded-3xl bg-accent text-muted-foreground group-hover:rounded-2xl group-hover:bg-primary group-hover:text-primary-foreground',
            )}
          >
            <Plus class="size-5" />
          </span>
        </button>
      {/snippet}
    </Tooltip.Trigger>
    <Tooltip.Content side="right" sideOffset={8}>New guild</Tooltip.Content>
  </Tooltip.Root>
</nav>

<Modal title="New Guild" variant="none" bind:open={creating}>
  <Create oncreated={() => (creating = false)} />
</Modal>
