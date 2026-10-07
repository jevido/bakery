<script lang="ts">
  // Coolify's Environment Variables for a Service
  // (resources/views/livewire/project/shared/environment-variable/all.blade.php
  // and show.blade.php, app/Livewire/Project/Shared/EnvironmentVariable/*.php;
  // Apache-2.0, see NOTICE), in the look of the Application's
  // (lib/EnvironmentVariables.svelte). A Service's variables are the ones its
  // Compose file declares, so there is no Add and no delete, and a variable
  // has no Buildtime, Runtime, Literal or Multiline setting. Generated ones
  // are Managed and locked; a Member's are edited in the row's dialog. Left
  // out: the Developer view.
  import { api, ApiError } from '../../lib/api'
  import Icon from '../../lib/Icon.svelte'
  import { href, servicePath } from '../../lib/router.svelte'
  import { projectAccess } from '../../lib/projectAccess.svelte'
  import type { Service, ServiceVariable } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import ClientPagination from '../../lib/ui/ClientPagination.svelte'
  import CopyButton from '../../lib/ui/CopyButton.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import FieldError from '../../lib/ui/FieldError.svelte'
  import Input from '../../lib/ui/Input.svelte'
  import Modal from '../../lib/ui/Modal.svelte'
  import SettingsSection from '../../lib/ui/SettingsSection.svelte'
  import TableDropdown from '../../lib/ui/TableDropdown.svelte'
  import { toast } from '../../lib/ui/toast.svelte'

  let { service, onchange }: { service: Service; onchange: (s: Service) => void } = $props()

  const hiddenValue = 'Hidden (viewers cannot see it)'
  const pageSizeKey = 'bakery.page-size.environment-variables'

  const vars = $derived(service.variables ?? [])

  let search = $state('')
  // Coolify's variable filters plus one per Component ("c:<name>", so a
  // Component named like a filter stays apart).
  let filters = $state<string[]>([])
  let sort = $state<'default' | 'name_asc' | 'name_desc'>('default')
  let page = $state(1)
  let pageSize = $state(Number(localStorage.getItem(pageSizeKey)) || 25)

  const kindLabels: Record<string, string> = { managed: 'Managed', user: 'User-defined' }
  const filterKeys = $derived([...Object.keys(kindLabels), ...service.components.map((c) => `c:${c.name}`)])
  const filterLabel = (f: string) => kindLabels[f] ?? f.slice(2)
  const activeFilterText = $derived(filters.map(filterLabel).join(', '))

  const rows = $derived.by(() => {
    const q = search.trim().toLowerCase()
    const kinds = filters.filter((f) => f in kindLabels)
    const components = filters.filter((f) => f.startsWith('c:')).map((f) => f.slice(2))
    const out = vars.filter(
      (v) =>
        (!q || v.name.toLowerCase().includes(q)) &&
        (kinds.length === 0 || kinds.includes(v.magic ? 'managed' : 'user')) &&
        (components.length === 0 || v.components.some((c) => components.includes(c))),
    )
    if (sort === 'name_asc') out.sort((a, b) => a.name.localeCompare(b.name))
    if (sort === 'name_desc') out.sort((a, b) => b.name.localeCompare(a.name))
    return out
  })
  const pageRows = $derived(rows.slice((page - 1) * pageSize, page * pageSize))
  const searching = $derived(search.trim() !== '' || filters.length > 0)

  function toggleFilter(f: string) {
    filters = filters.includes(f) ? filters.filter((x) => x !== f) : [...filters, f]
    page = 1
  }

  // --- The row's dialog.
  let editing = $state(false)
  let editingName = $state('')
  let value = $state('')
  let formError = $state('')
  let saving = $state(false)

  const editingVar = $derived<ServiceVariable | undefined>(vars.find((v) => v.name === editingName))
  const canEditValue = $derived(projectAccess.can('manage_applications') && !!editingVar && !editingVar.magic && !editingVar.hidden)
  const dirty = $derived(canEditValue && value !== editingVar?.value)

  function openEdit(v: ServiceVariable) {
    editingName = v.name
    value = v.value
    formError = ''
    editing = true
  }

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    if (saving || !dirty) return
    saving = true
    formError = ''
    try {
      const r = await api<{ service: Service }>('PATCH', `/services/${service.id}`, { variables: { [editingName]: value } })
      onchange(r.service)
      editing = false
      toast.success('Environment variable updated.', 'The change applies on the next Restart.')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      formError = err.errors.variables ?? err.message
    } finally {
      saving = false
    }
  }
</script>

