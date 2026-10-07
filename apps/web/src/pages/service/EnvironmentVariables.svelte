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
  import Pencil from '@lucide/svelte/icons/pencil'
  import { Badge } from '$lib/components/ui/badge'
  import { buttonVariants } from '$lib/components/ui/button'
  import { api, ApiError } from '../../lib/api'
  import CollectionToolbar from '../../lib/CollectionToolbar.svelte'
  import FilterPopover from '../../lib/FilterPopover.svelte'
  import Icon from '../../lib/Icon.svelte'
  import SearchField from '../../lib/SearchField.svelte'
  import SettingsGroup from '../../lib/settings/SettingsGroup.svelte'
  import SortPopover from '../../lib/SortPopover.svelte'
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
  import { toast } from '../../lib/ui/toast.svelte'

  let { service, onchange }: { service: Service; onchange: (s: Service) => void } = $props()

  const hiddenValue = 'Hidden (viewers cannot see it)'
  const pageSizeKey = 'bakery.page-size.environment-variables'

  const vars = $derived(service.variables ?? [])

  let searchText = $state('')
  // Coolify's variable filters plus one per Component ("c:<name>", so a
  // Component named like a filter stays apart).
  let filters = $state<string[]>([])
  let sort = $state<'default' | 'name_asc' | 'name_desc'>('default')
  let page = $state(1)
  let pageSize = $state(Number(localStorage.getItem(pageSizeKey)) || 25)

  const kindLabels: Record<string, string> = { managed: 'Managed', user: 'User-defined' }
  const filterGroups = $derived([
    { options: Object.entries(kindLabels).map(([value, label]) => ({ value, label })) },
    { options: service.components.map((c) => ({ value: `c:${c.name}`, label: c.name })) },
  ])
  const sortOptions: { value: typeof sort; label: string }[] = [
    { value: 'default', label: 'Default order' },
    { value: 'name_asc', label: 'Name A–Z' },
    { value: 'name_desc', label: 'Name Z–A' },
  ]
  // Values stay masked until their row's eye is pressed.
  let revealed = $state<string[]>([])

  const rows = $derived.by(() => {
    const q = searchText.trim().toLowerCase()
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
  const searching = $derived(searchText.trim() !== '' || filters.length > 0)

  function toggleReveal(n: string) {
    revealed = revealed.includes(n) ? revealed.filter((x) => x !== n) : [...revealed, n]
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

<SettingsGroup id="environment-variables-section" label="Environment variables" hint="Environment variables (secrets) for this resource." wide>
  <p class="text-sm text-muted-foreground">Manage this resource's environment variables below.</p>
  <p class="text-sm text-muted-foreground" data-testid="variables-from-compose">
    The Compose file declares these variables; add or remove one with Edit Compose file on
    <a class="underline underline-offset-2 hover:text-foreground" href={href(servicePath(service))}>General</a>.
  </p>

  {#if vars.length > 0}
    <CollectionToolbar ariaLabel="Environment variables controls">
      {#snippet search()}
        <SearchField bind:value={searchText} label="Search environment variables" oninput={() => (page = 1)} />
      {/snippet}
      {#snippet controls()}
        <FilterPopover bind:selected={filters} groups={filterGroups} label="Filter environment variables" clearLabel="Reset filters" onchange={() => (page = 1)} />
        <SortPopover bind:value={sort} options={sortOptions} label="Sort environment variables" onchange={() => (page = 1)} />
      {/snippet}
    </CollectionToolbar>
  {/if}

  <div id="environment-table-section" class="scroll-mt-28 overflow-hidden rounded-md border border-border">
    {#if searching && rows.length === 0}
      <div class="p-4"><Empty size="sm" title="No environment variables found" description="No variables match your search." /></div>
    {:else if rows.length > 0}
      <div data-testid="service-variables">
        {#each pageRows as v (v.name)}
          {@const shown = revealed.includes(v.name)}
          <div class="flex flex-wrap items-center gap-x-3 gap-y-1 border-b border-border px-4 py-2 last:border-b-0" data-testid={`variable-row-${v.name}`}>
            <div class="flex max-w-full min-w-0 items-center gap-2 sm:max-w-64">
              {#if v.magic}<span class="shrink-0 text-muted-foreground" title="Locked"><Icon name="lock" class="size-3.5" /></span>{/if}
              <button type="button" class="min-w-0 truncate text-left font-mono text-sm hover:underline" title={v.name} onclick={() => openEdit(v)}>
                {v.name}
              </button>
            </div>
            <span class="min-w-0 flex-1 truncate font-mono text-xs text-muted-foreground" data-testid={`variable-value-${v.name}`}>
              {#if v.hidden}<span class="font-sans italic">Hidden</span>{:else if !shown}••••••••{:else if v.value === ''}<span class="italic"
                  >(empty)</span
                >{:else}{v.value}{/if}
            </span>
            <div class="flex items-center gap-1">
              {#if v.magic}<Badge variant="outline">Managed</Badge>{/if}
              {#if !v.hidden}
                <button
                  type="button"
                  class={buttonVariants({ variant: 'ghost', size: 'icon-sm', class: 'text-muted-foreground' })}
                  title={shown ? 'Hide value' : 'Show value'}
                  aria-label={`${shown ? 'Hide' : 'Show'} ${v.name}`}
                  aria-pressed={shown}
                  onclick={() => toggleReveal(v.name)}
                >
                  <Icon name={shown ? 'eye-off' : 'eye'} class="size-3.5" />
                </button>
              {/if}
              <button
                type="button"
                class={buttonVariants({ variant: 'ghost', size: 'icon-sm', class: 'text-muted-foreground' })}
                title="Edit environment variable"
                aria-label={`Edit ${v.name}`}
                onclick={() => openEdit(v)}
              >
                <Pencil class="size-3.5" />
              </button>
            </div>
          </div>
        {/each}
        <ClientPagination bind:page bind:pageSize total={rows.length} storageKey={pageSizeKey} />
      </div>
    {:else}
      <div class="p-4"><Empty size="sm" title="No environment variables" description="The Compose file uses no variables." icon="variables" /></div>
    {/if}
  </div>
</SettingsGroup>

<Modal title="Edit environment variable" variant="none" closeOutside={false} bind:open={editing}>
  {#if editingVar}
    <form class="flex w-full flex-col gap-4" onsubmit={submit}>
      <Input label="Name" value={editingVar.name} disabled />
      {#if editingVar.hidden}
        <Input label="Value" value={hiddenValue} disabled class="italic" />
      {:else if editingVar.magic}
        <CopyButton label="Value" text={editingVar.value} secret testid="generated-value" />
        <p class="-mt-2 text-xs text-muted-foreground">
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
        <p class="text-sm text-muted-foreground">Used by {editingVar.components.join(', ')}.</p>
      {/if}
      <FieldError error={formError} />
      {#if canEditValue}
        <div class="flex flex-wrap items-center justify-end gap-2 border-t border-border pt-4">
          <Button type="submit" variant="highlighted" loading={saving} disabled={!dirty}>Update variable</Button>
        </div>
      {/if}
    </form>
  {/if}
</Modal>
