<script lang="ts">
  // Coolify's dropdown (resources/views/components/dropdown.blade.php): a
  // trigger with the up/down chevrons and a panel below it; on a phone the
  // panel is placed fixed so it stays on screen. `inline` makes it a
  // full-width field whose panel pushes the content down. Rows inside use
  // the `dropdown-item` class. Drawn as Paperclip's dropdown menu: an outline
  // trigger and a popover panel.
  import type { Snippet } from 'svelte'
  import { fly } from 'svelte/transition'
  import { motion } from './motion'
  import ChevronsUpDown from '@lucide/svelte/icons/chevrons-up-down'

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
      inline &&
        'h-9 w-full rounded-md border border-input bg-transparent px-3 py-2 text-left text-sm shadow-xs focus-visible:border-ring focus-visible:ring-(length:--rad-3) focus-visible:ring-ring/50 dark:bg-input/30 dark:hover:bg-input/50',
      triggerClass,
    ]}
  >
    <span class="flex items-center leading-none">
      {#if typeof title === 'string'}{title}{:else}{@render title()}{/if}
    </span>
    <ChevronsUpDown class="absolute right-0 mr-3 size-4 opacity-50" />
  </button>
  {#if open}
    <div
      bind:this={panel}
      style={panelStyle}
      transition:fly={{ y: -4, duration: motion(150) }}
      class={['origin-top', inline ? 'mt-1 w-full' : 'absolute top-full z-50 mt-1 max-w-[calc(100vw-1rem)] min-w-max md:top-0 md:mt-6']}
    >
      <div
        class={[
          'p-1',
          inline ? 'bg-transparent' : 'min-w-(--sz-8rem) rounded-md border bg-popover text-popover-foreground shadow-md',
          panelClass,
        ]}
      >
        {@render children(close)}
      </div>
    </div>
  {/if}
</div>
