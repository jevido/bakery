<script lang="ts">
  // The Filter popover of Paperclip's list toolbars (ui/src/pages/Issues.tsx;
  // MIT, see NOTICE), as the Environment page draws it: a ghost button naming
  // what is picked, a list of checkable options in optional groups and a
  // footer that clears them. Option values are unique across groups.
  import { buttonVariants } from '$lib/components/ui/button'
  import * as Popover from '$lib/components/ui/popover'
  import Icon from './Icon.svelte'

  type Option = { value: string; label: string }

  let {
    selected = $bindable([]),
    groups,
    label,
    clearLabel = 'Clear filters',
    onchange,
  }: {
    selected?: string[]
    groups: { label?: string; options: Option[] }[]
    label: string
    clearLabel?: string
    onchange?: () => void
  } = $props()

  const text = $derived(
    groups
      .flatMap((g) => g.options)
      .filter((o) => selected.includes(o.value))
      .map((o) => o.label)
      .join(', '),
  )

  function toggle(value: string) {
    selected = selected.includes(value) ? selected.filter((v) => v !== value) : [...selected, value]
    onchange?.()
  }
</script>

<Popover.Root>
  <Popover.Trigger
    class={buttonVariants({ variant: 'ghost', size: 'sm', class: ['max-w-64 text-xs', selected.length > 0 && 'bg-accent'].join(' ') })}
    title={text || 'Filter'}
  >
    <Icon name="filter" class="size-3.5 sm:size-3" />
    <span class="truncate">{text || 'Filter'}</span>
    {#if selected.length > 0}
      <span class="shrink-0 rounded-full bg-muted px-1.5 text-[10px] font-medium text-muted-foreground tabular-nums">{selected.length}</span>
    {/if}
  </Popover.Trigger>
  <Popover.Content align="start" class="w-56 p-0">
    <div class="max-h-80 overflow-y-auto p-2" role="listbox" aria-label={label} aria-multiselectable="true">
      {#each groups as group, i (i)}
        {#if group.options.length > 0}
          {#if group.label}
            <div class="px-2 pt-2 pb-1 text-[10px] font-semibold tracking-wide text-muted-foreground uppercase">{group.label}</div>
          {:else if i > 0}
            <div class="my-1 border-t border-border" aria-hidden="true"></div>
          {/if}
          {#each group.options as option (option.value)}
            {@const on = selected.includes(option.value)}
            <button
              type="button"
              role="option"
              aria-selected={on}
              class={[
                'flex w-full items-center justify-between gap-2 rounded-sm px-2 py-1.5 text-sm',
                on ? 'bg-accent/50 text-foreground' : 'text-muted-foreground hover:bg-accent/50',
              ]}
              onclick={() => toggle(option.value)}
            >
              <span class="min-w-0 flex-1 truncate text-left">{option.label}</span>
              <span
                class={[
                  'flex size-4 shrink-0 items-center justify-center rounded-sm border',
                  on ? 'border-primary bg-primary text-primary-foreground' : 'border-input',
                ]}
              >
                {#if on}<Icon name="check" class="size-3" />{/if}
              </span>
            </button>
          {/each}
        {/if}
      {/each}
    </div>
    <div class="border-t border-border p-2">
      <button
        type="button"
        class="w-full rounded-sm px-2 py-1.5 text-sm text-muted-foreground hover:bg-accent/50 hover:text-foreground disabled:opacity-50"
        disabled={selected.length === 0}
        onclick={() => {
          selected = []
          onchange?.()
        }}>{clearLabel}</button
      >
    </div>
  </Popover.Content>
</Popover.Root>
