<script lang="ts">
  // Coolify's notification event multiselect (resources/views/components/notification/event-multiselect.blade.php,
  // Apache-2.0, see NOTICE): a listbox trigger naming the selected events
  // with an n/m count, and a panel of check boxes; a click toggles one. Drawn
  // as phase 29's FilterPopover.svelte draws its options (src/lib/FilterPopover.svelte).
  import ChevronsUpDown from '@lucide/svelte/icons/chevrons-up-down'
  import { Checkbox } from '$lib/components/ui/checkbox'
  import * as Popover from '$lib/components/ui/popover'

  let {
    id,
    label,
    events,
    disabled = false,
    ontoggle,
  }: {
    id: string
    label: string
    events: { kind: string; label: string; enabled: boolean }[]
    disabled?: boolean
    ontoggle: (kind: string) => unknown
  } = $props()

  let open = $state(false)
  const selected = $derived(events.filter((e) => e.enabled))
  const selectedLabels = $derived(selected.map((e) => e.label).join(', '))
</script>

<div class="w-full min-w-0">
  <label for="{id}-trigger" class="mb-1.5 block text-sm font-medium text-foreground">{label}</label>
  <Popover.Root bind:open>
    <Popover.Trigger
      id="{id}-trigger"
      {disabled}
      class="border-input flex h-9 w-full min-w-0 items-center gap-2 rounded-md border bg-transparent px-3 py-1 text-sm shadow-xs outline-none focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px] disabled:cursor-not-allowed disabled:opacity-50"
      aria-haspopup="listbox"
      title={selected.length > 0 ? selectedLabels : undefined}
      data-testid="event-multiselect"
    >
      <span class="min-w-0 flex-1 truncate text-left" class:text-muted-foreground={selected.length === 0}>
        {selected.length === 0 ? 'No events selected' : selectedLabels}
      </span>
      <span class="shrink-0 rounded-full bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground tabular-nums">
        {selected.length}/{events.length}
      </span>
      <ChevronsUpDown class="size-3.5 shrink-0 opacity-50" />
    </Popover.Trigger>
    <Popover.Content align="start" class="w-72 p-0">
      <div class="max-h-80 overflow-y-auto p-2" role="listbox" aria-multiselectable="true">
        {#each events as event (event.kind)}
          <label
            class="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm text-foreground hover:bg-accent/50"
            data-testid="event-option"
          >
            <Checkbox checked={event.enabled} {disabled} onCheckedChange={() => ontoggle(event.kind)} />
            <span class="min-w-0 flex-1 truncate">{event.label}</span>
          </label>
        {/each}
      </div>
    </Popover.Content>
  </Popover.Root>
</div>
