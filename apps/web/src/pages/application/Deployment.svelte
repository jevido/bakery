<script lang="ts">
  // Coolify's Deployment page (resources/views/livewire/project/application/deployment/show.blade.php
  // and deployment-navbar.blade.php, Apache-2.0, see NOTICE): the history
  // embedded above, then the logs viewer with its toolbar (timestamps,
  // follow, fullscreen, copy, download, status, Cancel deployment, find in
  // logs). Coolify polls the log every two seconds; here it streams from
  // `GET /api/deployments/{id}/log`, which ends with the Deployment.
  import type { Attachment } from 'svelte/attachments'
  import { api, ApiError } from '../../lib/api'
  import Icon from '../../lib/Icon.svelte'
  import { projectAccess } from '../../lib/projectAccess.svelte'
  import type { Application, Deployment, LogLine } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import StatusBadge from '../../lib/ui/StatusBadge.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import DeploymentHistory, { deploymentStatus } from './DeploymentHistory.svelte'

  let {
    application,
    deploymentId,
    serverNames = {},
    onchange,
  }: {
    application: Application
    deploymentId: number
    serverNames?: Record<number, string>
    /** Called when the Deployment ends or is cancelled. */
    onchange?: () => void
  } = $props()

  type Line = LogLine & { id: number; at: string }

  let deployment = $state.raw<Deployment | null>(null)
  let missing = $state(false)
  let lines = $state.raw<Line[]>([])
  let connected = $state(false)
  let reload = $state(0)

  let showTimestamps = $state(true)
  let follow = $state(true)
  let fullscreen = $state(false)
  let searchInput = $state('')
  let search = $state('')
  let downloadOpen = $state(false)
  let cancelling = $state(false)

  // A long build keeps its tail; the head is still in the database.
  const MAX_LINES = 20000

  $effect(() => {
    const id = deploymentId
    deployment = null
    missing = false
    lines = []
    follow = true
    api<{ deployment: Deployment }>('GET', `/deployments/${id}`)
      .then((r) => (deployment = r.deployment))
      .catch((err) => {
        if (err instanceof ApiError && err.status === 404) missing = true
      })
  })

  // The log as server-sent events: `line`, `status` when the status changes
  // and `end` once it is final, after which EventSource must not reconnect.
  $effect(() => {
    if (missing) return
    const es = new EventSource(`/api/deployments/${deploymentId}/log`)
    let pending: Line[] = []
    let frame = 0
    const flush = () => {
      frame = 0
      const next = lines.concat(pending)
      pending = []
      lines = next.length > MAX_LINES ? next.slice(next.length - MAX_LINES) : next
    }
    es.onopen = () => (connected = true)
    es.onerror = () => (connected = false)
    es.addEventListener('line', (e) => {
      pending.push(JSON.parse(e.data))
      frame ||= requestAnimationFrame(flush)
    })
    es.addEventListener('status', (e) => {
      const was = deployment?.status
      deployment = JSON.parse(e.data)
      if (was && was !== deployment?.status) reload++
    })
    es.addEventListener('end', () => {
      es.close()
      connected = false
      if (frame) cancelAnimationFrame(frame)
      flush()
      reload++
      onchange?.()
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

  const pad = (n: number) => String(n).padStart(2, '0')
  function timestamp(at: string): string {
    const d = new Date(at)
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
  }

  const visible = $derived(search ? lines.filter((l) => (timestamp(l.at) + ' ' + l.line).toLowerCase().includes(search)) : lines)
  const status = $derived(deployment ? deploymentStatus(deployment) : null)
  const running = $derived(deployment?.active ?? false)

  function text(of: Line[]): string {
    return of.map((l) => (showTimestamps ? `${timestamp(l.at)} ${l.line}` : l.line).trim()).join('\n') + (of.length ? '\n' : '')
  }

  async function copyLogs() {
    if (visible.length === 0) return
    if (!navigator.clipboard?.writeText) {
      toast.error('Clipboard is not available. Please use HTTPS or localhost.')
      return
    }
    try {
      await navigator.clipboard.writeText(text(visible))
      toast.success('Logs copied to clipboard.')
    } catch {
      toast.error('Failed to copy logs to clipboard.')
    }
  }

  function download(of: Line[], suffix: string) {
    downloadOpen = false
    if (of.length === 0) return
    const url = URL.createObjectURL(new Blob([text(of)], { type: 'text/plain' }))
    const a = document.createElement('a')
    a.href = url
    a.download = `deployment-${deploymentId}${suffix}-${new Date().toISOString().slice(0, 19).replace(/[T:]/g, '-')}.txt`
    a.click()
    URL.revokeObjectURL(url)
  }

  async function cancel() {
    cancelling = true
    try {
      const r = await api<{ deployment: Deployment }>('POST', `/deployments/${deploymentId}/cancel`)
      deployment = r.deployment
      toast.success('Deployment cancelled.')
      reload++
      onchange?.()
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      toast.error('Deployment not cancelled', err.message)
    } finally {
      cancelling = false
    }
  }

  // Follows the bottom while `follow` is on; scrolling up turns it off and
  // scrolling back to the bottom turns it on again, as Coolify's does.
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
        follow = distance <= 10
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
    if (follow && viewport) viewport.scrollTop = viewport.scrollHeight
  }

  const outside: Attachment<HTMLElement> = (el) => {
    const onclick = (e: MouseEvent) => {
      if (!el.contains(e.target as Node)) downloadOpen = false
    }
    document.addEventListener('click', onclick)
    return () => document.removeEventListener('click', onclick)
  }
</script>

<svelte:window onkeydown={(e) => fullscreen && e.key === 'Escape' && (fullscreen = false)} />

<div class="chrome flex min-h-0 min-w-0 flex-col gap-4 xl:pt-px">
  <DeploymentHistory {application} {serverNames} selected={deploymentId} embedded {reload} />

  {#if missing}
    <Empty title="Deployment not found" description="This Deployment does not exist or was removed with its Application." icon="layers" />
  {:else}
    <div class="flex h-[calc(100dvh-8rem)] min-h-[32rem] w-full flex-col overflow-hidden xl:h-[32rem] xl:min-h-0 xl:flex-none">
      <div class={fullscreen ? 'logs-fullscreen flex flex-col' : 'mt-2 flex min-h-0 flex-1 flex-col overflow-hidden lg:mt-0'}>
        <div
          class={[
            'logs-viewer flex min-h-0 w-full flex-col overflow-hidden',
            fullscreen ? 'h-full' : 'flex-1 rounded-xl border border-neutral-200 shadow-sm dark:border-coolgray-200',
          ]}
        >
          <div class="logs-viewer-toolbar">
            <div class="logs-viewer-toolbar-controls">
              <div class="logs-viewer-primary">
                <div class="logs-viewer-actions">
                  <button
                    type="button"
                    title="Toggle Timestamps"
                    aria-pressed={showTimestamps}
                    class={['logs-viewer-btn', showTimestamps && 'logs-viewer-btn-active']}
                    onclick={() => (showTimestamps = !showTimestamps)}
                  >
                    <svg class="size-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
                    </svg>
                  </button>
                  <button
                    type="button"
                    title="Follow Logs"
                    aria-pressed={follow}
                    class={['logs-viewer-btn', follow && 'logs-viewer-btn-active']}
                    onclick={toggleFollow}
                  >
                    <svg class="size-4" viewBox="0 0 24 24" aria-hidden="true">
                      <path fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 5v14m4-4l-4 4m-4-4l4 4" />
                    </svg>
                  </button>
                  <button type="button" title={fullscreen ? 'Minimize' : 'Fullscreen'} class="logs-viewer-btn" onclick={() => (fullscreen = !fullscreen)}>
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
                  <button type="button" title="Copy Logs" class="logs-viewer-btn" onclick={copyLogs}>
                    <svg class="size-4" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" aria-hidden="true">
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        d="M15.75 17.25v3.375c0 .621-.504 1.125-1.125 1.125h-9.75a1.125 1.125 0 0 1-1.125-1.125V7.875c0-.621.504-1.125 1.125-1.125H6.75a9.06 9.06 0 0 1 1.5.124m7.5 10.376h3.375c.621 0 1.125-.504 1.125-1.125V11.25c0-4.46-3.243-8.161-7.5-8.876a9.06 9.06 0 0 0-1.5-.124H9.375c-.621 0-1.125.504-1.125 1.125v3.5m7.5 10.375H9.375a1.125 1.125 0 0 1-1.125-1.125v-9.25m12 6.625v-1.875a3.375 3.375 0 0 0-3.375-3.375h-1.5a1.125 1.125 0 0 1-1.125-1.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H9.75"
                      />
                    </svg>
                  </button>
                  <div class="relative shrink-0" {@attach outside}>
                    <button type="button" title="Download Logs" class="logs-viewer-btn" aria-expanded={downloadOpen} onclick={() => (downloadOpen = !downloadOpen)}>
                      <svg class="size-4" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" aria-hidden="true">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5M16.5 12 12 16.5m0 0L7.5 12m4.5 4.5V3" />
                      </svg>
                    </button>
                    {#if downloadOpen}
                      <div
                        class="absolute right-0 z-50 mt-2 w-max origin-top-right rounded-lg border border-neutral-200 bg-white p-1 shadow-dropdown dark:border-white/[0.1] dark:bg-[#181818]"
                      >
                        <button
                          type="button"
                          class="listbox-option text-neutral-700! hover:bg-neutral-100! dark:text-neutral-200! dark:hover:bg-white/[0.07]!"
                          onclick={() => download(visible, '')}>Download displayed logs</button
                        >
                        <button
                          type="button"
                          class="listbox-option text-neutral-700! hover:bg-neutral-100! dark:text-neutral-200! dark:hover:bg-white/[0.07]!"
                          onclick={() => download(lines, '-all-logs')}>Download all logs</button
                        >
                      </div>
                    {/if}
                  </div>
                </div>
                {#if status}
                  <StatusBadge status={status.label} type={status.type} class="logs-viewer-status-badge" />
                {/if}
              </div>
              <div class="logs-viewer-end">
                <!-- deployment-navbar.blade.php: Cancel deployment while it is queued or running. -->
                {#if running && projectAccess.can('deploy')}
                  <div class="logs-viewer-deployment-actions">
                    <div class="flex flex-wrap items-center justify-end gap-2">
                      <Button variant="error" class="logs-viewer-deployment-btn logs-viewer-cancel-btn" loading={cancelling} onclick={cancel}
                        >Cancel deployment</Button
                      >
                    </div>
                  </div>
                {/if}
                <div class="logs-viewer-search relative">
                  <Icon
                    name="search"
                    class="pointer-events-none absolute top-1/2 left-2.5 z-10 size-3.5 -translate-y-1/2 text-neutral-400 dark:text-neutral-500"
                  />
                  <input
                    type="search"
                    bind:value={searchInput}
                    placeholder="Find in logs"
                    aria-label="Find in logs"
                    class="h-8! w-full rounded-lg! border-neutral-200! bg-white! py-0! pr-8! pl-8! text-[12px]! text-neutral-800! shadow-none! placeholder:text-neutral-400 focus:border-ring! focus:ring-0! dark:border-white/[0.08]! dark:bg-white/[0.05]! dark:text-white! dark:placeholder:text-neutral-500"
                  />
                  {#if searchInput}
                    <button
                      type="button"
                      class="absolute top-1/2 right-2 z-10 flex size-5 -translate-y-1/2 items-center justify-center rounded text-neutral-400 transition-colors hover:bg-neutral-100 hover:text-neutral-800 dark:text-neutral-500 dark:hover:bg-white/[0.07] dark:hover:text-white"
                      aria-label="Clear search"
                      onclick={() => (searchInput = search = '')}
                    >
                      <Icon name="x" class="size-3" />
                    </button>
                  {/if}
                </div>
                <div class="logs-viewer-meta">
                  {#if search}<span class="text-xs whitespace-nowrap text-neutral-500">{visible.length} matches</span>{/if}
                </div>
              </div>
            </div>
          </div>
          <!-- Focusable so the keyboard can scroll it, as Coolify's tabindex="0". -->
          <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
          <div
            class="logs-viewer-viewport flex min-h-40 flex-1 flex-col overflow-x-hidden overflow-y-auto"
            tabindex="0"
            role="log"
            aria-live="off"
            onwheel={(e) => e.deltaY < 0 && (follow = false)}
            {@attach autoscroll}
          >
            <div class="flex min-w-0 flex-col font-logs text-[11px] leading-relaxed sm:text-xs" data-testid="deployment-log">
              {#if search && visible.length === 0}
                <div class="py-2 text-neutral-500">No matches found.</div>
              {/if}
              {#each visible as l (l.id)}
                <div class="log-line logs-viewer-line" data-log-line>
                  {#if showTimestamps}<span class="logs-viewer-timestamp">{timestamp(l.at)}</span>{/if}
                  <span class={['logs-viewer-line-text', l.stream === 'err' && 'text-red-500', l.stream === 'info' && 'font-bold']}
                    >{l.line.trim()}</span
                  >
                </div>
              {:else}
                {#if !search}
                  <span class="mb-2 font-logs text-neutral-400">{running && !connected ? 'Connecting…' : 'No logs yet.'}</span>
                {/if}
              {/each}
            </div>
          </div>
        </div>
      </div>
    </div>
  {/if}
</div>
