<script lang="ts" generics="T extends string | number | null">
  // The picker Paperclip's work pages open from a property (PickerButton in
  // ui/src/components/GoalProperties.tsx, the chips of NewGoalDialog.tsx;
  // MIT, see NOTICE): the trigger shows the current value, a click lists the
  // options and picking one closes it. `chip` draws the trigger as the
  // dialog's bordered chip, otherwise it is bare as in the properties panel.
  import type { Snippet } from 'svelte'
  import * as Popover from '@bakery/ui/components/ui/popover'

  let {
    value,
    options,
    label,
    chip = false,
    align = 'start',
    onpick,
    children,
  }: {
    value: T
    options: { value: T; label: string }[]
    /** Names the list for assistive technology, e.g. "Status". */
    label: string
    chip?: boolean
    align?: 'start' | 'end'
    onpick: (value: T) => void
    children: Snippet
  } = $props()

  let open = $state(false)
</script>

<Popover.Root bind:open>
  <Popover.Trigger
    aria-label={label}
    class={chip
      ? 'inline-flex items-center gap-1.5 rounded-md border px-2 py-1 text-xs transition-colors hover:bg-accent/50'
      : 'cursor-pointer transition-opacity hover:opacity-80'}
  >
    {@render children()}
  </Popover.Trigger>
  <Popover.Content {align} class="max-h-72 w-48 overflow-y-auto p-1">
    <div role="listbox" aria-label={label}>
      {#each options as option (option.value)}
        <button
          type="button"
          role="option"
          aria-selected={option.value === value}
          class={['flex w-full items-center gap-2 truncate rounded px-2 py-1.5 text-left text-xs hover:bg-accent/50', option.value === value && 'bg-accent']}
          onclick={() => {
            open = false
            onpick(option.value)
          }}
        >
          {option.label}
        </button>
      {/each}
    </div>
  </Popover.Content>
</Popover.Root>
