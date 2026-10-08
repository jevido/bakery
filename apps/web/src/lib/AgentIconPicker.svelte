<script lang="ts">
  // Paperclip's AgentIconPicker (ui/src/components/AgentIconPicker.tsx; MIT,
  // see NOTICE): the trigger opens a popover with a search field over a
  // grid of the Agent icons; picking one closes it.
  import type { Snippet } from 'svelte'
  import { Input } from '$lib/components/ui/input'
  import * as Popover from '$lib/components/ui/popover'
  import { agentIcons } from './AgentIcon.svelte'
  import { agentIconNames } from './agents'

  let {
    value,
    onchange,
    label = 'Agent icon',
    children,
  }: { value: string | null | undefined; onchange: (icon: string) => void; label?: string; children: Snippet } = $props()

  let open = $state(false)
  let search = $state('')

  const shown = $derived(agentIconNames.filter((name) => !search || name.includes(search.toLowerCase())))
</script>

<Popover.Root bind:open onOpenChange={(o) => !o && (search = '')}>
  <Popover.Trigger aria-label={label} class="cursor-pointer transition-opacity hover:opacity-80">
    {@render children()}
  </Popover.Trigger>
  <Popover.Content align="start" class="w-72 p-3">
    <!-- svelte-ignore a11y_autofocus -->
    <Input placeholder="Search icons..." bind:value={search} class="mb-2 h-8 text-sm" autofocus aria-label="Search icons" />
    <div class="grid max-h-48 grid-cols-7 gap-1 overflow-y-auto" role="listbox" aria-label={label}>
      {#each shown as name (name)}
        {@const Glyph = agentIcons[name]}
        <button
          type="button"
          role="option"
          aria-selected={(value || 'bot') === name}
          title={name}
          class={['flex size-8 items-center justify-center rounded transition-colors hover:bg-accent', (value || 'bot') === name && 'bg-accent ring-1 ring-primary']}
          onclick={() => {
            onchange(name)
            open = false
            search = ''
          }}
        >
          <Glyph class="size-4" />
        </button>
      {/each}
      {#if shown.length === 0}
        <p class="col-span-7 py-2 text-center text-xs text-muted-foreground">No icons match</p>
      {/if}
    </div>
  </Popover.Content>
</Popover.Root>
