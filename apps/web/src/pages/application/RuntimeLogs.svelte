<script lang="ts">
  // Coolify's Runtime Logs (resources/views/livewire/project/shared/logs.blade.php
  // and get-logs.blade.php, app/Livewire/Project/Shared/GetLogs.php,
  // Apache-2.0, see NOTICE): the running Container's card with the logs
  // viewer, its Lines field, refresh, stream, timestamps, colors, level
  // filter, follow, fullscreen, copy, download and find in logs. Coolify
  // polls `docker logs` every two seconds while streaming; here the logs URL
  // (`/api/applications/{id}/logs` or `/api/databases/{id}/logs`) sends the
  // last lines and, with `follow=1`, goes on with new ones as server-sent
  // events.
  import { untrack } from 'svelte'
  import type { Attachment } from 'svelte/attachments'
  import Icon from '../../lib/Icon.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import Spinner from '../../lib/ui/Spinner.svelte'
  import TableDropdown from '../../lib/ui/TableDropdown.svelte'
  import { toast } from '../../lib/ui/toast.svelte'

  let {
    url,
    container,
  }: {
    /** The Container logs endpoint, without a query. */
    url: string
    /** The running Container's name; '' when none runs, null while unknown. */
    container: string | null
  } = $props()

  type Level = 'error' | 'warning' | 'debug' | 'info'
  type Line = { id: number; raw: string; at: string; text: string; level: Level }

  // Coolify's MAX_LOG_LINES: "all" is at most this many.
  const MAX_LINES = 50000

  let lines = $state.raw<Line[]>([])
  let loading = $state(false)
  let expanded = $state(true)
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
  let downloadOpen = $state(false)
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
    downloadOpen = false
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
      downloadOpen = false
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

  const outside: Attachment<HTMLElement> = (el) => {
    const onclick = (e: MouseEvent) => {
      if (!el.contains(e.target as Node)) downloadOpen = false
    }
    document.addEventListener('click', onclick)
    return () => document.removeEventListener('click', onclick)
  }

  const levels: { level: Level; label: string; dot: string }[] = [
    { level: 'error', label: 'Error', dot: 'bg-red-500' },
    { level: 'warning', label: 'Warning', dot: 'bg-yellow-500' },
    { level: 'debug', label: 'Debug', dot: 'bg-purple-500' },
    { level: 'info', label: 'Info', dot: 'bg-blue-500' },
  ]
</script>

<svelte:window onkeydown={(e) => fullscreen && e.key === 'Escape' && (fullscreen = false)} />

