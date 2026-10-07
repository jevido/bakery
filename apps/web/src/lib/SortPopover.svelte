<script lang="ts" generics="T extends string">
  // Paperclip's "Sort: <label>" popover (ui/src/pages/Projects.tsx; MIT, see
  // NOTICE): a ghost button naming the current order, opening the options.
  import { buttonVariants } from '$lib/components/ui/button'
  import * as Popover from '$lib/components/ui/popover'
  import Icon from './Icon.svelte'

  let {
    value = $bindable(),
    options,
    label,
    onchange,
  }: { value: T; options: { value: T; label: string }[]; label: string; onchange?: () => void } = $props()

  const current = $derived(options.find((o) => o.value === value)?.label ?? options[0]?.label)
</script>

<Popover.Root>
  <Popover.Trigger class={buttonVariants({ variant: 'ghost', size: 'sm', class: 'w-fit text-xs' })} title="Sort">
    <Icon name="sort-direction" class="size-3.5 sm:size-3" />
    <span>Sort: {current}</span>
  </Popover.Trigger>
  <Popover.Content align="start" class="w-48 p-2">
    <div class="space-y-0.5" role="listbox" aria-label={label}>
      {#each options as option (option.value)}
        <button
          type="button"
          role="option"
          aria-selected={value === option.value}
          class={[
            'flex w-full items-center justify-between rounded-sm px-2 py-1.5 text-sm',
            value === option.value ? 'bg-accent/50 text-foreground' : 'text-muted-foreground hover:bg-accent/50',
          ]}
          onclick={() => {
            value = option.value
            onchange?.()
          }}
        >
          <span>{option.label}</span>
          {#if value === option.value}<Icon name="check" class="size-3 text-muted-foreground" />{/if}
        </button>
      {/each}
    </div>
  </Popover.Content>
</Popover.Root>
