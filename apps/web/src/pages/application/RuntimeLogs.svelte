<script lang="ts">
  // Coolify's Runtime Logs (resources/views/livewire/project/shared/logs.blade.php
  // and get-logs.blade.php, app/Livewire/Project/Shared/GetLogs.php,
  // Apache-2.0, see NOTICE): the running Container's card with the logs
  // viewer, its Lines field, refresh, stream, timestamps, colors, level
  // filter, follow, fullscreen, copy, download and find in logs. Coolify
  // polls `docker logs` every two seconds while streaming; here the logs URL
  // (`/api/applications/{id}/logs`, `/api/databases/{id}/logs` or
  // `/api/services/{id}/components/{name}/logs`) sends the last lines and,
  // with `follow=1`, goes on with new ones as server-sent events.
  import { untrack } from 'svelte'
  import type { Attachment } from 'svelte/attachments'
  import { Button } from '$lib/components/ui/button'
  import * as DropdownMenu from '$lib/components/ui/dropdown-menu'
  import { Input } from '$lib/components/ui/input'
  import { cn } from '$lib/utils'
  import Icon from '../../lib/Icon.svelte'
  import SearchField from '../../lib/SearchField.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import Spinner from '../../lib/ui/Spinner.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import LogButton from './LogButton.svelte'

  let {
    url,
    container,
    expandByDefault = true,
  }: {
    /** The Container logs endpoint, without a query. */
    url: string
    /** The running Container's name; '' when none runs, null while unknown. */
    container: string | null
    /** Coolify opens the card only when it is the resource's one Container. */
    expandByDefault?: boolean
  } = $props()

  type Level = 'error' | 'warning' | 'debug' | 'info'
  type Line = { id: number; raw: string; at: string; text: string; level: Level }

  // Coolify's MAX_LOG_LINES: "all" is at most this many.
  const MAX_LINES = 50000

  let lines = $state.raw<Line[]>([])
  let loading = $state(false)
  let expanded = $state(untrack(() => expandByDefault))
  let streaming = $state(false)
  let showTimestamps = $state(true)
  let numberOfLines = $state(100)
  let follow = $state(false)
  let followManuallyDisabled = false
  let fullscreen = $state(false)
  let colorLogs = $state(localStorage.getItem('bakery-color-logs') === 'true')
  let logFilters = $state<Record<Level, boolean>>(
    JSON.parse(localStorage.getItem('bakery-log-filters') ?? 'null') ?? { error: true, warning: true, debug: true, info: true },
  )
  let searchInput = $state('')
  let search = $state('')
  let downloadingAll = $state(false)

  let nextId = 0
  const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']
  const pad = (n: number) => String(n).padStart(2, '0')

  // Podman prefixes each line with the time it was written (RFC 3339); it is
  // shown as Coolify's Y-M-d H:i:s, in the browser's time zone.
  function parse(raw: string): Line {
    const m = /^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})?)\s(.*)$/.exec(raw)
    let at = ''
    let text = raw
    if (m) {
      text = m[2]
      const d = new Date(m[1])
      if (!isNaN(d.getTime()))
        at = `${d.getFullYear()}-${MONTHS[d.getMonth()]}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
    }
    return { id: nextId++, raw, at, text, level: levelOf(text.toLowerCase()) }
  }

  function levelOf(content: string): Level {
    if (/\b(error|err|failed|failure|exception|fatal|panic|critical)\b/.test(content)) return 'error'
    if (/\b(warn|warning|wrn|caution)\b/.test(content)) return 'warning'
    if (/\b(debug|dbg|trace|verbose)\b/.test(content)) return 'debug'
    return 'info'
  }

  const tail = (n: number) => (n < 0 || n > MAX_LINES ? MAX_LINES : n || 100)

  /** Reads the last n lines once and hands them over when the stream ends. */
  function read(n: number): Promise<string[]> {
    return new Promise((resolve, reject) => {
      const got: string[] = []
      const es = new EventSource(`${url}?lines=${n}`)
      es.addEventListener('line', (e) => got.push(JSON.parse(e.data).line))
      es.addEventListener('end', () => {
        es.close()
        resolve(got)
      })
      es.onerror = () => {
        es.close()
        reject(new Error('The logs could not be read.'))
      }
    })
  }

  async function getLogs() {
    if (streaming || !container) return
    loading = true
    try {
      lines = (await read(tail(numberOfLines))).map(parse)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    } finally {
      loading = false
    }
  }

  function showAllLogs() {
    numberOfLines = -1
    getLogs()
  }

  // A new Container (a redeploy) is read afresh; only the Container is
  // tracked, not the Lines field getLogs reads.
  $effect(() => {
    if (!container) {
      lines = []
      untrack(() => (streaming = false))
      return
    }
    untrack(getLogs)
  })

  // Streaming keeps Coolify's window of the last Lines: the stream starts
  // with them and drops the oldest as new ones come.
  $effect(() => {
    if (!streaming || !container) return
    const keep = tail(numberOfLines)
    const es = new EventSource(`${url}?lines=${keep}&follow=1`)
    let pending: Line[] = []
    let first = true
    let frame = 0
    const flush = () => {
      frame = 0
      const next = (first ? [] : lines).concat(pending)
      first = false
      pending = []
      lines = next.length > keep ? next.slice(next.length - keep) : next
    }
    es.addEventListener('line', (e) => {
      pending.push(parse(JSON.parse(e.data).line))
      frame ||= requestAnimationFrame(flush)
    })
    es.addEventListener('end', () => {
      es.close()
      if (frame) cancelAnimationFrame(frame)
      flush()
      streaming = false
    })
    return () => {
      es.close()
      if (frame) cancelAnimationFrame(frame)
    }
  })

  $effect(() => {
    const value = searchInput.trim().toLowerCase()
    const t = setTimeout(() => (search = value), 300)
    return () => clearTimeout(t)
  })

  const shown = (l: Line) => logFilters[l.level] !== false && (!search || l.raw.toLowerCase().includes(search))
  const visible = $derived(lines.filter(shown))

  function toggleLogFilter(level: Level) {
    logFilters[level] = !logFilters[level]
    localStorage.setItem('bakery-log-filters', JSON.stringify(logFilters))
  }

  function toggleColorLogs() {
    colorLogs = !colorLogs
    localStorage.setItem('bakery-color-logs', String(colorLogs))
  }

  /** The text split around each match of the search, for highlighting. */
  function parts(text: string): { text: string; match: boolean }[] {
    if (!search) return [{ text, match: false }]
    const out: { text: string; match: boolean }[] = []
    const lower = text.toLowerCase()
    let last = 0
    for (let i = lower.indexOf(search); i !== -1; i = lower.indexOf(search, last)) {
      if (i > last) out.push({ text: text.slice(last, i), match: false })
      out.push({ text: text.slice(i, i + search.length), match: true })
      last = i + search.length
    }
    if (last < text.length) out.push({ text: text.slice(last), match: false })
    return out
  }

  const lineText = (l: Line) => (showTimestamps && l.at ? `${l.at} ${l.text}` : l.text).replace(/\s+/g, ' ').trim()

  async function copyLogs() {
    if (!navigator.clipboard?.writeText) {
      toast.error('Clipboard is not available. Please use HTTPS or localhost.')
      return
    }
    try {
      await navigator.clipboard.writeText(lines.map((l) => (showTimestamps ? l.raw : l.text)).join('\n'))
      toast.success('Logs copied to clipboard.')
    } catch {
      toast.error('Failed to copy logs to clipboard.')
    }
  }

  function save(content: string, suffix: string) {
    const url = URL.createObjectURL(new Blob([content], { type: 'text/plain' }))
    const a = document.createElement('a')
    a.href = url
    a.download = `${container || 'logs'}${suffix}-${new Date().toISOString().slice(0, 19).replace(/[T:]/g, '-')}.txt`
    a.click()
    URL.revokeObjectURL(url)
  }

  function downloadLogs() {
    save(visible.map((l) => lineText(l) + '\n').join(''), '-logs')
  }

  async function downloadAllLogs() {
    downloadingAll = true
    try {
      const all = await read(MAX_LINES)
      if (all.length === 0) return
      save(all.join('\n') + '\n', '-all-logs')
      toast.success('All logs downloaded.')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    } finally {
      downloadingAll = false
    }
  }

  // Follow keeps the bottom in view; scrolling up stops it, and scrolling
  // back to the bottom starts it again unless it was switched off by hand,
  // as Coolify's does.
  let viewport = $state<HTMLElement | null>(null)
  const autoscroll: Attachment<HTMLElement> = (el) => {
    viewport = el
    let programmatic = false
    let debounce = 0
    const onscroll = () => {
      if (programmatic) return
      clearTimeout(debounce)
      debounce = setTimeout(() => {
        const distance = el.scrollHeight - el.scrollTop - el.clientHeight
        if (!follow && !followManuallyDisabled && distance <= 10) follow = true
      }, 150)
    }
    el.addEventListener('scroll', onscroll)
    const observer = new MutationObserver(() => {
      if (!follow) return
      programmatic = true
      el.scrollTop = el.scrollHeight
      requestAnimationFrame(() => (programmatic = false))
    })
    observer.observe(el, { childList: true, subtree: true })
    return () => {
      el.removeEventListener('scroll', onscroll)
      observer.disconnect()
      clearTimeout(debounce)
      viewport = null
    }
  }

  function toggleFollow() {
    follow = !follow
    followManuallyDisabled = !follow
    if (follow && viewport) viewport.scrollTop = viewport.scrollHeight
  }

  function keyScroll(e: KeyboardEvent) {
    if (follow && ['ArrowUp', 'PageUp', 'Home'].includes(e.key)) follow = false
  }

  // Coolify's level colors: a dot in the filter, a tint behind the line.
  const levels: { level: Level; label: string; dot: string; tint: string }[] = [
    { level: 'error', label: 'Error', dot: 'bg-red-500', tint: 'bg-red-500/10 dark:bg-red-500/15' },
    { level: 'warning', label: 'Warning', dot: 'bg-yellow-500', tint: 'bg-yellow-500/10 dark:bg-yellow-500/15' },
    { level: 'debug', label: 'Debug', dot: 'bg-purple-500', tint: 'bg-purple-500/10 dark:bg-purple-500/15' },
    { level: 'info', label: 'Info', dot: 'bg-blue-500', tint: 'bg-blue-500/10 dark:bg-blue-500/15' },
  ]
  const tint = Object.fromEntries(levels.map((l) => [l.level, l.tint])) as Record<Level, string>
</script>

<svelte:window onkeydown={(e) => fullscreen && e.key === 'Escape' && (fullscreen = false)} />

<div class="chrome mt-4 w-full lg:mt-3">
  {#if container === null}
    <div class="flex min-h-40 w-full items-center justify-center rounded-lg border"><Spinner text="Loading containers" /></div>
  {:else if container === ''}
    <Empty size="lg" title="Runtime logs unavailable" description="No containers are running, so there are no runtime logs to show." icon="file-content" />
  {:else}
    <div class="w-full min-w-0 rounded-lg border bg-card">
      <button
        type="button"
        class="flex min-h-11 w-full items-center gap-2 rounded-lg px-3.5 py-2.5 text-left transition-colors hover:bg-accent/50"
        aria-expanded={expanded}
        onclick={() => (expanded = !expanded)}
      >
        <Icon name="chevron-right" class={cn('size-4 shrink-0 text-muted-foreground transition-transform', expanded && 'rotate-90')} />
        <h4 class="min-w-0 truncate font-mono text-[13px] font-medium">{container}</h4>
        {#if streaming}<Spinner />{/if}
      </button>
      {#if expanded}
        <div class={cn('flex min-w-0 flex-col', fullscreen ? 'fixed inset-0 z-60 bg-background' : 'border-t')}>
          <div class="flex flex-wrap items-center gap-x-3 gap-y-1.5 border-b px-2 py-1.5">
            <div class="flex flex-wrap items-center gap-0.5">
              <LogButton
                label="Toggle Timestamps"
                icon="clock"
                active={showTimestamps}
                aria-pressed={showTimestamps}
                onclick={() => (showTimestamps = !showTimestamps)}
              />
              <LogButton label="Follow Logs" icon="follow" active={follow} aria-pressed={follow} onclick={toggleFollow} />
              <LogButton label="Toggle Log Colors" icon="palette" active={colorLogs} aria-pressed={colorLogs} onclick={toggleColorLogs} />
              <DropdownMenu.Root>
                <DropdownMenu.Trigger>
                  {#snippet child({ props })}
                    <LogButton {...props} label="Filter Log Levels" icon="filter" active={Object.values(logFilters).some((v) => !v)} />
                  {/snippet}
                </DropdownMenu.Trigger>
                <DropdownMenu.Content align="start" class="w-40">
                  {#each levels as l (l.level)}
                    <DropdownMenu.CheckboxItem checked={logFilters[l.level]} onCheckedChange={() => toggleLogFilter(l.level)}>
                      <span class={cn('size-2.5 rounded-full', l.dot)}></span>
                      {l.label}
                    </DropdownMenu.CheckboxItem>
                  {/each}
                </DropdownMenu.Content>
              </DropdownMenu.Root>
              <LogButton label={fullscreen ? 'Minimize' : 'Fullscreen'} icon={fullscreen ? 'minimize' : 'maximize'} onclick={() => (fullscreen = !fullscreen)} />
              <LogButton label="Copy Logs" icon="copy" onclick={copyLogs} />
              <DropdownMenu.Root>
                <DropdownMenu.Trigger>
                  {#snippet child({ props })}
                    <LogButton {...props} label="Download Logs" icon="download" />
                  {/snippet}
                </DropdownMenu.Trigger>
                <DropdownMenu.Content align="start" class="w-52">
                  <DropdownMenu.Item onSelect={downloadLogs}>Download displayed logs</DropdownMenu.Item>
                  <DropdownMenu.Item disabled={downloadingAll} onSelect={downloadAllLogs}>
                    {downloadingAll ? 'Downloading...' : 'Download all logs'}
                  </DropdownMenu.Item>
                </DropdownMenu.Content>
              </DropdownMenu.Root>
              <LogButton label="Refresh Logs" icon="refresh" disabled={streaming} onclick={getLogs} />
              <LogButton
                label={streaming ? 'Stop Streaming' : 'Stream Logs'}
                icon={streaming ? 'pause' : 'play'}
                active={streaming}
                aria-pressed={streaming}
                onclick={() => (streaming = !streaming)}
              />
            </div>
            <div class="flex w-full min-w-0 flex-wrap items-center gap-2 sm:ml-auto sm:w-auto sm:flex-nowrap">
              <form
                class="flex shrink-0 items-center gap-1.5"
                onsubmit={(e) => {
                  e.preventDefault()
                  getLogs()
                }}
              >
                <span class="text-xs font-medium text-muted-foreground">Lines</span>
                <Input
                  type="number"
                  bind:value={numberOfLines}
                  placeholder="100"
                  min="-1"
                  max="50000"
                  title="Number of lines (max 50,000; use -1 for all)"
                  aria-label="Lines"
                  readonly={streaming}
                  class="h-8 w-18 px-2 text-center text-xs [appearance:textfield]"
                />
                <Button variant="ghost" size="xs" title="Show all logs" disabled={streaming} onclick={showAllLogs}>All</Button>
              </form>
              {#if search}<span class="text-xs whitespace-nowrap text-muted-foreground">{visible.length} matches</span>{/if}
              <div class="min-w-0 flex-1 sm:w-56 sm:flex-none">
                <SearchField bind:value={searchInput} label="Find in logs" oninput={() => {
                    if (!searchInput) search = ''
                  }} />
              </div>
            </div>
          </div>
          <!-- Focusable so the keyboard can scroll it, as Coolify's tabindex="0". -->
          <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
          <div
            class={cn(
              'flex w-full min-w-0 flex-col overflow-x-hidden overflow-y-auto bg-log px-2 pt-2 text-log-foreground after:flex-[0_0_2rem] after:content-[""] sm:px-3',
              fullscreen ? 'flex-1' : 'max-h-[min(40rem,70dvh)] min-h-48 rounded-b-lg sm:max-h-[40rem] sm:min-h-72',
            )}
            tabindex="0"
            role="log"
            aria-live="off"
            onwheel={(e) => e.deltaY < 0 && (follow = false)}
            onkeydown={keyScroll}
            {@attach autoscroll}
          >
            {#if lines.length > 0}
              <div class="max-w-full cursor-default font-mono text-[11px] leading-relaxed sm:text-xs" data-testid="runtime-log">
                {#if search && visible.length === 0}
                  <div class="py-2 text-muted-foreground">No matches found.</div>
                {/if}
                {#each visible as l (l.id)}
                  <div class={cn('flex min-w-0 flex-col gap-0.5 py-0.5 sm:flex-row sm:gap-2 sm:py-0', colorLogs && tint[l.level])} data-log-line>
                    {#if l.at && showTimestamps}<span class="shrink-0 text-[10px] text-muted-foreground sm:text-xs">{l.at}</span>{/if}
                    <span class="min-w-0 whitespace-pre-wrap [overflow-wrap:anywhere]"
                      >{#each parts(l.text) as p, i (i)}{#if p.match}<mark class="bg-warning/30 text-inherit">{p.text}</mark>{:else}{p.text}{/if}{/each}</span
                    >
                  </div>
                {/each}
              </div>
            {:else}
              <pre class="max-w-full font-mono text-xs break-all whitespace-pre-wrap text-muted-foreground">{loading ? 'Loading…' : 'No logs yet.'}</pre>
            {/if}
          </div>
        </div>
      {/if}
    </div>
  {/if}
</div>
