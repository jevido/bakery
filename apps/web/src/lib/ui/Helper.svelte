<script lang="ts">
  // Coolify's helper (resources/views/components/helper.blade.php): the `?`
  // (info-circle) that opens a fixed popup on hover, focus or tap, placed
  // under the trigger and kept inside the viewport, drawn as Paperclip's
  // popover.
  import type { Snippet } from 'svelte'
  import Icon from '../Icon.svelte'

  let {
    helper,
    label = 'More information',
    icon,
  }: { helper: string | Snippet; label?: string; icon?: Snippet } = $props()

  const id = $props.id()
  let open = $state(false)
  let style = $state('')
  let trigger = $state<HTMLButtonElement>()
  let popup = $state<HTMLDivElement>()
  let hideTimer: ReturnType<typeof setTimeout> | undefined

  function show() {
    clearTimeout(hideTimer)
    open = true
  }

  function hide() {
    clearTimeout(hideTimer)
    hideTimer = setTimeout(() => (open = false), 150)
  }

  function position() {
    if (!trigger || !popup) return
    const padding = 8
    const width = Math.min(320, Math.max(0, innerWidth - padding * 2))
    popup.style.maxWidth = `${width}px`
    const t = trigger.getBoundingClientRect()
    const p = popup.getBoundingClientRect()
    let top = t.bottom + padding
    let left = t.right - p.width
    if (top + p.height > innerHeight - padding) top = t.top - p.height - padding
    left = Math.min(Math.max(padding, left), innerWidth - p.width - padding)
    top = Math.min(Math.max(padding, top), innerHeight - p.height - padding)
    style = `max-width: ${width}px; max-height: ${innerHeight - padding * 2}px; overflow-y: auto; top: ${top}px; left: ${left}px;`
  }

  $effect(() => {
    if (open && popup) position()
  })

  function outside(e: PointerEvent) {
    if (!open) return
    const target = e.target as Node
    if (trigger?.contains(target) || popup?.contains(target)) return
    open = false
  }
</script>

<svelte:window
  onpointerdown={outside}
  onkeydown={(e) => e.key === 'Escape' && (open = false)}
  onresize={() => open && position()}
  onscroll={() => open && position()}
/>

<div class="chrome relative inline-flex align-middle">
  <button
    type="button"
    bind:this={trigger}
    class={['relative inline-flex cursor-pointer text-muted-foreground transition-colors hover:text-foreground shrink-0 items-center justify-center border-0 bg-transparent p-0 leading-none', !icon && 'size-3.5']}
    aria-label={label}
    aria-describedby={open ? id : undefined}
    onmouseenter={show}
    onmouseleave={hide}
    onfocus={show}
    onblur={hide}
    onclick={(e) => {
      e.preventDefault()
      e.stopPropagation()
      show()
    }}
  >
    {#if icon}
      {@render icon()}
    {:else}
      <Icon name="info-circle" class="size-3.5 text-muted-foreground transition-colors hover:text-foreground" />
    {/if}
  </button>
  {#if open}
    <div
      bind:this={popup}
      {id}
      role="tooltip"
      {style}
      class="fixed z-[10000] w-max max-w-[min(20rem,calc(100vw-2rem))] rounded-md border bg-popover break-words whitespace-normal text-popover-foreground shadow-md"
      onmouseenter={show}
      onmouseleave={hide}
    >
      <div class="px-3 py-2 text-xs leading-5">
        {#if typeof helper === 'string'}{helper}{:else}{@render helper()}{/if}
      </div>
    </div>
  {/if}
</div>
