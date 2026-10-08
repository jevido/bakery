<script lang="ts">
  // Coolify's page-size-select (resources/views/components/page-size-select.blade.php):
  // the rows per page from a short list or "Custom…" (1–100), remembered under
  // `storageKey` when one is given.
  import { untrack } from 'svelte'
  import * as Popover from '@bakery/ui/components/ui/popover'
  import Icon from '../Icon.svelte'

  let {
    value = $bindable(),
    options = [10, 25, 50, 100],
    storageKey,
    onchange,
  }: {
    value: number
    options?: number[]
    storageKey?: string
    onchange?: (size: number) => void
  } = $props()

  let open = $state(false)
  let customizing = $state(false)
  const option_ = 'flex w-full items-center justify-between rounded-sm px-2 py-1.5 text-sm tabular-nums'
  let custom = $state(untrack(() => value))
  let customInput = $state<HTMLInputElement>()

  function apply(size: number) {
    const clamped = Math.min(100, Math.max(1, Math.round(Number(size)) || 1))
    value = clamped
    custom = clamped
    customizing = false
    if (storageKey) localStorage.setItem(storageKey, String(clamped))
    onchange?.(clamped)
  }

  // The saved size is read once, when the list first shows.
  untrack(() => {
    const saved = storageKey ? Number(localStorage.getItem(storageKey)) : 0
    if (saved >= 1 && saved <= 100) apply(saved)
  })

  $effect(() => {
    if (customizing) customInput?.focus()
  })
</script>

<div class="flex h-7 items-center gap-1.5 text-xs text-muted-foreground">
  {#if !customizing}
    <Popover.Root bind:open>
      <Popover.Trigger
        aria-label="Items per page"
        aria-haspopup="listbox"
        class="inline-flex h-7 w-12 items-center justify-between rounded-sm px-1 text-xs leading-none text-muted-foreground tabular-nums transition-colors outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50"
      >
        <span>{value}</span>
        <Icon name="chevron-down" class="size-3 opacity-60" />
      </Popover.Trigger>
      <Popover.Content align="start" class="w-28 p-1">
        <div class="space-y-0.5" role="listbox" aria-label="Items per page">
          {#each options as option (option)}
            <button
              type="button"
              class={[option_, value === option ? 'bg-accent/50 text-foreground' : 'text-muted-foreground hover:bg-accent/50']}
              role="option"
              aria-selected={value === option}
              onclick={() => {
                apply(option)
                open = false
              }}
            >
              <span>{option}</span>
              {#if value === option}<Icon name="check" class="size-3 text-muted-foreground" />{/if}
            </button>
          {/each}
          <button
            type="button"
            class={[option_, 'text-muted-foreground hover:bg-accent/50']}
            role="option"
            aria-selected="false"
            onclick={() => {
              open = false
              customizing = true
            }}
          >
            Custom…
          </button>
        </div>
      </Popover.Content>
    </Popover.Root>
  {:else}
    <input
      bind:this={customInput}
      bind:value={custom}
      onkeydown={(e) => {
        if (e.key === 'Enter') {
          e.preventDefault()
          apply(custom)
        } else if (e.key === 'Escape') {
          e.preventDefault()
          customizing = false
        }
      }}
      onblur={() => customizing && apply(custom)}
      type="number"
      min="1"
      max="100"
      inputmode="numeric"
      aria-label="Custom items per page"
      class="h-7 w-14 rounded-md border bg-transparent px-1.5 py-0 text-xs text-foreground tabular-nums outline-none focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/50"
    />
  {/if}
</div>
