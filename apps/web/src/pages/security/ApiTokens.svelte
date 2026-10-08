<script lang="ts">
  // Coolify's API Tokens page (resources/views/livewire/security/api-tokens.blade.php
  // and app/Livewire/Security/ApiTokens.php, Apache-2.0, see NOTICE) in
  // Paperclip's settings look (ui/src/pages/CompanySettings.tsx, MIT, see
  // NOTICE): New API token, Copy your token after a create, and Issued
  // tokens, searched and paged in the browser. The Permissions picker is
  // drawn as phase 31 task 01's EventMultiselect draws its panel
  // (src/pages/notifications/EventMultiselect.svelte), with each option's
  // helper and disabled state kept. The Permissions a Role may not grant are
  // disabled, as GET /api/api-tokens/permissions says. Coolify's "API
  // disabled" state is left out: The Bakery has no instance switch for its API.
  import { ChevronsUpDown, Plus } from '@lucide/svelte'
  import * as Popover from '@bakery/ui/components/ui/popover'
  import { api, ApiError } from '../../lib/api'
  import CollectionToolbar from '../../lib/CollectionToolbar.svelte'
  import { ago } from '../../lib/format'
  import SearchField from '../../lib/SearchField.svelte'
  import SettingsGroup from '../../lib/settings/SettingsGroup.svelte'
  import { statusBadgeClasses } from '@bakery/ui/statusColors'
  import type { ApiToken, Permission } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import Checkbox from '../../lib/ui/Checkbox.svelte'
  import ClientPagination from '../../lib/ui/ClientPagination.svelte'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import CopyButton from '../../lib/ui/CopyButton.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import Input from '../../lib/ui/Input.svelte'
  import Select from '../../lib/ui/Select.svelte'
  import Spinner from '../../lib/ui/Spinner.svelte'
  import StatusBadge from '@bakery/ui/StatusBadge.svelte'
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
  // The value of the token just made; the API never shows it again.
  let created = $state.raw<{ id: number; token: string } | null>(null)

  let searchText = $state('')
  let page = $state(1)
  let pageSize = $state(10)

  // Coolify sorts the Permissions after every change.
  const permissions = $derived(choices.map((c) => c.permission).filter((p) => checks[p]).sort())
  const selectedLabel = $derived(permissions.map((p) => headline(p.replace(':', ' '))).join(', '))

  const filtered = $derived.by(() => {
    const query = searchText.trim().toLowerCase()
    const list = tokens ?? []
    return query ? list.filter((t) => `${t.name} ${t.permissions.join(' ')}`.toLowerCase().includes(query)) : list
  })
  const visible = $derived(filtered.slice((page - 1) * pageSize, page * pageSize))

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
      toast.error(err instanceof Error ? err.message : String(err))
      throw err
    }
    if (created?.id === t.id) created = null
    await load()
  }

  function ymd(at: string): string {
    const d = new Date(at)
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
  }

  const badge: Record<Permission, string> = {
    root: statusBadgeClasses.error,
    write: statusBadgeClasses.warning,
    deploy: 'bg-primary/10 text-primary',
    read: statusBadgeClasses.neutral,
    'read:sensitive': statusBadgeClasses.neutral,
  }

  const pageSizeKey = 'bakery.page-size.api-tokens'
</script>

{#if loadError}
  <p class="text-sm text-destructive">{loadError}</p>
{:else if tokens === null}
  <Spinner text="Loading…" />
{:else}
  <div class="flex flex-col gap-8">
    <form onsubmit={create}>
      <SettingsGroup id="new-api-token" label="New API token">
        {#snippet actions()}
          <Button type="submit" variant="highlighted" loading={busy} data-testid="create-token">
            <Plus class="size-3.5" />
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

        <div class="w-full min-w-0">
          <label for="permissions-trigger" class="mb-1.5 block text-sm font-medium text-foreground">Permissions</label>
          <Popover.Root bind:open={permissionsOpen}>
            <Popover.Trigger
              id="permissions-trigger"
              class="flex h-9 w-full min-w-0 items-center gap-2 rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-xs outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
              aria-haspopup="listbox"
              title={selectedLabel}
              data-testid="permissions-trigger"
            >
              <span class="min-w-0 flex-1 truncate text-left">Selected permissions: {selectedLabel}</span>
              <ChevronsUpDown class="size-3.5 shrink-0 opacity-50" />
            </Popover.Trigger>
            <Popover.Content align="start" class="w-80 p-1">
              <div class="max-h-80 overflow-y-auto" role="listbox" aria-multiselectable="true">
                {#each choices as choice (choice.permission)}
                  <div data-testid="permission-{choice.permission.replace(':', '-')}">
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
            </Popover.Content>
          </Popover.Root>
        </div>
      </SettingsGroup>
    </form>

    {#if created}
      <SettingsGroup id="copy-your-token" label="Copy your token">
        <div class="flex flex-col gap-3" data-testid="new-token">
          <p class="text-sm text-muted-foreground">This value will not be shown again after you leave this page.</p>
          <CopyButton text={created.token} testid="new-token-value" />
        </div>
      </SettingsGroup>
    {/if}

    <SettingsGroup id="issued-tokens" label="Issued tokens">
      {#if tokens.length === 0}
        <div class="rounded-md border border-border p-4">
          <Empty title="No API tokens" description="Create a token when an external client needs access." icon="keys" size="sm" />
        </div>
      {:else}
        <CollectionToolbar ariaLabel="Issued tokens controls">
          {#snippet search()}
            <SearchField bind:value={searchText} label="Search tokens" oninput={() => (page = 1)} />
          {/snippet}
          {#snippet controls()}
            <span class="text-xs text-muted-foreground tabular-nums">
              {filtered.length}
              {filtered.length === 1 ? 'token' : 'tokens'}
            </span>
          {/snippet}
        </CollectionToolbar>

        <div class="overflow-hidden rounded-md border border-border">
          {#if filtered.length === 0}
            <div class="p-4">
              <Empty size="sm" title="No matching tokens" description="Try a different description or permission." icon="search" />
            </div>
          {:else}
            {#each visible as token (token.id)}
              <div class="flex flex-wrap items-center gap-x-3 gap-y-1.5 border-b border-border px-4 py-2.5 last:border-b-0" data-testid="api-token">
                <span class="min-w-0 shrink-0 basis-40 truncate text-sm font-medium text-foreground">{token.name}</span>
                <div class="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">
                  {#each token.permissions as permission (permission)}
                    <span
                      class={['inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium', badge[permission]]}
                      data-testid="token-permission">{permission}</span
                    >
                  {/each}
                </div>
                <span class="shrink-0 text-xs text-muted-foreground">{token.last_used_at ? ago(token.last_used_at) : 'Never'}</span>
                <span class="shrink-0 text-xs text-muted-foreground" data-testid="token-expires-at">
                  {#if !token.expires_at}
                    Never
                  {:else if new Date(token.expires_at).getTime() <= Date.now()}
                    <StatusBadge label="Expired" type="error" />
                  {:else}
                    {ymd(token.expires_at)}
                  {/if}
                </span>
                <div class="ml-auto shrink-0">
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
                        class="inline-flex h-7 items-center rounded-md px-2 text-xs font-medium text-destructive transition-colors hover:bg-destructive/10"
                        onclick={show}
                        data-testid="revoke-token">Revoke</button
                      >
                    {/snippet}
                  </ConfirmationModal>
                </div>
              </div>
            {/each}
            <ClientPagination bind:page bind:pageSize total={filtered.length} storageKey={pageSizeKey} />
          {/if}
        </div>
      {/if}
    </SettingsGroup>
  </div>
{/if}
