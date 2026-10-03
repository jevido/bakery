<script lang="ts">
  // Coolify's dropdown (resources/views/components/dropdown.blade.php): a
  // trigger with the up/down chevrons and a panel below it; on a phone the
  // panel is placed fixed so it stays on screen. `inline` makes it a
  // full-width field whose panel pushes the content down. Rows inside use
  // the `dropdown-item` class.
  import type { Snippet } from 'svelte'
  import { fly } from 'svelte/transition'

  let {
    title,
    inline = false,
    triggerClass = '',
    panelClass = '',
    children,
  }: {
    title: Snippet | string
    inline?: boolean
    triggerClass?: string
    panelClass?: string
    children: Snippet<[() => void]>
  } = $props()

  let open = $state(false)
  let panelStyle = $state('')
  let root = $state<HTMLDivElement>()
  let trigger = $state<HTMLButtonElement>()
  let panel = $state<HTMLDivElement>()

  function close() {
    open = false
  }

  function place() {
    if (inline || innerWidth >= 768 || !trigger || !panel) {
      panelStyle = ''
      return
    }
    const t = trigger.getBoundingClientRect()
    const p = panel.getBoundingClientRect()
    const padding = 8
    let left = t.left
    if (left + p.width + padding > innerWidth) left = innerWidth - p.width - padding
    left = Math.max(padding, left)
    let top = t.bottom + 4
    if (top > innerHeight - p.height - padding) top = Math.max(padding, t.top - p.height - padding)
    panelStyle = `position: fixed; left: ${left}px; top: ${top}px;`
  }

  $effect(() => {
    if (open && panel) place()
  })
</script>

<svelte:window
  onclick={(e) => !inline && open && root && !root.contains(e.target as Node) && close()}
  onkeydown={(e) => open && e.key === 'Escape' && close()}
  onresize={() => open && place()}
/>

<div class={['chrome relative', inline && 'w-full']} bind:this={root}>
  <button
    type="button"
    bind:this={trigger}
    aria-expanded={open}
    onclick={() => (open = !open)}
    class={[
      'inline-flex items-center justify-start pr-8 transition-colors focus:outline-hidden disabled:pointer-events-none disabled:opacity-50',
      inline && 'w-full border border-neutral-300 bg-white px-3 py-2 text-left dark:border-coolgray-300 dark:bg-coolgray-100',
      triggerClass,
    ]}
  >
    <span class="flex h-full flex-col items-start leading-none">
      {#if typeof title === 'string'}{title}{:else}{@render title()}{/if}
    </span>
    <svg class="absolute right-0 mr-3 h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" d="M8.25 15L12 18.75 15.75 15m-7.5-6L12 5.25 15.75 9" />
    </svg>
  </button>
  {#if open}
    <div
      bind:this={panel}
      style={panelStyle}
      transition:fly={{ y: -4, duration: 150 }}
      class={['origin-top', inline ? 'mt-1 w-full' : 'absolute top-full z-50 mt-1 max-w-[calc(100vw-1rem)] min-w-max md:top-0 md:mt-6']}
    >
      <div
        class={[
          'border border-neutral-300 bg-white p-1 dark:border-coolgray-300',
          inline ? 'border-0 bg-transparent shadow-none dark:border-0 dark:bg-transparent' : 'shadow-[var(--shadow-dropdown)] dark:bg-coolgray-200',
          panelClass,
        ]}
      >
        {@render children(close)}
      </div>
    </div>
  {/if}
</div>
