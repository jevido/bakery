<script lang="ts" module>
  // Paperclip's CompanyPatternIcon (ui/src/components/CompanyPatternIcon.tsx;
  // MIT, see NOTICE): a Guild's name, hashed, seeds a two-tone Bayer 4×4
  // dither drawn to a canvas, with the name's initial over it. A Guild has no
  // logo yet, so the pattern is all there is.
  const BAYER_4X4 = [
    [0, 8, 2, 10],
    [12, 4, 14, 6],
    [3, 11, 1, 9],
    [15, 7, 13, 5],
  ] as const

  function hashString(value: string): number {
    let hash = 2166136261
    for (let i = 0; i < value.length; i++) {
      hash ^= value.charCodeAt(i)
      hash = Math.imul(hash, 16777619)
    }
    return hash >>> 0
  }

  function mulberry32(seed: number): () => number {
    let state = seed >>> 0
    return () => {
      state = (state + 0x6d2b79f5) >>> 0
      let t = Math.imul(state ^ (state >>> 15), 1 | state)
      t ^= t + Math.imul(t ^ (t >>> 7), 61 | t)
      return ((t ^ (t >>> 14)) >>> 0) / 4294967296
    }
  }

  function hslToRgb(h: number, s: number, l: number): [number, number, number] {
    const hue = ((h % 360) + 360) % 360
    const sat = Math.max(0, Math.min(100, s)) / 100
    const light = Math.max(0, Math.min(100, l)) / 100
    const c = (1 - Math.abs(2 * light - 1)) * sat
    const x = c * (1 - Math.abs(((hue / 60) % 2) - 1))
    const m = light - c / 2
    const [r, g, b] =
      hue < 60 ? [c, x, 0] : hue < 120 ? [x, c, 0] : hue < 180 ? [0, c, x] : hue < 240 ? [0, x, c] : hue < 300 ? [x, 0, c] : [c, 0, x]
    return [Math.round((r + m) * 255), Math.round((g + m) * 255), Math.round((b + m) * 255)]
  }

  // The same name always draws the same picture, so it is drawn once.
  const cache = new Map<string, string>()

  function pattern(seed: string, logicalSize = 22, cellSize = 2): string {
    const hit = cache.get(seed)
    if (hit !== undefined) return hit
    const canvas = document.createElement('canvas')
    canvas.width = logicalSize * cellSize
    canvas.height = logicalSize * cellSize
    const ctx = canvas.getContext('2d')
    if (!ctx) return ''

    const rand = mulberry32(hashString(seed))
    const hue = Math.floor(rand() * 360)
    const [offR, offG, offB] = hslToRgb(hue, 54 + Math.floor(rand() * 14), 36 + Math.floor(rand() * 12))
    const [onR, onG, onB] = hslToRgb(hue + (rand() > 0.5 ? 10 : -10), 86 + Math.floor(rand() * 10), 82 + Math.floor(rand() * 10))

    const center = (logicalSize - 1) / 2
    const half = Math.max(center, 1)
    const angle = rand() * Math.PI * 2
    const dirX = Math.cos(angle)
    const dirY = Math.sin(angle)
    const maxProjection = Math.abs(dirX * half) + Math.abs(dirY * half)
    const diagonalFrequency = 0.34 + rand() * 0.12
    const antiDiagonalFrequency = 0.33 + rand() * 0.12
    const diagonalPhase = rand() * Math.PI * 2
    const antiDiagonalPhase = rand() * Math.PI * 2

    ctx.fillStyle = `rgb(${offR} ${offG} ${offB})`
    ctx.fillRect(0, 0, canvas.width, canvas.height)
    ctx.fillStyle = `rgb(${onR} ${onG} ${onB})`
    const dotRadius = cellSize * 0.46

    for (let y = 0; y < logicalSize; y++) {
      const dy = y - center
      for (let x = 0; x < logicalSize; x++) {
        const dx = x - center
        // A side-to-side gradient whose shade comes from the dither's density.
        const gradient = ((dx * dirX + dy * dirY) / maxProjection + 1) * 0.5
        const diagonal = Math.sin((dx + dy) * diagonalFrequency + diagonalPhase) * 0.5 + 0.5
        const antiDiagonal = Math.sin((dx - dy) * antiDiagonalFrequency + antiDiagonalPhase) * 0.5 + 0.5
        const hatch = diagonal * 0.5 + antiDiagonal * 0.5
        const signal = Math.max(0, Math.min(1, gradient + (hatch - 0.5) * 0.22))
        const level = Math.max(0, Math.min(15, Math.floor(signal * 16)))
        if (level <= BAYER_4X4[y & 3][x & 3]) continue
        ctx.beginPath()
        ctx.arc(x * cellSize + cellSize / 2, y * cellSize + cellSize / 2, dotRadius, 0, Math.PI * 2)
        ctx.fill()
      }
    }

    const url = canvas.toDataURL('image/png')
    cache.set(seed, url)
    return url
  }
</script>

<script lang="ts">
  import { cn } from './utils'

  let { name, class: className }: { name: string; class?: string } = $props()

  const initial = $derived(name.trim().charAt(0).toUpperCase() || '?')
  const src = $derived(pattern(name.trim().toLowerCase()))
</script>

<div
  class={cn('relative flex size-11 items-center justify-center overflow-hidden text-base font-semibold text-white', className)}
  data-testid="guild-icon"
>
  {#if src}
    <img {src} alt="" aria-hidden="true" class="absolute inset-0 size-full" style="image-rendering: pixelated" />
  {:else}
    <div class="absolute inset-0 bg-muted"></div>
  {/if}
  <span class="relative z-10 drop-shadow-(--drop-shadow-extract-1)">{initial}</span>
</div>
