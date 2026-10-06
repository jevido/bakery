<script lang="ts">
  // Coolify's notification event multiselect (resources/views/components/notification/event-multiselect.blade.php,
  // Apache-2.0, see NOTICE): a listbox trigger naming the selected events
  // with an n/m count, and a panel of check boxes; a click toggles one.
  import { cubicIn, cubicOut } from 'svelte/easing'
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
  let root = $state<HTMLDivElement>()
  const selected = $derived(events.filter((e) => e.enabled))
  const selectedLabels = $derived(selected.map((e) => e.label).join(', '))

  // Coolify's x-transition: from 0.25rem up, 98 % and transparent.
  function pop(_: Element, { duration, easing }: { duration: number; easing: (t: number) => number }) {
    return {
      duration,
      easing,
      css: (t: number) => `opacity: ${t}; transform: translateY(${(t - 1) * 0.25}rem) scale(${0.98 + 0.02 * t})`,
    }
  }
</script>

<svelte:window
  onclick={(e) => {
    if (open && root && !root.contains(e.target as Node)) open = false
  }}
  onkeydown={(e) => {
    if (e.key === 'Escape') open = false
  }}
/>

<div class="w-full min-w-0" bind:this={root}>
  <label for="{id}-trigger" class="mb-1.5 block text-[12px] font-medium text-black dark:text-fg">{label}</label>
  <div class="relative min-w-0">
    <button
      id="{id}-trigger"
      type="button"
      class="listbox-trigger"
      onclick={() => (open = !open)}
      {disabled}
      aria-haspopup="listbox"
      aria-expanded={open}
      title={selected.length > 0 ? selectedLabels : undefined}
      data-testid="event-multiselect"
    >
      <span class="listbox-trigger-label">{selected.length === 0 ? 'No events selected' : selectedLabels}</span>
      <span
        class="shrink-0 rounded-full bg-neutral-100 px-1.5 py-0.5 text-[10px] font-medium text-neutral-500 dark:bg-white/[0.07] dark:text-fg-dim"
      >
        {selected.length}/{events.length}
      </span>
      <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" class="size-3.5 shrink-0 opacity-60">
        <path stroke-linecap="round" stroke-linejoin="round" d="m8 9 4-4 4 4m0 6-4 4-4-4" />
      </svg>
    </button>

    {#if open}
      <div class="listbox-panel" role="listbox" aria-multiselectable="true" in:pop={{ duration: 100, easing: cubicOut }} out:pop={{ duration: 75, easing: cubicIn }}>
        {#each events as event (event.kind)}
          <button
            type="button"
            class="listbox-option"
            role="option"
            aria-selected={event.enabled}
            onclick={() => ontoggle(event.kind)}
            {disabled}
            data-testid="event-option"
          >
            <span class="truncate">{event.label}</span>
            <span
              class={[
                'flex size-4 shrink-0 items-center justify-center rounded-[5px] border',
                event.enabled
                  ? 'border-coollabs bg-coollabs text-white dark:border-warning dark:bg-warning dark:text-black'
                  : 'border-neutral-300 bg-white dark:border-white/[0.14] dark:bg-white/[0.045]',
              ]}
            >
              {#if event.enabled}
                <svg class="size-3" viewBox="0 0 12 12" fill="none" aria-hidden="true">
                  <path d="m2.25 6.15 2.35 2.3 5.15-5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
                </svg>
              {/if}
            </span>
          </button>
        {/each}
      </div>
    {/if}
  </div>
</div>