<div class="chrome mt-4 w-full lg:mt-3">
  {#if container === null}
    <div class="loading-state-card flex min-h-40 w-full items-center justify-center"><Spinner text="Loading containers" /></div>
  {:else if container === ''}
    <Empty size="lg" title="Runtime logs unavailable" description="No containers are running, so there are no runtime logs to show." icon="file-content" />
  {:else}
    <div class="flex flex-col gap-4">
      <div class="runtime-log-shell w-full min-w-0">
        <button type="button" class="runtime-log-trigger w-full" aria-expanded={expanded} onclick={() => (expanded = !expanded)}>
          <svg class={['size-4 transition-transform', expanded && 'rotate-90']} viewBox="0 0 24 24" aria-hidden="true">
            <path fill="currentColor" d="M8.59 16.59L13.17 12 8.59 7.41 10 6l6 6-6 6-1.41-1.41z" />
          </svg>
          <h4>{container}</h4>
          {#if streaming}<Spinner />{/if}
        </button>
        {#if expanded}
          <div
            class={fullscreen ? 'logs-fullscreen flex flex-col overflow-visible! bg-white dark:bg-coolgray-100' : 'relative mx-auto w-full'}
          >
            <div class={['runtime-log-panel', fullscreen && 'h-full w-full']}>
              <div class="runtime-log-toolbar logs-viewer-toolbar">
                <div class="logs-viewer-toolbar-controls">
                  <div class="logs-viewer-actions">
                    <button type="button" title="Refresh Logs" class="runtime-log-icon-button order-8" disabled={streaming} onclick={getLogs}>
                      <Icon name="refresh" class="size-3.5" />
                    </button>
                    <button
                      type="button"
                      title={streaming ? 'Stop Streaming' : 'Stream Logs'}
                      aria-pressed={streaming}
                      class={['runtime-log-icon-button order-9', streaming && 'runtime-log-icon-button-active']}
                      onclick={() => (streaming = !streaming)}
                    >
                      {#if streaming}
                        <svg class="size-4" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M6 4h4v16H6V4zm8 0h4v16h-4V4z" /></svg>
                      {:else}
                        <svg class="size-4" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M8 5v14l11-7L8 5z" /></svg>
                      {/if}
                    </button>
                    <button type="button" title="Copy Logs" class="runtime-log-icon-button order-6" onclick={copyLogs}>
                      <svg class="size-4" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" aria-hidden="true">
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          d="M15.75 17.25v3.375c0 .621-.504 1.125-1.125 1.125h-9.75a1.125 1.125 0 0 1-1.125-1.125V7.875c0-.621.504-1.125 1.125-1.125H6.75a9.06 9.06 0 0 1 1.5.124m7.5 10.376h3.375c.621 0 1.125-.504 1.125-1.125V11.25c0-4.46-3.243-8.161-7.5-8.876a9.06 9.06 0 0 0-1.5-.124H9.375c-.621 0-1.125.504-1.125 1.125v3.5m7.5 10.375H9.375a1.125 1.125 0 0 1-1.125-1.125v-9.25m12 6.625v-1.875a3.375 3.375 0 0 0-3.375-3.375h-1.5a1.125 1.125 0 0 1-1.125-1.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H9.75"
                        />
                      </svg>
                    </button>
                    <div class="relative order-7 shrink-0" {@attach outside}>
                      <button
                        type="button"
                        title="Download Logs"
                        class="runtime-log-icon-button"
                        aria-expanded={downloadOpen}
                        onclick={() => (downloadOpen = !downloadOpen)}
                      >
                        <svg class="size-4" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" aria-hidden="true">
                          <path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5M16.5 12 12 16.5m0 0L7.5 12m4.5 4.5V3" />
                        </svg>
                      </button>
                      {#if downloadOpen}
                        <div
                          class="runtime-log-menu absolute right-0 z-[90] mt-2 w-max min-w-52 origin-top-right rounded-lg border border-neutral-200 p-1 shadow-dropdown dark:border-white/[0.1]"
                        >
                          <button
                            type="button"
                            class="listbox-option text-neutral-700! hover:bg-neutral-100! dark:text-neutral-200! dark:hover:bg-white/[0.07]!"
                            onclick={downloadLogs}>Download displayed logs</button
                          >
                          <button
                            type="button"
                            class={[
                              'listbox-option text-neutral-700! hover:bg-neutral-100! dark:text-neutral-200! dark:hover:bg-white/[0.07]!',
                              downloadingAll && 'cursor-not-allowed opacity-50',
                            ]}
                            disabled={downloadingAll}
                            onclick={downloadAllLogs}
                          >
                            {#if downloadingAll}<Spinner text="Downloading..." />{:else}Download all logs{/if}
                          </button>
                        </div>
                      {/if}
                    </div>
                    <button
                      type="button"
                      title="Toggle Timestamps"
                      aria-pressed={showTimestamps}
                      class={['runtime-log-icon-button order-1', showTimestamps && 'runtime-log-icon-button-active']}
                      onclick={() => (showTimestamps = !showTimestamps)}
                    >
                      <svg class="size-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
                      </svg>
                    </button>
                    <button
                      type="button"
                      title="Toggle Log Colors"
                      aria-pressed={colorLogs}
                      class={['runtime-log-icon-button order-3', colorLogs && 'runtime-log-icon-button-active']}
                      onclick={toggleColorLogs}
                    >
                      <svg class="size-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          d="M9.53 16.122a3 3 0 0 0-5.78 1.128 2.25 2.25 0 0 1-2.4 2.245 4.5 4.5 0 0 0 8.4-2.245c0-.399-.078-.78-.22-1.128Zm0 0a15.998 15.998 0 0 0 3.388-1.62m-5.043-.025a15.994 15.994 0 0 1 1.622-3.395m3.42 3.42a15.995 15.995 0 0 0 4.764-4.648l3.876-5.814a1.151 1.151 0 0 0-1.597-1.597L14.146 6.32a15.996 15.996 0 0 0-4.649 4.763m3.42 3.42a6.776 6.776 0 0 0-3.42-3.42"
                        />
                      </svg>
                    </button>
                    <div class="order-4">
                      <TableDropdown panelClass="runtime-log-menu min-w-40!">
                        {#snippet trigger({ open, toggle })}
                          <button
                            type="button"
                            title="Filter Log Levels"
                            class={['runtime-log-icon-button', Object.values(logFilters).some((v) => !v) && 'runtime-log-icon-button-active']}
                            aria-haspopup="listbox"
                            aria-expanded={open}
                            onclick={toggle}
                          >
                            <svg class="size-4" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" aria-hidden="true">
                              <path
                                stroke-linecap="round"
                                stroke-linejoin="round"
                                d="M12 3c2.755 0 5.455.232 8.083.678.533.09.917.556.917 1.096v1.044a2.25 2.25 0 0 1-.659 1.591l-5.432 5.432a2.25 2.25 0 0 0-.659 1.591v2.927a2.25 2.25 0 0 1-1.244 2.013L9.75 21v-6.568a2.25 2.25 0 0 0-.659-1.591L3.659 7.409A2.25 2.25 0 0 1 3 5.818V4.774c0-.54.384-1.006.917-1.096A48.32 48.32 0 0 1 12 3Z"
                              />
                            </svg>
                          </button>
                        {/snippet}
                        {#snippet children()}
                          {#each levels as l (l.level)}
                            <button type="button" class="listbox-option" onclick={() => toggleLogFilter(l.level)}>
                              <span class={['h-2.5 w-2.5 rounded-full', l.dot]}></span>
                              <span class="flex-1 text-left">{l.label}</span>
                              {#if logFilters[l.level]}<span>✓</span>{/if}
                            </button>
                          {/each}
                        {/snippet}
                      </TableDropdown>
                    </div>
                    <button
                      type="button"
                      title="Follow Logs"
                      aria-pressed={follow}
                      class={['runtime-log-icon-button order-2', follow && 'runtime-log-icon-button-active']}
                      onclick={toggleFollow}
                    >
                      <svg class="size-4" viewBox="0 0 24 24" aria-hidden="true">
                        <path fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 5v14m4-4l-4 4m-4-4l4 4" />
                      </svg>
                    </button>
                    <button
                      type="button"
                      title={fullscreen ? 'Minimize' : 'Fullscreen'}
                      class="runtime-log-icon-button order-5"
                      onclick={() => (fullscreen = !fullscreen)}
                    >
                      {#if fullscreen}
                        <svg class="size-4" viewBox="0 0 24 24" aria-hidden="true">
                          <path fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 14h4m0 0v4m0-4l-6 6m14-10h-4m0 0V6m0 4l6-6" />
                        </svg>
                      {:else}
                        <svg class="size-4" viewBox="0 0 24 24" aria-hidden="true">
                          <path
                            fill="currentColor"
                            d="M9.793 12.793a1 1 0 0 1 1.497 1.32l-.083.094L6.414 19H9a1 1 0 0 1 .117 1.993L9 21H4a1 1 0 0 1-.993-.883L3 20v-5a1 1 0 0 1 1.993-.117L5 15v2.586l4.793-4.793ZM20 3a1 1 0 0 1 .993.883L21 4v5a1 1 0 0 1-1.993.117L19 9V6.414l-4.793 4.793a1 1 0 0 1-1.497-1.32l.083-.094L17.586 5H15a1 1 0 0 1-.117-1.993L15 3h5Z"
                          />
                        </svg>
                      {/if}
                    </button>
                  </div>
                  <div class="logs-viewer-end runtime-logs-viewer-end">
                    <div class="logs-viewer-meta">
                      <form
                        class="logs-viewer-lines"
                        onsubmit={(e) => {
                          e.preventDefault()
                          getLogs()
                        }}
                      >
                        <span class="logs-viewer-lines-label">Lines</span>
                        <input
                          type="number"
                          bind:value={numberOfLines}
                          placeholder="100"
                          min="-1"
                          max="50000"
                          title="Number of lines (max 50,000; use -1 for all)"
                          aria-label="Lines"
                          readonly={streaming}
                          class="input logs-viewer-lines-input"
                        />
                        <button type="button" title="Show all logs" class="runtime-log-icon-button" disabled={streaming} onclick={showAllLogs}>All</button>
                      </form>
                      {#if search}<span class="text-xs whitespace-nowrap text-gray-500 dark:text-gray-400">{visible.length} matches</span>{/if}
                    </div>
                    <div class="logs-viewer-search relative">
                      <Icon
                        name="search"
                        class="pointer-events-none absolute top-1/2 left-2.5 z-10 size-3.5 -translate-y-1/2 text-neutral-400 dark:text-fg-faint"
                      />
                      <input
                        type="search"
                        bind:value={searchInput}
                        placeholder="Find in logs"
                        aria-label="Find in logs"
                        class="h-8! w-full rounded-lg! border-neutral-200! bg-white! py-0! pr-8! pl-8! text-[12px]! shadow-none! placeholder:text-neutral-400 focus:border-accent! focus:ring-0! dark:border-white/[0.08]! dark:bg-white/[0.035]! dark:text-fg! dark:placeholder:text-fg-faint"
                      />
                      {#if searchInput}
                        <button
                          type="button"
                          class="absolute top-1/2 right-2 z-10 flex size-5 -translate-y-1/2 items-center justify-center rounded text-neutral-400 transition-colors hover:bg-neutral-100 hover:text-black dark:text-fg-faint dark:hover:bg-white/[0.07] dark:hover:text-fg"
                          aria-label="Clear search"
                          onclick={() => (searchInput = search = '')}
                        >
                          <Icon name="x" class="size-3" />
                        </button>
                      {/if}
                    </div>
                  </div>
                </div>
              </div>
              <!-- Focusable so the keyboard can scroll it, as Coolify's tabindex="0". -->
              <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
              <div
                class={[
                  'runtime-log-viewport logs-viewer-viewport flex w-full min-w-0 flex-col overflow-x-hidden overflow-y-auto',
                  fullscreen ? 'flex-1' : 'max-h-[min(40rem,70dvh)] sm:max-h-[40rem]',
                ]}
                tabindex="0"
                role="log"
                aria-live="off"
                onwheel={(e) => e.deltaY < 0 && (follow = false)}
                onkeydown={keyScroll}
                {@attach autoscroll}
              >
                {#if lines.length > 0}
                  <div class="max-w-full cursor-default font-logs text-[11px] leading-relaxed sm:text-xs" data-testid="runtime-log">
                    {#if search && visible.length === 0}
                      <div class="py-2 text-gray-500 dark:text-gray-400">No matches found.</div>
                    {/if}
                    {#each visible as l (l.id)}
                      <div class={['log-line logs-viewer-line', colorLogs && `log-${l.level}`]} data-log-line>
                        {#if l.at && showTimestamps}<span class="logs-viewer-timestamp text-gray-500">{l.at}</span>{/if}
                        <span class="logs-viewer-line-text"
                          >{#each parts(l.text) as p, i (i)}{#if p.match}<span class="log-highlight">{p.text}</span>{:else}{p.text}{/if}{/each}</span
                        >
                      </div>
                    {/each}
                  </div>
                {:else}
                  <pre class="max-w-full font-logs break-all whitespace-pre-wrap text-neutral-400">{loading ? 'Loading…' : 'No logs yet.'}</pre>
                {/if}
              </div>
            </div>
          </div>
        {/if}
      </div>
    </div>
  {/if}
</div>
