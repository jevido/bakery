<script lang="ts">
  // The guild rail, as apps/web/src/lib/GuildRail.svelte draws it: a column of
  // the person's Guilds on the shown Bakery, left of the sidebar, the one
  // shown marked by a pill on its left edge. No "+": a Guild is created in
  // the dashboard.
  import * as Tooltip from '@bakery/ui/components/ui/tooltip'
  import GuildIcon from '@bakery/ui/GuildIcon.svelte'
  import { cn } from '@bakery/ui/utils'
  import { connected } from './bakeries.svelte'
  import { shownGuilds } from './guilds.svelte'
  import { agentsPath, href, router } from './router.svelte'

  const guildID = $derived('guild' in router.route ? router.route.guild : null)
  const list = $derived(connected.shown && !connected.shown.signed_out ? (shownGuilds.list ?? []) : [])

  // Discord's marks: a rounded square instead of a circle on hover and for the
  // shown one, and a pill on the left edge, short on hover, tall when shown.
  const ITEM = 'group relative flex w-full justify-center outline-none'
  const PILL = 'absolute top-1/2 left-0 w-1 -translate-y-1/2 rounded-r-full bg-foreground transition-all duration-150'
  const ICON = 'size-12 transition-[border-radius] duration-150 group-focus-visible:ring-2 group-focus-visible:ring-ring'
</script>

<nav class="flex h-full w-[72px] shrink-0 flex-col items-center border-r border-border bg-background py-3" aria-label="Guilds" data-testid="guild-rail">
  <div class="flex min-h-0 w-full flex-1 flex-col items-center gap-2 overflow-y-auto [scrollbar-width:none]">
    {#each list as g (g.id)}
      {@const current = g.id === guildID}
      <Tooltip.Root>
        <Tooltip.Trigger>
          {#snippet child({ props })}
            <a
              {...props}
              href={href(agentsPath(connected.shownIndex, g.id))}
              class={ITEM}
              aria-label={g.name}
              aria-current={current ? 'true' : undefined}
              data-testid="guild-rail-item"
            >
              <span class={cn(PILL, current ? 'h-10' : 'h-0 group-hover:h-2')} data-testid="guild-rail-pill"></span>
              <GuildIcon name={g.name} class={cn(ICON, current ? 'rounded-2xl' : 'rounded-3xl group-hover:rounded-2xl')} />
            </a>
          {/snippet}
        </Tooltip.Trigger>
        <Tooltip.Content side="right" sideOffset={8}>{g.name}</Tooltip.Content>
      </Tooltip.Root>
    {/each}
  </div>
</nav>
