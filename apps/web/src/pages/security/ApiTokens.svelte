<script lang="ts">
  // Coolify's API Tokens page (resources/views/livewire/security/api-tokens.blade.php
  // and app/Livewire/Security/ApiTokens.php, Apache-2.0, see NOTICE): New API
  // token, Copy your token after a create, and Issued tokens, searched and
  // paged in the browser. The Permissions a Role may not grant are disabled,
  // as GET /api/api-tokens/permissions says. Coolify's "API disabled" state is
  // left out: The Bakery has no instance switch for its API.
  import { cubicIn, cubicOut } from 'svelte/easing'
  import { api, ApiError } from '../../lib/api'
  import { ago } from '../../lib/format'
  import Icon from '../../lib/Icon.svelte'
  import type { ApiToken, Permission } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import Checkbox from '../../lib/ui/Checkbox.svelte'
  import ClientPagination from '../../lib/ui/ClientPagination.svelte'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import CopyButton from '../../lib/ui/CopyButton.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import Helper from '../../lib/ui/Helper.svelte'
  import Input from '../../lib/ui/Input.svelte'
  import Select from '../../lib/ui/Select.svelte'
  import SettingsSection from '../../lib/ui/SettingsSection.svelte'
  import Spinner from '../../lib/ui/Spinner.svelte'
  import StatusBadge from '../../lib/ui/StatusBadge.svelte'
  import { toast } from '../../lib/ui/toast.svelte'

  const expirations = [
    { value: '7', label: '7 days' },
    { value: '30', label: '30 days' },
    { value: '60', label: '60 days' },
    { value: '90', label: '90 days' },
    { value: '365', label: '1 year' },
    { value: '', label: 'Never' },
  ]

  // In Coolify's panel order, with its labels and helpers.
  const choices: { permission: Permission; label: string; helper: string }[] = [
    { permission: 'root', label: 'Root', helper: 'Full access to every API operation.' },
    { permission: 'write', label: 'Write', helper: 'Create and update resources.' },
    { permission: 'deploy', label: 'Deploy', helper: 'Trigger deployments through webhooks.' },
    { permission: 'read', label: 'Read', helper: 'Read non-sensitive resource data.' },
    { permission: 'read:sensitive', label: 'Read sensitive data', helper: 'Include secrets, logs, passwords, and Compose content.' },
  ]

  let tokens = $state.raw<ApiToken[] | null>(null)
  let allowed = $state.raw<Partial<Record<Permission, boolean>>>({})
  let loadError = $state('')

  let description = $state('')
  let expiresInDays = $state('30')
  let checks = $state<Record<Permission, boolean>>({ root: false, write: false, deploy: false, read: true, 'read:sensitive': false })
  let errors = $state<Record<string, string>>({})
  let busy = $state(false)
  let permissionsOpen = $state(false)
  let permissionsRoot = $state<HTMLDivElement>()
  // The value of the token just made; the API never shows it again.
  let created = $state.raw<{ id: number; token: string } | null>(null)

  let search = $state('')
  let page = $state(1)
  let pageSize = $state(10)

  // Coolify sorts the Permissions after every change.
  const permissions = $derived(choices.map((c) => c.permission).filter((p) => checks[p]).sort())
  const selectedLabel = $derived(permissions.map((p) => headline(p.replace(':', ' '))).join(', '))

  const filtered = $derived.by(() => {
    const query = search.trim().toLowerCase()
    const list = tokens ?? []
    return query ? list.filter((t) => `${t.name} ${t.permissions.join(' ')}`.toLowerCase().includes(query)) : list
  })
  const totalPages = $derived(Math.max(1, Math.ceil(filtered.length / pageSize)))
  const visible = $derived(filtered.slice((Math.min(page, totalPages) - 1) * pageSize, Math.min(page, totalPages) * pageSize))

  function headline(s: string): string {
    return s
      .split(/\s+/)
      .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
      .join(' ')
  }

  async function load() {
    const [t, p] = await Promise.all([
      api<{ api_tokens: ApiToken[] }>('GET', '/api-tokens'),
      api<{ permissions: { name: Permission; allowed: boolean }[] }>('GET', '/api-tokens/permissions'),
    ])
    // Coolify lists the newest first.
    tokens = [...t.api_tokens].sort((a, b) => b.created_at.localeCompare(a.created_at))
    allowed = Object.fromEntries(p.permissions.map((x) => [x.name, x.allowed]))
  }
  load().catch((e) => (loadError = e.message))

  function setPermissions(list: Permission[]) {
    for (const c of choices) checks[c.permission] = list.includes(c.permission)
  }

  // Coolify's updatedPermissions: root alone, deploy alone, read:sensitive
  // brings read, and nothing left falls back to read.
  function updated(permission: Permission) {
    if (!allowed[permission]) {
      toast.error(`You do not have permission to use ${permission} permissions.`)
      checks[permission] = false
      return
    }
    if (permission === 'root' && checks.root) setPermissions(['root'])
    else if (permission === 'read:sensitive' && !checks.read) checks.read = true
    else if (permission === 'deploy' && checks.deploy) setPermissions(['deploy'])
    else if (permissions.length === 0) setPermissions(['read'])
  }

  async function create(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    errors = {}
    try {
      const r = await api<{ api_token: ApiToken; token: string }>('POST', '/api-tokens', {
        name: description,
        permissions,
        expires_in_days: expiresInDays === '' ? null : Number(expiresInDays),
      })
      created = { id: r.api_token.id, token: r.token }
      description = ''
      permissionsOpen = false
      await load()
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      if (err.errors.name) errors = { name: err.errors.name }
      else toast.error(Object.values(err.errors)[0] ?? err.message)
    } finally {
      busy = false
    }
  }

  async function revoke(t: ApiToken) {
    try {
      await api('DELETE', `/api-tokens/${t.id}`)
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      toast.error(err.message)
      return
    }
    if (created?.id === t.id) created = null
    await load()
  }

  function ymd(at: string): string {
    const d = new Date(at)
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
  }

  const badge: Record<Permission, string> = {
    root: 'border-error/20 bg-error/10 text-error',
    write: 'border-warning/20 bg-warning/10 text-warning',
    deploy: 'border-coollabs/20 bg-coollabs/10 text-coollabs',
    read: 'border-neutral-200 bg-neutral-100 text-neutral-600 dark:border-white/[0.08] dark:bg-white/[0.06] dark:text-fg-dim',
    'read:sensitive': 'border-neutral-200 bg-neutral-100 text-neutral-600 dark:border-white/[0.08] dark:bg-white/[0.06] dark:text-fg-dim',
  }

  // Coolify's x-transition.origin.top: from 0.25rem up, 98 % and transparent.
  function pop(_: Element, { duration, easing }: { duration: number; easing: (t: number) => number }) {
    return {
      duration,
      easing,
      css: (t: number) => `opacity: ${t}; transform: translateY(${(t - 1) * 0.25}rem) scale(${0.98 + 0.02 * t})`,
    }
  }
