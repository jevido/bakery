<script lang="ts">
  // Coolify's Deployment page (resources/views/livewire/project/application/deployment/show.blade.php
  // and deployment-navbar.blade.php, Apache-2.0, see NOTICE): the history
  // embedded above, then the logs viewer with its toolbar (timestamps,
  // follow, fullscreen, copy, download, status, Cancel deployment, find in
  // logs). Coolify polls the log every two seconds; here it streams from
  // `GET /api/deployments/{id}/log`, which ends with the Deployment.
  import type { Attachment } from 'svelte/attachments'
  import * as DropdownMenu from '$lib/components/ui/dropdown-menu'
  import { cn } from '$lib/utils'
  import { api, ApiError } from '../../lib/api'
  import SearchField from '../../lib/SearchField.svelte'
  import { projectAccess } from '../../lib/projectAccess.svelte'
  import type { Application, Deployment, LogLine } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import StatusBadge from '../../lib/ui/StatusBadge.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import DeploymentHistory, { deploymentStatus } from './DeploymentHistory.svelte'
  import LogButton from './LogButton.svelte'

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
  const when = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'medium' })
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
</script>

<svelte:window onkeydown={(e) => fullscreen && e.key === 'Escape' && (fullscreen = false)} />

<div class="chrome flex min-h-0 min-w-0 flex-col gap-4 xl:pt-px">
  <DeploymentHistory {application} {serverNames} selected={deploymentId} embedded {reload} />

  {#if missing}
    <Empty title="Deployment not found" description="This Deployment does not exist or was removed with its Application." icon="layers" />
  {:else}
    {#if deployment}
      <div class="flex flex-wrap items-center gap-x-4 gap-y-1.5 text-xs text-muted-foreground">
        {#if status}<StatusBadge status={status.label} type={status.type} />{/if}
        {#if deployment.commit_sha}
          <span>Commit <code class="font-mono text-foreground">{deployment.commit_sha.slice(0, 7)}</code></span>
        {:else if deployment.source_image}
          <span>Image <code class="font-mono text-foreground">{deployment.source_image.split('/').pop()}</code></span>
        {/if}
        <span>Started {deployment.started_at ? when.format(new Date(deployment.started_at)) : '-'}</span>
        <span>Finished {deployment.finished_at ? when.format(new Date(deployment.finished_at)) : '-'}</span>
        <span>Server {serverNames[deployment.server_id] ?? `Server #${deployment.server_id}`}</span>
      </div>
    {/if}
    <div class="flex h-[calc(100dvh-8rem)] min-h-[32rem] w-full flex-col overflow-hidden xl:h-[32rem] xl:min-h-0 xl:flex-none">
      <div
        class={cn(
          'flex min-h-0 w-full flex-col overflow-hidden',
          fullscreen ? 'fixed inset-0 z-60 bg-background' : 'mt-2 flex-1 rounded-lg border bg-card lg:mt-0',
        )}
      >
        <div class="flex shrink-0 flex-wrap items-center gap-x-3 gap-y-1.5 border-b px-2 py-1.5">
          <div class="flex min-w-0 flex-wrap items-center gap-2">
            <div class="flex flex-wrap items-center gap-0.5">
              <LogButton
                label="Toggle Timestamps"
                icon="clock"
                active={showTimestamps}
                aria-pressed={showTimestamps}
                onclick={() => (showTimestamps = !showTimestamps)}
              />
              <LogButton label="Follow Logs" icon="follow" active={follow} aria-pressed={follow} onclick={toggleFollow} />
              <LogButton label={fullscreen ? 'Minimize' : 'Fullscreen'} icon={fullscreen ? 'minimize' : 'maximize'} onclick={() => (fullscreen = !fullscreen)} />
              <LogButton label="Copy Logs" icon="copy" onclick={copyLogs} />
              <DropdownMenu.Root>
                <DropdownMenu.Trigger>
                  {#snippet child({ props })}
                    <LogButton {...props} label="Download Logs" icon="download" />
                  {/snippet}
                </DropdownMenu.Trigger>
                <DropdownMenu.Content align="start" class="w-52">
                  <DropdownMenu.Item onSelect={() => download(visible, '')}>Download displayed logs</DropdownMenu.Item>
                  <DropdownMenu.Item onSelect={() => download(lines, '-all-logs')}>Download all logs</DropdownMenu.Item>
                </DropdownMenu.Content>
              </DropdownMenu.Root>
            </div>
            {#if status}
              <StatusBadge status={status.label} type={status.type} />
            {/if}
          </div>
          <div class="flex w-full min-w-0 flex-wrap items-center justify-end gap-2 sm:ml-auto sm:w-auto sm:flex-nowrap">
            <!-- deployment-navbar.blade.php: Cancel deployment while it is queued or running. -->
            {#if running && projectAccess.can('deploy')}
              <Button variant="error" class="h-8 px-3 text-xs" loading={cancelling} onclick={cancel}>Cancel deployment</Button>
            {/if}
            {#if search}<span class="text-xs whitespace-nowrap text-muted-foreground">{visible.length} matches</span>{/if}
            <div class="min-w-0 flex-1 sm:w-56 sm:flex-none">
              <SearchField
                bind:value={searchInput}
                label="Find in logs"
                oninput={() => {
                  if (!searchInput) search = ''
                }}
              />
            </div>
          </div>
        </div>
        <!-- Focusable so the keyboard can scroll it, as Coolify's tabindex="0". -->
        <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
        <div
          class={cn(
            'flex min-h-40 min-w-0 flex-1 flex-col overflow-x-hidden overflow-y-auto bg-log px-2 pt-2 text-log-foreground after:flex-[0_0_2rem] after:content-[""] sm:px-3',
            !fullscreen && 'rounded-b-lg',
          )}
          tabindex="0"
          role="log"
          aria-live="off"
          onwheel={(e) => e.deltaY < 0 && (follow = false)}
          {@attach autoscroll}
        >
          <div class="flex min-w-0 flex-col font-mono text-[11px] leading-relaxed sm:text-xs" data-testid="deployment-log">
            {#if search && visible.length === 0}
              <div class="py-2 text-muted-foreground">No matches found.</div>
            {/if}
            {#each visible as l (l.id)}
              <div class="flex min-w-0 flex-col gap-0.5 py-0.5 sm:flex-row sm:gap-2 sm:py-0" data-log-line>
                {#if showTimestamps}<span class="shrink-0 text-[10px] text-muted-foreground sm:text-xs">{timestamp(l.at)}</span>{/if}
                <span
                  class={cn('min-w-0 whitespace-pre-wrap [overflow-wrap:anywhere]', l.stream === 'err' && 'text-destructive', l.stream === 'info' && 'font-bold')}
                  >{l.line.trim()}</span
                >
              </div>
            {:else}
              {#if !search}
                <span class="mb-2 text-muted-foreground">{running && !connected ? 'Connecting…' : 'No logs yet.'}</span>
              {/if}
            {/each}
          </div>
        </div>
      </div>
    </div>
  {/if}
</div>
