<script lang="ts">
  // Paperclip's Org chart in its embedded form (ui/src/pages/OrgChart.tsx;
  // MIT, see NOTICE): a card per Agent with its icon, status dot, name and
  // Title (else Job), stepped lines from each Manager to its reports, pan by
  // dragging, zoom with the wheel or a pinch, and the zoom in, zoom out and
  // fit buttons; it fits itself on first show. Pointer events stand in for
  // Paperclip's separate mouse and touch handlers. Its import and export
  // buttons are left out, as the agents document says.
  import { Maximize2, Minus, Plus } from '@lucide/svelte'
  import AgentIcon from '@bakery/ui/AgentIcon.svelte'
  import type { AgentStatus, OrgNode } from './agents'
  import {
    CARD_H,
    CARD_W,
    chartBounds,
    clampZoom,
    collectEdges,
    fitChartToViewport,
    flattenLayout,
    layoutForest,
    panToward,
    type Point,
  } from './orgLayout'
  import { href } from './router.svelte'

  let { org }: { org: OrgNode[] } = $props()

  const TOUCH_MOVE_THRESHOLD = 6

  // Paperclip's status dot hues; a hire waiting for its Approval is grey.
  const dotColors: Record<AgentStatus, string> = {
    idle: '#facc15',
    paused: '#facc15',
    pending_approval: '#a3a3a3',
    terminated: '#a3a3a3',
  }

  const layout = $derived(layoutForest(org))
  const nodes = $derived(flattenLayout(layout))
  const edges = $derived(collectEdges(layout))
  const bounds = $derived(chartBounds(nodes))

  let viewport = $state<HTMLDivElement>()
  let pan = $state<Point>({ x: 0, y: 0 })
  let zoom = $state(1)

  // The pointers down on the chart, and the gesture they started: one
  // pointer pans, two pinch around their centre.
  const pointers = new Map<number, Point>()
  let gesture: { pan: Point; zoom: number; start: Point; distance: number } | null = null
  let moved = false
  let dragging = $state(false)

  const local = (p: Point): Point => {
    const rect = viewport!.getBoundingClientRect()
    return { x: p.x - rect.left, y: p.y - rect.top }
  }
  const centre = (): Point => {
    const [a, b] = [...pointers.values()]
    return local(b ? { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 } : a)
  }
  const spread = () => {
    const [a, b] = [...pointers.values()]
    return b ? Math.hypot(a.x - b.x, a.y - b.y) : 0
  }
  const begin = () => {
    gesture = pointers.size === 0 ? null : { pan, zoom, start: centre(), distance: spread() }
  }

  function onpointerdown(e: PointerEvent) {
    if (e.pointerType === 'mouse' && e.button !== 0) return
    // A mouse on a card clicks it, as in Paperclip; a finger may pan from one.
    if (e.pointerType === 'mouse' && (e.target as Element).closest('[data-org-card]')) return
    if (pointers.size === 0) moved = false
    pointers.set(e.pointerId, { x: e.clientX, y: e.clientY })
    dragging = true
    begin()
  }

  function onpointermove(e: PointerEvent) {
    if (!pointers.has(e.pointerId) || !gesture) return
    pointers.set(e.pointerId, { x: e.clientX, y: e.clientY })
    const at = centre()
    const dx = at.x - gesture.start.x
    const dy = at.y - gesture.start.y
    if (pointers.size >= 2 && gesture.distance > 0) {
      const distance = spread()
      const next = clampZoom(gesture.zoom * (distance / gesture.distance))
      moved ||= Math.abs(distance - gesture.distance) > TOUCH_MOVE_THRESHOLD || Math.hypot(dx, dy) > TOUCH_MOVE_THRESHOLD
      const scale = next / gesture.zoom
      zoom = next
      pan = { x: at.x - scale * (gesture.start.x - gesture.pan.x), y: at.y - scale * (gesture.start.y - gesture.pan.y) }
      return
    }
    moved ||= Math.hypot(dx, dy) > TOUCH_MOVE_THRESHOLD
    pan = { x: gesture.pan.x + dx, y: gesture.pan.y + dy }
  }

  function onpointerup(e: PointerEvent) {
    if (!pointers.delete(e.pointerId)) return
    dragging = pointers.size > 0
    begin()
  }

  // A drag that ends on a card must not open it.
  function onclickcapture(e: MouseEvent) {
    if (!moved) return
    moved = false
    e.preventDefault()
    e.stopPropagation()
  }

  const zoomToward = (next: number, point: Point) => {
    const clamped = clampZoom(next)
    pan = panToward(point, pan, zoom, clamped)
    zoom = clamped
  }
  const zoomCentre = (factor: number) => {
    if (viewport) zoomToward(zoom * factor, { x: viewport.clientWidth / 2, y: viewport.clientHeight / 2 })
  }
  const fit = () => {
    if (!viewport) return false
    const fitted = fitChartToViewport(viewport.clientWidth, viewport.clientHeight, bounds)
    if (!fitted) return false
    zoom = fitted.zoom
    pan = fitted.pan
    return true
  }

  // The wheel zooms toward the cursor; the listener is not passive so the
  // page does not scroll under it.
  $effect(() => {
    const el = viewport
    if (!el) return
    const onwheel = (e: WheelEvent) => {
      e.preventDefault()
      zoomToward(zoom * (e.deltaY < 0 ? 1.1 : 0.9), local({ x: e.clientX, y: e.clientY }))
    }
    el.addEventListener('wheel', onwheel, { passive: false })
    return () => el.removeEventListener('wheel', onwheel)
  })

  // Fit once per tree, as soon as the viewport has a size.
  let fittedFor: OrgNode[] | null = null
  $effect(() => {
    void bounds
    if (fittedFor === org || nodes.length === 0) return
    if (fit()) fittedFor = org
  })

  const controlClass =
    'flex size-9 items-center justify-center rounded border border-border bg-background text-sm transition-colors hover:bg-accent sm:size-7'
