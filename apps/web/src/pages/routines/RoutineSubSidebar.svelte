<script lang="ts">
  // Paperclip's RoutineSubSidebar (ui/src/components/RoutineSubSidebar.tsx;
  // MIT, see NOTICE): the Routine page's sections in two groups, the
  // current one marked, arrow keys moving between them. Below md the same
  // list is a select. Left out with their sections: Variables, Secrets,
  // Delivery and History.
  import { Activity, Circle, Clock3, Play } from '@lucide/svelte'
  import * as Select from '@bakery/ui/components/ui/select'
  import { go, href, type RoutineSection } from '../../lib/router.svelte'

  let { id, section, dirty = false }: { id: number; section: RoutineSection; dirty?: boolean } = $props()

  type Item = { key: RoutineSection; label: string; icon: typeof Circle }
  const groups: { label: string; items: Item[] }[] = [
    {
      label: 'Routine',
      items: [
        { key: '', label: 'Overview', icon: Circle },
        { key: 'triggers', label: 'Triggers', icon: Clock3 },
      ],
    },
    {
      label: 'Operate',
      items: [
        { key: 'runs', label: 'Runs', icon: Play },
        { key: 'activity', label: 'Activity', icon: Activity },
      ],
    },
  ]
  const items = groups.flatMap((g) => g.items)
  const path = (key: RoutineSection) => `/routines/${id}${key ? `/${key}` : ''}`

  let links: HTMLAnchorElement[] = []

  function onkeydown(e: KeyboardEvent, index: number) {
    const to = ({ ArrowDown: index + 1, ArrowUp: index - 1, Home: 0, End: items.length - 1 } as Record<string, number>)[e.key]
    if (to === undefined) return
    e.preventDefault()
    const next = (to + items.length) % items.length
    links[next]?.focus()
    go(path(items[next].key))
  }
</script>

<nav aria-label="Routine sections" class="hidden w-52 shrink-0 self-stretch flex-col gap-4 overflow-y-auto border-r border-border bg-background px-3 py-4 md:flex">
  {#each groups as group (group.label)}
    <div class="flex flex-col gap-0.5">
      <p class="mx-2 px-2 pb-1 font-mono text-(length:--text-nano) font-medium tracking-widest text-muted-foreground/60 uppercase">{group.label}</p>
      {#each group.items as item (item.key)}
        {@const index = items.indexOf(item)}
        {@const active = item.key === section}
        <a
          bind:this={links[index]}
          href={href(path(item.key))}
          role="tab"
          aria-current={active ? 'page' : undefined}
          tabindex={active ? 0 : -1}
          onkeydown={(e: KeyboardEvent) => onkeydown(e, index)}
          class={[
            'mx-2 flex items-center gap-2.5 rounded-lg px-2 py-1.5 text-(length:--text-compact) font-medium no-underline transition-colors motion-safe:duration-150',
            active ? 'bg-accent text-foreground' : 'text-foreground/80 hover:bg-accent/50 hover:text-foreground',
          ]}
        >
          <item.icon class="size-4 shrink-0" />
          <span class="truncate">{item.label}</span>
          {#if item.key === '' && dirty}
            <span aria-label="Unsaved changes" class="ml-auto size-1.5 shrink-0 rounded-full bg-amber-500 ring-2 ring-background"></span>
          {/if}
        </a>
      {/each}
    </div>
  {/each}
</nav>

<div class="sticky top-0 z-10 border-b border-border bg-background px-4 py-2 md:hidden">
  <Select.Root type="single" value={section} onValueChange={(v) => go(path(v as RoutineSection))}>
    <Select.Trigger class="h-11 w-full" aria-label="Routine section">{items.find((i) => i.key === section)?.label}</Select.Trigger>
    <Select.Content>
      {#each groups as group (group.label)}
        <Select.Group>
          <Select.GroupHeading class="text-(length:--text-micro) uppercase">{group.label}</Select.GroupHeading>
          {#each group.items as item (item.key)}
            <Select.Item value={item.key} label={item.label} class="h-11" />
          {/each}
        </Select.Group>
      {/each}
    </Select.Content>
  </Select.Root>
</div>
