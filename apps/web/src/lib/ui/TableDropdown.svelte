<script lang="ts">
  // Coolify's table dropdown (resources/views/components/table/dropdown.blade.php):
  // a trigger and a listbox panel placed fixed under the trigger's right edge,
  // or above it when there is no room below. Rows inside use `listbox-option`.
  // The panel is Paperclip's dropdown menu content; it stays this hand-placed
  // panel rather than bits-ui's DropdownMenu, whose roving focus and typeahead
  // would change the rows callers render.
  import type { Snippet } from 'svelte'
  import { scale } from 'svelte/transition'

  let {
    panelClass = '',
    role = 'listbox',
    trigger,
    children,
  }: {
    panelClass?: string
    role?: string
    trigger: Snippet<[{ open: boolean; toggle: () => void }]>
    children: Snippet<[() => void]>
  } = $props()

  let open = $state(false)
  let panelStyle = $state('position: fixed; min-width: 0; visibility: hidden;')
  let root = $state<HTMLDivElement>()
  let triggerBox = $state<HTMLDivElement>()
  let panel = $state<HTMLDivElement>()

  function toggle() {
    if (open) {
      open = false
      return
    }
    panelStyle = 'position: fixed; min-width: 0; visibility: hidden;'
    open = true
  }

  function close() {
    open = false
  }

  function place() {
    if (!triggerBox || !panel) return
    const t = triggerBox.getBoundingClientRect()
    const p = panel.getBoundingClientRect()
    const padding = 8
    const left = Math.max(padding, Math.min(t.right - p.width, innerWidth - p.width - padding))
    const top = innerHeight - t.bottom - padding >= p.height ? t.bottom + 4 : Math.max(padding, t.top - p.height - 4)
    panelStyle = `position: fixed; left: ${left}px; top: ${top}px; min-width: 0;`
  }

  $effect(() => {
    if (open && panel) place()
  })
</script>

<svelte:window
  onclick={(e) => open && root && !root.contains(e.target as Node) && close()}
  onkeydown={(e) => open && e.key === 'Escape' && close()}
  onresize={() => open && place()}
  onscroll={() => open && place()}
/>

<div class="relative" bind:this={root}>
  <div bind:this={triggerBox}>{@render trigger({ open, toggle })}</div>
  {#if open}
    <div
      bind:this={panel}
      style={panelStyle}
      {role}
      transition:scale={{ start: 0.98, duration: 120 }}
      class={[
        'fixed! right-auto! bottom-auto! z-[90]! mt-0! flex max-h-64 w-max max-w-[min(24rem,calc(100vw-1.5rem))] origin-top flex-col gap-px overflow-x-hidden overflow-y-auto rounded-md border bg-popover p-1 text-popover-foreground shadow-md',
        panelClass,
      ]}
    >
      {@render children(close)}
    </div>
  {/if}
</div>