{#snippet check(on: boolean)}
  {#if on}
    <span class="data-table-cell-check">
      <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" class="size-4">
        <path stroke-linecap="round" stroke-linejoin="round" d="m4.5 12.75 6 6 9-13.5" />
      </svg>
    </span>
  {:else}
    <span class="data-table-cell-dash">-</span>
  {/if}
{/snippet}

{#snippet lock()}
  <svg class="size-3.5 shrink-0 text-neutral-400 dark:text-fg-faint" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" aria-label="Locked">
    <g fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="2">
      <path d="M5 13a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2v6a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2v-6z" />
      <path d="M11 16a1 1 0 1 0 2 0a1 1 0 0 0-2 0m-3-5V7a4 4 0 1 1 8 0v4" />
    </g>
  </svg>
{/snippet}

<div class="chrome flex flex-col gap-4">
  <SettingsSection id="environment-variables-section" title="Environment variables" helper="Environment variables (secrets) for this resource.">
    <p class="text-sm text-neutral-500 dark:text-fg-dim">Manage this resource's environment variables below.</p>
    <p class="mt-1 text-[13px] text-neutral-500 dark:text-fg-dim" data-testid="variables-from-compose">
      The Compose file declares these variables; add or remove one with Edit Compose file on
      <a class="underline" href={href(servicePath(service))}>General</a>.
    </p>
  </SettingsSection>

  {#if vars.length > 0}
    <div class="table-toolbar mt-2 flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center">
      <div class="w-full min-w-0 flex-1 sm:max-w-md">
        <div class="table-search relative w-full min-w-0">
          <input
            type="search"
            placeholder="Search environment variables"
            aria-label="Search environment variables"
            class="input w-full pl-8!"
            bind:value={search}
            oninput={() => (page = 1)}
          />
          <div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-2.5">
            <Icon name="search" class="size-3.5 text-neutral-400 dark:text-fg-faint" />
          </div>
        </div>
      </div>
      <div class="flex flex-wrap items-center gap-2 sm:ml-auto">
        <div class="table-filter">
          <TableDropdown panelClass="w-52! overflow-hidden! p-0!">
            {#snippet trigger({ open, toggle })}
              <button
                type="button"
                aria-haspopup="listbox"
                aria-expanded={open}
                title={activeFilterText || undefined}
                class={['button max-w-80 min-w-0', filters.length > 0 && 'button-highlighted']}
                onclick={toggle}
              >
                <Icon name="filter" class="size-3.5 shrink-0" />
                <span class="truncate">{activeFilterText || 'Filter'}</span>
                {#if filters.length > 0}
                  <span
                    class="shrink-0 rounded-full bg-neutral-100 px-1.5 py-0.5 text-[10px] font-medium text-neutral-500 dark:bg-white/[0.07] dark:text-fg-dim"
                    >{filters.length}</span
                  >
                {/if}
              </button>
            {/snippet}
            {#snippet children(close)}
              <div class="max-h-80 overflow-y-auto p-1">
                {#each filterKeys as f, i (f)}
                  {@const selected = filters.includes(f)}
                  {#if i === Object.keys(kindLabels).length}
                    <div class="my-1 border-t border-neutral-200 dark:border-white/10" aria-hidden="true"></div>
                  {/if}
                  <button type="button" class="listbox-option" role="option" aria-selected={selected} onclick={() => toggleFilter(f)}>
                    <span class="truncate">{filterLabel(f)}</span>
                    <span
                      class={[
                        'flex size-4 shrink-0 items-center justify-center rounded-[5px] border',
                        selected
                          ? 'border-coollabs bg-coollabs text-white dark:border-warning dark:bg-warning dark:text-black'
                          : 'border-neutral-300 bg-white dark:border-white/[0.14] dark:bg-white/[0.045]',
                      ]}
                    >
                      {#if selected}
                        <svg class="size-3" viewBox="0 0 12 12" fill="none" aria-hidden="true">
                          <path d="m2.25 6.15 2.35 2.3 5.15-5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
                        </svg>
                      {/if}
                    </span>
                  </button>
                {/each}
              </div>
              <div class="border-t border-neutral-200 bg-white p-1 dark:border-white/10 dark:bg-raised">
                <button
                  type="button"
                  class="listbox-option text-neutral-500 dark:text-fg-dim"
                  disabled={filters.length === 0}
                  onclick={() => {
                    filters = []
                    page = 1
                    close()
                  }}
                >
                  <span>Reset filters</span>
                  <Icon name="x" class="size-3.5" />
                </button>
              </div>
            {/snippet}
          </TableDropdown>
        </div>
        <TableDropdown panelClass="w-44!">
          {#snippet trigger({ open, toggle })}
            <button type="button" class="button" aria-haspopup="listbox" aria-expanded={open} onclick={toggle}>
              <svg class="size-3.5 opacity-65" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <path
                  d="M8 5v14m0 0-3-3m3 3 3-3M16 19V5m0 0-3 3m3-3 3 3"
                  stroke="currentColor"
                  stroke-width="1.7"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
              </svg>
              Sort
            </button>
          {/snippet}
          {#snippet children(close)}
            {#each [['default', 'Default order'], ['name_asc', 'Name A–Z'], ['name_desc', 'Name Z–A']] as const as [key, label] (key)}
              <button
                type="button"
                class="listbox-option"
                role="option"
                aria-selected={sort === key}
                onclick={() => {
                  sort = key
                  close()
                }}
              >
                <span>{label}</span>
                {#if sort === key}<span>✓</span>{/if}
              </button>
            {/each}
          {/snippet}
        </TableDropdown>
      </div>
    </div>
  {/if}

  <div id="environment-table-section" class={['application-settings-section-body relative mt-1 w-full scroll-mt-28', rows.length > 0 && 'is-flush']}>
    {#if searching && rows.length === 0}
      <Empty size="sm" title="No environment variables found" description="No variables match your search." />
    {:else if rows.length > 0}
      <div class="data-table w-full" data-testid="service-variables">
        <div class="environment-table-scroll relative">
          <div class="data-table-header env-table-grid-service">
            <span>Name</span>
            <span class="text-center">Managed</span>
            <span></span>
          </div>
          {#each pageRows as v (v.name)}
            <div class="env-table-item" data-testid={`variable-row-${v.name}`}>
              <div class="data-table-row env-table-grid-service">
                <div class="flex min-w-0 items-center gap-2">
                  {#if v.magic}{@render lock()}{/if}
                  <button
                    type="button"
                    class="env-key-label min-w-0 truncate text-left font-mono text-[13px] text-black dark:text-fg"
                    title={v.name}
                    onclick={() => openEdit(v)}
                  >
                    {v.name}
                  </button>
                </div>
                {@render check(!!v.magic)}
                <div class="justify-self-end">
                  <button
                    type="button"
                    class="icon-button shrink-0"
                    title="Edit environment variable"
                    aria-label={`Edit ${v.name}`}
                    onclick={() => openEdit(v)}
                  >
                    <Icon name="settings" class="size-3.5" />
                  </button>
                </div>
              </div>
            </div>
          {/each}
        </div>
        <ClientPagination bind:page bind:pageSize total={rows.length} storageKey={pageSizeKey} />
      </div>
    {:else}
      <Empty size="sm" title="No environment variables" description="The Compose file uses no variables." icon="variables" />
    {/if}
  </div>
</div>

<Modal title="Edit environment variable" variant="none" closeOutside={false} bind:open={editing}>
  {#if editingVar}
    <form class="flex w-full flex-col gap-4" onsubmit={submit}>
      <Input label="Name" value={editingVar.name} disabled />
      {#if editingVar.hidden}
        <div class="w-full">
          <span class="mb-1 flex items-center gap-1 text-sm font-medium">Value</span>
          <input disabled type="text" value={hiddenValue} aria-label="Value" class="input w-full italic text-neutral-500!" />
        </div>
      {:else if editingVar.magic}
        <CopyButton label="Value" text={editingVar.value} secret testid="generated-value" />
        <p class="-mt-2 text-xs text-neutral-500 dark:text-fg-faint">
          Generated ({editingVar.magic}) by The Bakery when the Service was created; it does not change.
        </p>
      {:else}
        <Input
          label="Value"
          type="password"
          bind:value
          placeholder={editingVar.default ?? ''}
          disabled={!canEditValue}
          helper={editingVar.default !== null ? `Empty uses the Compose file's default: ${editingVar.default || '(empty)'}` : undefined}
        />
      {/if}
      {#if editingVar.components.length > 0}
        <p class="text-[13px] text-neutral-500 dark:text-fg-dim">Used by {editingVar.components.join(', ')}.</p>
      {/if}
      <FieldError error={formError} />
      {#if canEditValue}
        <div class="flex flex-wrap items-center justify-end gap-2 border-t border-neutral-200 pt-4 dark:border-white/[0.07]">
          <Button type="submit" variant="highlighted" loading={saving} disabled={!dirty}>Update variable</Button>
        </div>
      {/if}
    </form>
  {/if}
</Modal>