</script>

<svelte:window
  onclick={(e) => {
    if (permissionsOpen && permissionsRoot && !permissionsRoot.contains(e.target as Node)) permissionsOpen = false
  }}
  onkeydown={(e) => {
    if (e.key === 'Escape') permissionsOpen = false
  }}
/>

{#if loadError}
  <p class="text-sm text-error">{loadError}</p>
{:else if tokens === null}
  <Spinner text="Loading…" />
{:else}
  <div class="application-settings-form flex flex-col gap-6">
    <form onsubmit={create}>
      <SettingsSection id="new-api-token" title="New API token">
        {#snippet actions()}
          <Button type="submit" variant="highlighted" loading={busy} data-testid="create-token">
            <Icon name="plus" class="size-3.5" />
            Create token
          </Button>
        {/snippet}

        <div class="grid gap-4 lg:grid-cols-2">
          <Input
            id="description"
            label="Description"
            placeholder="CI deployment token"
            bind:value={description}
            error={errors.name}
            required
            data-testid="token-description"
          />
          <Select id="expiresInDays" label="Expires in" bind:value={expiresInDays} data-testid="token-expires">
            {#each expirations as option (option.value)}
              <option value={option.value}>{option.label}</option>
            {/each}
          </Select>
        </div>

        <div class="mt-5 border-t border-neutral-200 pt-4 dark:border-white/[0.08]">
          <div class="mb-3 flex items-center gap-2">
            <h4 class="text-[12px] font-semibold text-black dark:text-fg">Permissions</h4>
            <Helper helper="Only grant the abilities this token needs." />
          </div>
          <div class="relative" bind:this={permissionsRoot}>
            <button
              type="button"
              class="listbox-trigger"
              onclick={() => (permissionsOpen = !permissionsOpen)}
              aria-haspopup="listbox"
              aria-expanded={permissionsOpen}
              data-testid="permissions-trigger"
            >
              <span class="truncate">Selected permissions: {selectedLabel}</span>
              <Icon name="chevron-down" class="size-3.5 shrink-0 opacity-60" />
            </button>

            {#if permissionsOpen}
              <div
                class="listbox-panel top-full! mt-1! w-full!"
                role="listbox"
                in:pop={{ duration: 100, easing: cubicOut }}
                out:pop={{ duration: 75, easing: cubicIn }}
              >
                {#each choices as choice (choice.permission)}
                  <div class="listbox-option p-0!" data-testid="permission-{choice.permission.replace(':', '-')}">
                    <Checkbox
                      id="permission-{choice.permission.replace(':', '-')}"
                      label={choice.label}
                      helper={choice.helper}
                      fullWidth
                      bind:checked={checks[choice.permission]}
                      disabled={!allowed[choice.permission] || (choice.permission !== 'root' && checks.root)}
                      instantSave={() => updated(choice.permission)}
                    />
                  </div>
                {/each}
              </div>
            {/if}
          </div>
        </div>
      </SettingsSection>
    </form>

    {#if created}
      <SettingsSection id="copy-your-token" title="Copy your token">
        <div class="flex flex-col gap-3" data-testid="new-token">
          <p class="text-sm text-neutral-500 dark:text-fg-dim">This value will not be shown again after you leave this page.</p>
          <CopyButton text={created.token} testid="new-token-value" />
        </div>
      </SettingsSection>
    {/if}

    <SettingsSection id="issued-tokens" title="Issued tokens" flush>
      {#if tokens.length > 1}
        <div class="border-b border-neutral-200 p-3 dark:border-white/[0.08]">
          <div class="relative max-w-sm">
            <Icon
              name="search"
              class="pointer-events-none absolute top-1/2 left-2.5 z-10 size-3.5 -translate-y-1/2 text-neutral-400 dark:text-fg-faint"
            />
            <input
              bind:value={search}
              oninput={() => (page = 1)}
              type="search"
              placeholder="Search tokens"
              aria-label="Search tokens"
              class="input h-8! w-full rounded-lg! border-neutral-200! bg-white! py-0! pr-8! pl-8! text-[12px]! shadow-none! placeholder:text-neutral-400 focus:border-ring! focus:ring-0! dark:border-white/[0.08]! dark:bg-white/[0.035]! dark:text-fg! dark:placeholder:text-fg-faint"
            />
            {#if search}
              <button
                type="button"
                onclick={() => {
                  search = ''
                  page = 1
                }}
                class="absolute top-1/2 right-2 flex size-5 -translate-y-1/2 items-center justify-center rounded text-neutral-400 transition-colors hover:bg-neutral-100 hover:text-black dark:text-fg-faint dark:hover:bg-white/[0.07] dark:hover:text-fg"
                aria-label="Clear search"
              >
                <Icon name="x" class="size-3" />
              </button>
            {/if}
          </div>
        </div>
      {/if}

      {#if tokens.length === 0}
        <div class="p-4">
          <Empty title="No API tokens" description="Create a token when an external client needs access." icon="keys" size="sm" />
        </div>
      {:else if filtered.length === 0}
        <div class="p-4">
          <Empty size="sm" title="No matching tokens" description="Try a different description or permission." />
        </div>
      {:else}
        <div class="data-table">
          <div class="data-table-header api-tokens-table-grid">
            <span>Description</span>
            <span>Permissions</span>
            <span>Last used</span>
            <span>Created</span>
            <span>Expires</span>
            <span class="text-right">Actions</span>
          </div>
          {#each visible as token (token.id)}
            <div
              class="data-table-row api-tokens-table-grid border-b border-neutral-200 last:border-b-0 dark:border-white/[0.07]"
              data-testid="api-token"
            >
              <div class="min-w-0 truncate text-[12px] font-medium text-black dark:text-fg">{token.name}</div>
              <div class="flex min-w-0 flex-wrap items-center gap-1.5">
                {#each token.permissions as permission (permission)}
                  <span
                    class={['inline-flex min-h-5 items-center rounded-md border px-2 py-0.5 text-[10px] font-medium', badge[permission]]}
                    data-testid="token-permission">{permission}</span
                  >
                {/each}
              </div>
              <div class="text-[11px] text-neutral-500 dark:text-fg-dim">{token.last_used_at ? ago(token.last_used_at) : 'Never'}</div>
              <div class="text-[11px] text-neutral-500 dark:text-fg-dim">{ymd(token.created_at)}</div>
              <div class="text-[11px] text-neutral-500 dark:text-fg-dim" data-testid="token-expires-at">
                {#if !token.expires_at}
                  Never
                {:else if new Date(token.expires_at).getTime() <= Date.now()}
                  <StatusBadge label="Expired" type="error" />
                {:else}
                  {ymd(token.expires_at)}
                {/if}
              </div>
              <div class="flex justify-end">
                <ConfirmationModal
                  title="Confirm API Token Revocation?"
                  actions={['This API token will be permanently revoked.']}
                  confirmationText={token.name}
                  confirmationLabel="Enter the token description to confirm"
                  shortConfirmationLabel="Token description"
                  step2ButtonText="Revoke token"
                  onconfirm={() => revoke(token)}
                >
                  {#snippet trigger(show)}
                    <button
                      type="button"
                      class="inline-flex h-7 items-center rounded-md px-2 text-[11px] font-medium text-error transition-colors hover:bg-error/10"
                      onclick={show}
                      data-testid="revoke-token">Revoke</button
                    >
                  {/snippet}
                </ConfirmationModal>
              </div>
            </div>
          {/each}
        </div>
        <ClientPagination bind:page bind:pageSize total={filtered.length} storageKey="bakery.page-size.api-tokens" />
      {/if}
    </SettingsSection>
  </div>
{/if}
