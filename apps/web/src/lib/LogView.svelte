<script lang="ts">
  import type { Attachment } from 'svelte/attachments'
  import type { LogLine } from './types'

  let {
    url,
    empty = 'No output yet.',
    onstatus,
    onend,
  }: {
    /** An SSE endpoint sending `line` events, and `end` when it is done. */
    url: string
    empty?: string
    onstatus?: (data: unknown) => void
    onend?: (data: { status?: string; reason?: string }) => void
  } = $props()

  let lines = $state<LogLine[]>([])
  let ended = $state(false)
  let connected = $state(false)
  // Follow the bottom until the reader scrolls up; scrolling back down
  // resumes following.
  let follow = $state(true)

  const MAX_LINES = 5000

  $effect(() => {
    lines = []
    ended = false
    const es = new EventSource(url)
    es.onopen = () => (connected = true)
    es.onerror = () => (connected = false)
    es.addEventListener('line', (e) => {
      lines.push(JSON.parse(e.data))
      if (lines.length > MAX_LINES) lines.splice(0, lines.length - MAX_LINES)
    })
    es.addEventListener('status', (e) => onstatus?.(JSON.parse(e.data)))
    es.addEventListener('end', (e) => {
      es.close()
      ended = true
      connected = false
      onend?.(JSON.parse(e.data))
    })
    return () => es.close()
  })

  const autoscroll: Attachment<HTMLElement> = (el) => {
    const onscroll = () => (follow = el.scrollHeight - el.scrollTop - el.clientHeight < 24)
    el.addEventListener('scroll', onscroll)
    const observer = new MutationObserver(() => {
      if (follow) el.scrollTop = el.scrollHeight
    })
    observer.observe(el, { childList: true, subtree: true })
    return () => {
      el.removeEventListener('scroll', onscroll)
      observer.disconnect()
    }
  }
</script>

<div class="log mono" {@attach autoscroll} role="log" aria-live="off">
  {#each lines as l, i (i)}
    <div class={['line', l.stream]}>{l.line}</div>
  {:else}
    <div class="muted">{ended ? empty : connected ? 'Waiting for output…' : 'Connecting…'}</div>
  {/each}
</div>

<style>
  .log {
    background: #0b0c0e;
    color: #d6d7db;
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.75rem;
    height: min(60vh, 36rem);
    overflow: auto;
    font-size: 0.8rem;
    line-height: 1.5;
  }
  .line {
    white-space: pre-wrap;
    word-break: break-word;
  }
  .info {
    color: #e0a867;
  }
  .err {
    color: #f19b9b;
  }
</style>
