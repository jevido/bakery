<script lang="ts">
  // Coolify's Server General page (resources/views/livewire/server/show.blade.php
  // for a Remote server, partials/localhost-general.blade.php for the Local
  // one, partials/server-details.blade.php, the "Validate and configure"
  // dialog of server/validate-and-install.blade.php;
  // app/Livewire/Server/Show.php; Apache-2.0, see NOTICE), with only what The
  // Bakery has: no cloud provider, connection timeout, timezone, wildcard
  // domain or build server. The Local server is reached through the local
  // Podman socket, not SSH, so only its Name and Description are editable.
  // Validation checks and never installs anything, so the dialog lists The
  // Bakery's checks instead of Coolify's install steps.
  import { untrack } from 'svelte'
  import { api, ApiError } from '../../lib/api'
  import Icon from '../../lib/Icon.svelte'
  import { session } from '../../lib/session.svelte'
  import type { Server, ServerCheck, ServerDetails } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import CheckpointItem, { type CheckpointStatus } from '../../lib/ui/CheckpointItem.svelte'
  import Input from '../../lib/ui/Input.svelte'
  import Modal from '../../lib/ui/Modal.svelte'
  import SettingsSection from '../../lib/ui/SettingsSection.svelte'
  import Spinner from '../../lib/ui/Spinner.svelte'
  import StatusBadge from '../../lib/ui/StatusBadge.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import UnsavedBar from '../../lib/ui/UnsavedBar.svelte'
  import { isFunctional } from './status'

  let { server, onchange }: { server: Server; onchange: (s: Server) => void } = $props()

  // Only the Instance admin changes the Local server, which every Guild shares.
  const canUpdate = $derived(session.isAdmin && (server.kind === 'remote' || session.instanceAdmin))
  const remote = $derived(server.kind === 'remote')
  const functional = $derived(isFunctional(server))

  // --- Server overview: the details read live from the Server's Podman.
  let details = $state.raw<ServerDetails | null>(null)
  let detailsError = $state('')
  let detailsLoading = $state(false)

  async function loadDetails() {
    detailsLoading = true
    try {
      details = (await api<{ details: ServerDetails }>('GET', `/servers/${server.id}/details`)).details
      detailsError = ''
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      details = null
      detailsError = err.message
    } finally {
      detailsLoading = false
    }
  }

  $effect(() => {
    void server.id
    untrack(loadDetails)
  })

  const when = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' })
  const na = (v: string | number | null | undefined) => (v === null || v === undefined || v === '' || v === 0 ? 'N/A' : String(v))
  const detailRows = $derived(
    details
      ? [
          ['Operating system', na(details.os)],
          ['Architecture', na(details.arch)],
          ['Kernel', na(details.kernel)],
          ['CPU cores', na(details.cpus)],
          ['Memory', details.memory_bytes ? `${(details.memory_bytes / 1073741824).toFixed(1)} GB` : 'N/A'],
          ['Podman version', na(details.podman_version)],
          ['Up since', details.up_since ? when.format(new Date(details.up_since)) : 'N/A'],
        ]
      : [],
  )

  // --- Connection: one form behind the unsaved bar.
  let name = $state('')
  let description = $state('')
  let host = $state('')
  let user = $state('')
  let port = $state<string | number>('')
  let errors = $state<Record<string, string>>({})
  let saving = $state(false)

  function reset() {
    name = server.name
    description = server.description ?? ''
    host = server.host
    user = server.user
    port = String(server.port)
    errors = {}
  }

  $effect(() => {
    void server.id
    untrack(reset)
  })

  const dirty = $derived(
    canUpdate &&
      (name !== server.name ||
        description !== (server.description ?? '') ||
        (remote && (host !== server.host || user !== server.user || String(port) !== String(server.port)))),
  )

  async function save() {
    if (saving || !dirty) return
    saving = true
    errors = {}
    const connection = remote ? { host, user, port: Number(port) || 0 } : { host: '', user: '', port: 0 }
    try {
      const r = await api<{ server: Server }>('PATCH', `/servers/${server.id}`, { name, description, ...connection })
      const forgot = !!server.host_key_fingerprint && !r.server.host_key_fingerprint
      onchange(r.server)
      untrack(reset)
      toast.success('Server updated.', forgot ? 'The host key was forgotten; validate the connection again.' : '')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      errors = Object.keys(err.errors).length ? err.errors : { name: err.message }
    } finally {
      saving = false
    }
  }

  // --- Validate and configure.
  const checkpoints: { name: ServerCheck['name']; title: string; description: string }[] = [
    { name: 'ssh', title: 'Server is reachable', description: 'Verify SSH connectivity and key-based authentication' },
    { name: 'socket', title: 'Podman socket answers', description: 'Reach the rootless Podman API socket' },
    { name: 'podman', title: 'Minimum Podman version', description: 'Require Podman 4.4 or newer' },
    { name: 'linger', title: 'Linger is on', description: "Keep the user's containers running after logout" },
    { name: 'ports', title: 'Ports 80 and 443', description: 'Let the proxy listen on 80 and 443' },
  ]
  const visibleCheckpoints = $derived(checkpoints.filter((c) => remote || c.name !== 'ssh'))
  const checkTitle = (name: string) => checkpoints.find((c) => c.name === name)?.title ?? name

  let dialogOpen = $state(false)
  let asking = $state(false)
  let validating = $state(false)
  let result = $state.raw<ServerCheck[] | null>(null)
  let validateError = $state('')

  const failed = $derived((result ?? []).filter((c) => !c.ok && c.required))
  const complete = $derived(result !== null && failed.length === 0 && !validateError)

  function checkpointStatus(name: string, index: number): CheckpointStatus {
    if (validating) return index === 0 ? 'running' : 'pending'
    const c = result?.find((x) => x.name === name)
    if (!c) return 'pending'
    if (c.ok) return 'success'
    return c.required ? 'error' : 'warning'
  }

  function openValidation() {
    result = null
    validateError = ''
    dialogOpen = true
    if (functional) asking = true
    else startValidating()
  }

  async function startValidating() {
    asking = false
    validating = true
    result = null
    validateError = ''
    try {
      const r = await api<{ server: Server }>('POST', `/servers/${server.id}/validate`)
      result = r.server.validation.checks
      onchange(r.server)
      loadDetails()
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      validateError = err.message
    } finally {
      validating = false
    }
  }

  const mark = (c: ServerCheck) => (c.ok ? '✓' : c.required ? '✗' : '!')
