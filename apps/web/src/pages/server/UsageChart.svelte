<script lang="ts" module>
  export type Sample = { at: number; value: number }

  /** Coolify's formatPercent: two decimals below 1 %, one above. */
  export function formatPercent(value: number): string {
    const precision = Math.abs(value) < 1 ? 2 : 1
    return `${Number(value.toFixed(precision))}%`
  }

  // ApexCharts' forceNiceScale with tickAmount 4 over 0..max*1.2.
  function niceTop(max: number): { top: number; step: number } {
    const target = max > 0 ? max * 1.2 : 1
    const raw = target / 4
    const pow = 10 ** Math.floor(Math.log10(raw))
    const n = raw / pow
    const step = (n <= 1 ? 1 : n <= 2 ? 2 : n <= 2.5 ? 2.5 : n <= 5 ? 5 : 10) * pow
    return { top: step * Math.ceil(target / step), step }
  }
</script>

<script lang="ts">
  // The area chart of Coolify's server/charts.blade.php (ApexCharts there),
  // drawn as inline SVG: a smooth line over a fading fill, a dashed grid,
  // percent ticks on the left, UTC times below, and the value under the
  // pointer.
  let { name, color, samples, empty }: { name: string; color: string; samples: Sample[]; empty: string } = $props()

  const height = 240
  const pad = { top: 12, right: 12, bottom: 28, left: 48 }
  let width = $state(600)

  const gradientId = $props.id()

  const scale = $derived(niceTop(Math.max(0, ...samples.map((s) => s.value))))
  const ticks = $derived(Array.from({ length: Math.round(scale.top / scale.step) + 1 }, (_, i) => i * scale.step))
  const t0 = $derived(samples[0]?.at ?? 0)
  const t1 = $derived(samples.at(-1)?.at ?? 0)
  const x = (at: number) => pad.left + (t1 > t0 ? ((at - t0) / (t1 - t0)) * (width - pad.left - pad.right) : (width - pad.left - pad.right) / 2)
  const y = (v: number) => pad.top + (1 - v / scale.top) * (height - pad.top - pad.bottom)

  const points = $derived(samples.map((s) => [x(s.at), y(s.value)] as const))

  // A Catmull-Rom curve through every sample, as "curve: smooth" draws it.
  const line = $derived.by(() => {
    if (points.length === 0) return ''
    let d = `M${points[0][0]},${points[0][1]}`
    for (let i = 1; i < points.length; i++) {
      const [p0, p1, p2, p3] = [points[i - 2] ?? points[i - 1], points[i - 1], points[i], points[i + 1] ?? points[i]]
      const c1 = [p1[0] + (p2[0] - p0[0]) / 6, p1[1] + (p2[1] - p0[1]) / 6]
      const c2 = [p2[0] - (p3[0] - p1[0]) / 6, p2[1] - (p3[1] - p1[1]) / 6]
      d += ` C${c1[0]},${c1[1]} ${c2[0]},${c2[1]} ${p2[0]},${p2[1]}`
    }
    return d
  })
  const area = $derived(
    points.length > 1 ? `${line} L${points.at(-1)![0]},${y(0)} L${points[0][0]},${y(0)} Z` : '',
  )

  const time = new Intl.DateTimeFormat(undefined, { timeZone: 'UTC', hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false })
  const stamp = new Intl.DateTimeFormat(undefined, { timeZone: 'UTC', dateStyle: 'short', timeStyle: 'medium', hour12: false })
  const timeTicks = $derived.by(() => {
    if (samples.length < 2) return samples.map((s) => s.at)
    const n = width < 480 ? 3 : 5
    return Array.from({ length: n }, (_, i) => t0 + ((t1 - t0) * i) / (n - 1))
  })

  let hover = $state<number | null>(null)
  function pointer(e: PointerEvent) {
    if (samples.length === 0) return
    const rect = (e.currentTarget as SVGElement).getBoundingClientRect()
    const px = e.clientX - rect.left
    let best = 0
    for (let i = 1; i < points.length; i++) if (Math.abs(points[i][0] - px) < Math.abs(points[best][0] - px)) best = i
    hover = best
  }
</script>

<div class="relative min-h-[240px] w-full" bind:clientWidth={width} data-testid="usage-chart" data-samples={samples.length}>
  <svg
    {width}
    {height}
    viewBox="0 0 {width} {height}"
    role="img"
    aria-label="{name} usage"
    class="block text-black dark:text-white"
    onpointermove={pointer}
    onpointerleave={() => (hover = null)}
  >
    <defs>
      <linearGradient id={gradientId} x1="0" x2="0" y1="0" y2="1">
        <stop offset="0%" stop-color={color} stop-opacity="0.28" />
        <stop offset="90%" stop-color={color} stop-opacity="0.02" />
        <stop offset="100%" stop-color={color} stop-opacity="0.02" />
      </linearGradient>
    </defs>
    {#each ticks as t (t)}
      <line x1={pad.left} x2={width - pad.right} y1={y(t)} y2={y(t)} stroke="rgba(128, 128, 128, 0.14)" stroke-dasharray="4" />
      <text x={pad.left - 8} y={y(t)} dy="0.32em" text-anchor="end" font-size="11" fill="currentColor">{formatPercent(t)}</text>
    {/each}
    {#each timeTicks as t, i (i)}
      <text
        x={x(t)}
        y={height - 8}
        text-anchor={timeTicks.length < 2 ? 'middle' : i === 0 ? 'start' : i === timeTicks.length - 1 ? 'end' : 'middle'}
        font-size="11"
        fill="currentColor">{time.format(t)}</text
      >
    {/each}
    {#if area}<path d={area} fill="url(#{gradientId})" />{/if}
    {#if line}<path d={line} fill="none" stroke={color} stroke-width="2" stroke-linejoin="round" />{/if}
    {#if points.length === 1}<circle cx={points[0][0]} cy={points[0][1]} r="3" fill={color} />{/if}
    {#if hover !== null && points[hover]}
      <line x1={points[hover][0]} x2={points[hover][0]} y1={pad.top} y2={y(0)} stroke="rgba(128, 128, 128, 0.4)" stroke-dasharray="3" />
      <circle cx={points[hover][0]} cy={points[hover][1]} r="4" fill={color} />
    {/if}
  </svg>
  {#if samples.length === 0}
    <p class="absolute inset-0 flex items-center justify-center text-sm text-neutral-500 dark:text-fg-dim">{empty}</p>
  {/if}
  {#if hover !== null && samples[hover]}
    <div
      class="pointer-events-none absolute top-2 rounded-md border border-neutral-200 bg-white px-2.5 py-1.5 text-xs shadow-sm dark:border-white/[0.08] dark:bg-raised"
      style:left="{Math.min(points[hover][0] + 8, width - 180)}px"
    >
      <div class="text-neutral-700 dark:text-fg-dim">{name}: <span class="font-semibold text-black dark:text-fg">{formatPercent(samples[hover].value)}</span></div>
      <div class="text-neutral-500 dark:text-fg-faint">{stamp.format(samples[hover].at)} UTC</div>
    </div>
  {/if}
</div>