</script>

<!-- Paperclip's embedded chart fills its flex page; this page is not one, so
     the chart takes the height left under the top bar, tabs and count. -->
<div class="flex h-[calc(100dvh-13rem)] min-h-[420px] flex-col">
  <div
    bind:this={viewport}
    data-testid="org-chart-viewport"
    role="application"
    aria-label="Org chart"
    class="relative min-h-[420px] w-full flex-1 overflow-hidden rounded-lg border border-border bg-muted/20"
    style:cursor={dragging ? 'grabbing' : 'grab'}
    style:touch-action="none"
    style:overscroll-behavior="contain"
    {onpointerdown}
    {onpointermove}
    {onpointerup}
    onpointercancel={onpointerup}
    onpointerleave={onpointerup}
    onclickcapture={onclickcapture}
  >
    <div class="absolute top-3 right-3 z-10 flex flex-col gap-1.5">
      <button type="button" class={controlClass} onclick={() => zoomCentre(1.2)} title="Zoom in" aria-label="Zoom in">
        <Plus class="size-4 sm:size-3.5" />
      </button>
      <button type="button" class={controlClass} onclick={() => zoomCentre(0.8)} title="Zoom out" aria-label="Zoom out">
        <Minus class="size-4 sm:size-3.5" />
      </button>
      <button type="button" class={controlClass} onclick={fit} title="Fit to screen" aria-label="Fit chart to screen">
        <Maximize2 class="size-4 sm:size-3.5" />
      </button>
    </div>

    <svg class="pointer-events-none absolute inset-0 size-full" aria-hidden="true">
      <g transform={`translate(${pan.x}, ${pan.y}) scale(${zoom})`}>
        {#each edges as { parent, child } (`${parent.id}-${child.id}`)}
          {@const x1 = parent.x + CARD_W / 2}
          {@const y1 = parent.y + CARD_H}
          {@const x2 = child.x + CARD_W / 2}
          {@const midY = (y1 + child.y) / 2}
          <path
            data-testid="org-chart-edge"
            d={`M ${x1} ${y1} L ${x1} ${midY} L ${x2} ${midY} L ${x2} ${child.y}`}
            fill="none"
            stroke="var(--border)"
            stroke-width="1.5"
          />
        {/each}
      </g>
    </svg>

    <div
      data-testid="org-chart-card-layer"
      class="absolute inset-0"
      style:transform={`translate(${pan.x}px, ${pan.y}px) scale(${zoom})`}
      style:transform-origin="0 0"
    >
      {#each nodes as node (node.id)}
        <a
          data-org-card
          data-testid="org-chart-card"
          href={href(`/agents/${node.id}`)}
          draggable="false"
          class="absolute block cursor-pointer rounded-lg border bg-card text-card-foreground transition-[box-shadow,border-color] duration-150 select-none hover:border-foreground/20 hover:shadow-md"
          style:left={`${node.x}px`}
          style:top={`${node.y}px`}
          style:width={`${CARD_W}px`}
          style:min-height={`${CARD_H}px`}
        >
          <div class="flex items-center gap-3 px-4 py-3">
            <div class="relative shrink-0">
              <div class="flex size-9 items-center justify-center rounded-full bg-muted">
                <AgentIcon icon={node.icon} class="size-4.5 text-foreground/70" />
              </div>
              <span
                class="absolute -right-0.5 -bottom-0.5 size-3 rounded-full border-2 border-card"
                style:background-color={dotColors[node.status]}
                title={node.status}
              ></span>
            </div>
            <div class="flex min-w-0 flex-1 flex-col items-start">
              <span class="text-sm leading-tight font-semibold text-foreground">{node.name}</span>
              <span class="mt-0.5 text-micro leading-tight text-muted-foreground">{node.title || node.job_label}</span>
            </div>
          </div>
        </a>
      {/each}
    </div>
  </div>
</div>