</script>

{#snippet overview()}
  <SettingsSection
    id="server-overview-section"
    title="Server overview"
    helper={remote
      ? 'Connection health, operating system, and hardware details.'
      : 'Operating system and hardware details for the server running The Bakery.'}
  >
    {#snippet actions()}
      <Button class="size-8! px-0!" title="Refresh server details" aria-label="Refresh server details" loading={detailsLoading} onclick={loadDetails}>
        {#if !detailsLoading}<Icon name="refresh" class="size-3.5" />{/if}
      </Button>
      <StatusBadge status={functional ? 'Ready' : 'Validation required'} type={functional ? 'success' : 'warning'} />
    {/snippet}

    <div class="flex items-start gap-3">
      <div
        class="flex size-9 shrink-0 items-center justify-center rounded-lg bg-neutral-100 text-neutral-600 dark:bg-white/[0.06] dark:text-fg-dim"
      >
        <Icon name="servers" class="size-4.5" />
      </div>
      <div class="min-w-0">
        <p class="truncate text-sm font-medium text-neutral-950 dark:text-fg">{remote ? server.name : 'Localhost'}</p>
        <p class="mt-1 text-xs leading-5 text-neutral-500 dark:text-fg-dim">
          {#if functional}
            The server is reachable, validated, and ready to host resources.
          {:else if remote}
            Validate the SSH connection before using this server.
          {:else}
            Validate the local Podman connection before using this server.
          {/if}
        </p>
      </div>
    </div>

    <div class="mt-4 border-t border-neutral-200 pt-4 dark:border-white/[0.08]">
      {#if details}
        <dl class="grid gap-x-6 gap-y-5 sm:grid-cols-2 lg:grid-cols-3" data-testid="server-details">
          {#each detailRows as [label, value] (label)}
            <div>
              <dt class="text-xs font-medium text-neutral-500 dark:text-fg-dim">{label}</dt>
              <dd class="ms-0 mt-1 text-sm font-medium break-words text-neutral-950 dark:text-fg">{value}</dd>
            </div>
          {/each}
        </dl>
      {:else if detailsError}
        <p class="text-sm text-error" data-testid="server-details-error">{detailsError}</p>
      {:else}
        <Spinner text="Reading server details…" />
      {/if}
    </div>
  </SettingsSection>
{/snippet}

{#snippet previousOutput()}
  {#if server.validation.checks.length > 0}
    <SettingsSection id="server-validation-output-section" title="Previous validation output" helper="The latest output produced while checking this server.">
      <div class="max-h-72 overflow-auto rounded-lg bg-neutral-950 p-4 font-mono text-xs leading-5 text-neutral-300" data-testid="checks">
        {#each server.validation.checks as c (c.name)}
          <div class={c.ok ? '' : c.required ? 'text-red-400' : 'text-warning'}>
            {mark(c)} {checkTitle(c.name)}: {c.detail}
          </div>
        {/each}
        {#if server.validation.checked_at}
          <div class="mt-2 text-neutral-500">Checked {when.format(new Date(server.validation.checked_at))}.</div>
        {/if}
      </div>
    </SettingsSection>
  {/if}
{/snippet}

<form
  class="chrome application-settings-form flex flex-col gap-6"
  onsubmit={(e) => {
    e.preventDefault()
    save()
  }}
>
  {#if canUpdate}
    <UnsavedBar {dirty} {saving} onsave={save} onreset={reset} />
  {/if}

  {@render overview()}
  {#if !remote}{@render previousOutput()}{/if}

  <SettingsSection
    id="server-connection-section"
    title="Connection"
    helper={remote
      ? 'Configure how The Bakery identifies, reaches, and validates this server.'
      : 'Configure how The Bakery identifies and connects to this server.'}
  >
    {#snippet actions()}
      {#if canUpdate}
        <Button variant={!remote || functional ? 'default' : 'highlighted'} onclick={openValidation} data-testid="validate">
          <Icon name={functional ? 'refresh' : 'alert-circle'} class="size-3.5" />
          {remote && functional ? 'Revalidate connection' : 'Validate connection'}
        </Button>
      {/if}
    {/snippet}

    <div class="grid gap-4 sm:grid-cols-2">
      <Input label="Name" bind:value={name} error={errors.name} required disabled={!canUpdate || validating} />
      <Input label="Description" bind:value={description} error={errors.description} disabled={!canUpdate || validating} />
    </div>

    {#if remote}
      <div class="mt-4 grid gap-4 lg:grid-cols-3">
        <Input
          type="password"
          label="IP address or domain"
          helper="Enter a hostname or IP address without http:// or https://."
          bind:value={host}
          error={errors.host}
          required
          disabled={!canUpdate || validating}
        />
        <Input label="SSH user" bind:value={user} error={errors.user} required disabled={!canUpdate || validating} />
        <Input type="number" label="SSH port" bind:value={port} error={errors.port} required disabled={!canUpdate || validating} />
      </div>
      <p class="mt-3 text-xs text-neutral-500 dark:text-fg-dim">A new host, port or user forgets the host key; validate the connection again afterwards.</p>
    {/if}
  </SettingsSection>

  {#if remote}{@render previousOutput()}{/if}
</form>

<Modal title="Validate and configure" bind:open={dialogOpen} variant="none" closeOutside={!validating}>
  <div class="flex flex-col gap-4" data-testid="validate-dialog">
    {#if asking}
      <div
        class="rounded-[10px] border border-neutral-200 bg-neutral-50 px-4 py-3 text-[13px] leading-5 text-neutral-600 dark:border-white/[0.08] dark:bg-white/[0.05] dark:text-fg-dim"
      >
        This will revalidate the server: {remote ? 'the SSH connection, ' : ''}the Podman socket and version, linger and the
        ports for the proxy. Nothing is installed or restarted.
      </div>
      <Button variant="highlighted" onclick={startValidating}>Continue</Button>
    {:else}
      <div class="overflow-hidden rounded-[10px] border border-neutral-200 dark:border-white/[0.08]">
        <div class="border-b border-neutral-200 px-4 py-2.5 dark:border-white/[0.08]">
          <h3 class="text-[13px] font-medium text-neutral-600 dark:text-fg-dim">Validation checkpoints</h3>
        </div>
        <div class="divide-y divide-neutral-200 dark:divide-white/[0.07]">
          {#each visibleCheckpoints as c, i (c.name)}
            {@const status = checkpointStatus(c.name, i)}
            {@const check = result?.find((x) => x.name === c.name)}
            {@const detail = check?.detail}
            <CheckpointItem
              title={c.title}
              description={check && !check.required ? `${c.description} (not required)` : c.description}
              {status}
              data-checkpoint-status={status}
            >
              {#if detail && status !== 'success'}{detail}{/if}
            </CheckpointItem>
          {/each}
        </div>
      </div>

      {#if complete}
        <div class="flex items-center justify-between gap-3 rounded-[10px] border border-emerald-500/20 bg-emerald-500/[0.06] px-4 py-3">
          <div class="flex items-center gap-2 text-[13px] font-medium text-emerald-700 dark:text-emerald-300">
            <Icon name="check-circle" class="size-4 shrink-0" />
            Validation complete
          </div>
          <Button onclick={() => (dialogOpen = false)}>Close</Button>
        </div>
      {:else if !validating && (failed.length > 0 || validateError)}
        <div
          class="rounded-[10px] border border-red-500/20 bg-red-500/[0.06] px-4 py-3 text-[13px] leading-5 text-red-700 dark:text-red-300"
          data-testid="validation-failed"
        >
          <div class="mb-1 flex items-center gap-2 text-[12px] font-semibold tracking-[0.06em] uppercase">
            <Icon name="alert-circle" class="size-3.5 shrink-0" />
            Validation failed
          </div>
          <div class="font-mono text-[12px] leading-5 break-words whitespace-pre-line">
            {validateError || failed.map((c) => `${checkTitle(c.name)}: ${c.detail}`).join('\n')}
          </div>
        </div>
        <div>
          <Button onclick={startValidating}>
            <Icon name="refresh" class="size-3.5" />
            Retry validation
          </Button>
        </div>
      {/if}
    {/if}
  </div>
</Modal>
