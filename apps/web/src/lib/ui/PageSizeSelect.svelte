<script lang="ts">
  // Coolify's page-size-select (resources/views/components/page-size-select.blade.php):
  // the rows per page from a short list or "Custom…" (1–100), remembered under
  // `storageKey` when one is given.
  import { untrack } from 'svelte'
  import Icon from '../Icon.svelte'
  import TableDropdown from './TableDropdown.svelte'

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

  let customizing = $state(false)
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

<div class="mb-0! flex h-7 items-center gap-1.5 text-[11px] text-neutral-500 dark:text-fg-dim">
  {#if !customizing}
    <span class="relative inline-flex h-7 w-12 items-center">
      <TableDropdown panelClass="min-w-24!">
        {#snippet trigger({ open, toggle })}
          <button
            type="button"
            aria-label="Items per page"
            aria-haspopup="listbox"
            aria-expanded={open}
            onclick={toggle}
            class="inline-flex h-7! w-12! items-center justify-between border-0 px-1 text-[11px]! leading-none! text-neutral-500 tabular-nums transition-colors hover:text-black dark:text-fg-dim dark:hover:text-fg"
          >
            <span>{value}</span>
            <Icon name="chevron-down" class="size-3 text-neutral-400 dark:text-fg-faint" />
          </button>
        {/snippet}
        {#snippet children(close)}
          {#each options as option (option)}
            <button
              type="button"
              class="listbox-option"
              role="option"
              aria-selected={value === option}
              onclick={() => {
                apply(option)
                close()
              }}
            >
              <span>{option}</span>
              {#if value === option}<Icon name="check" class="size-3.5" />{/if}
            </button>
          {/each}
          <button
            type="button"
            class="listbox-option"
            role="option"
            aria-selected="false"
            onclick={() => {
              close()
              customizing = true
            }}
          >
            Custom…
          </button>
        {/snippet}
      </TableDropdown>
    </span>
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
      class="mb-0! h-7! w-14! rounded-md! border-neutral-200! bg-transparent! px-1.5! py-0! text-[11px]! tabular-nums shadow-none! focus:border-neutral-300! focus:ring-0! dark:border-white/[0.08]! dark:text-fg-dim!"
    />
  {/if}
</div>
