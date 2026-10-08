<script lang="ts">
  // Coolify's Environment Variables (resources/views/livewire/project/shared/environment-variable/all.blade.php,
  // add.blade.php and show.blade.php, app/Livewire/Project/Shared/EnvironmentVariable/*.php;
  // Apache-2.0, see NOTICE): the normal view (toolbar, one row per variable
  // with its flags, add and edit in a dialog) and the Developer view (one
  // `KEY=value` line each). Every change saves the whole set through `path`,
  // as the API takes it. Used by Applications and, for shared variables, by
  // Project and Environment settings.
  // Left out, as a Bakery variable has no such setting: Managed (generated)
  // variables, Literal, Multiline, Lock, comments, Preview Deployments
  // variables, the order and build secrets choices.
  import { untrack } from 'svelte'
  import Pencil from '@lucide/svelte/icons/pencil'
  import { Badge } from '@bakery/ui/components/ui/badge'
  import { buttonVariants } from '@bakery/ui/components/ui/button'
  import { api, ApiError } from './api'
  import CollectionToolbar from './CollectionToolbar.svelte'
  import FilterPopover from './FilterPopover.svelte'
  import Icon from './Icon.svelte'
  import SearchField from './SearchField.svelte'
  import SettingsGroup from './settings/SettingsGroup.svelte'
  import SortPopover from './SortPopover.svelte'
  import { projectAccess } from './projectAccess.svelte'
  import type { EnvironmentVariable, InheritedVariable } from './types'
  import Button from './ui/Button.svelte'
  import Callout from './ui/Callout.svelte'
  import ClientPagination from './ui/ClientPagination.svelte'
  import ConfirmationModal from './ui/ConfirmationModal.svelte'
  import Empty from './ui/Empty.svelte'
  import FieldError from './ui/FieldError.svelte'
  import Input from './ui/Input.svelte'
  import Modal from './ui/Modal.svelte'
  import Select from './ui/Select.svelte'
  import Spinner from './ui/Spinner.svelte'
  import Textarea from './ui/Textarea.svelte'
  import { toast } from './ui/toast.svelte'
  import UnsavedBar from './ui/UnsavedBar.svelte'

  let {
    path,
    title = 'Environment variables',
    helper = 'Environment variables (secrets) for this resource.',
  }: {
    /** The API path the variables are read from and saved to. */
    path: string
    title?: string
    helper?: string
  } = $props()

  // The same rule the API checks a name with.
  const validName = /^[A-Za-z_][A-Za-z0-9_]*$/

  let vars = $state.raw<EnvironmentVariable[]>([])
  let inherited = $state.raw<InheritedVariable[]>([])
  let loaded = $state(false)
  let loadError = $state('')
  let view = $state<'normal' | 'dev'>('normal')

  let searchText = $state('')
  let filters = $state<string[]>([])
  let sort = $state<'default' | 'name_asc' | 'name_desc'>('default')
  const sortOptions: { value: typeof sort; label: string }[] = [
    { value: 'default', label: 'Default order' },
    { value: 'name_asc', label: 'Name A–Z' },
    { value: 'name_desc', label: 'Name Z–A' },
  ]
  // Values stay masked until their row's eye is pressed.
  let revealed = $state<string[]>([])
  let page = $state(1)
  let pageSize = $state(Number(localStorage.getItem('bakery.page-size.environment-variables')) || 25)

  // The add and edit dialogs share one form.
  let adding = $state(false)
  let editing = $state(false)
  let editingName = $state('')
  let name = $state('')
  let value = $state('')
  let build = $state(false)
  let runtime = $state(true)
  let formError = $state('')
  let saving = $state(false)

  let developer = $state('')
  let developerErrors = $state<string[]>([])

  async function load() {
    const r = await api<{ environment_variables: EnvironmentVariable[]; inherited?: InheritedVariable[] }>('GET', path)
    vars = r.environment_variables
    inherited = r.inherited ?? []
    developer = format(vars)
    loaded = true
  }

  $effect(() => {
    void path
    untrack(() => {
      loaded = false
      loadError = ''
    })
    // Variable values are Secrets; a viewer's request would be refused.
    if (!projectAccess.can('see_secrets')) return
    load().catch((e) => (loadError = e.message))
  })

  // Saves the whole set; the API's message, when it refuses, is returned.
  async function replace(next: EnvironmentVariable[]): Promise<string> {
    try {
      await api('PUT', path, {
        environment_variables: next.map((v) => ({ name: v.name, value: v.value, build: v.build, runtime: v.runtime })),
      })
      // Read back, so overridden shared variables are marked again.
      await load()
      return ''
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      return err.errors.environment_variables ?? err.message
    }
  }

  const filterGroups = [
    {
      options: [
        { value: 'buildtime', label: 'Buildtime' },
        { value: 'runtime', label: 'Runtime' },
      ],
    },
  ]

  const rows = $derived.by(() => {
    const q = searchText.trim().toLowerCase()
    const out = vars.filter(
      (v) =>
        (!q || v.name.toLowerCase().includes(q)) &&
        (!filters.includes('buildtime') || v.build) &&
        (!filters.includes('runtime') || v.runtime),
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

  function openAdd() {
    name = ''
    value = ''
    build = false
    runtime = true
    formError = ''
    adding = true
  }

  function openEdit(v: EnvironmentVariable) {
    editingName = v.name
    name = v.name
    value = v.value
    build = v.build
    runtime = v.runtime
    formError = ''
    editing = true
  }

  const editingVar = $derived(vars.find((v) => v.name === editingName))
  const editDirty = $derived(
    !!editingVar && (name !== editingVar.name || value !== editingVar.value || build !== editingVar.build || runtime !== editingVar.runtime),
  )

  async function submitAdd(e: SubmitEvent) {
    e.preventDefault()
    if (saving) return
    const n = name.trim()
    formError = vars.some((v) => v.name === n) ? `${n} already exists.` : ''
    if (formError) return
    saving = true
    formError = await replace([...vars, { name: n, value, build, runtime }])
    saving = false
    if (formError) return
    adding = false
    toast.success('Environment variable added.')
  }

  async function submitEdit(e: SubmitEvent) {
    e.preventDefault()
    if (saving || !editDirty) return
    saving = true
    const n = name.trim()
    formError = await replace(vars.map((v) => (v.name === editingName ? { name: n, value, build, runtime } : v)))
    saving = false
    if (formError) return
    editing = false
    toast.success('Environment variable updated.')
  }

  async function remove() {
    const message = await replace(vars.filter((v) => v.name !== editingName))
    if (message) {
      toast.error('Environment variable not deleted', message)
      throw new Error(message)
    }
    editing = false
    toast.success('Environment variable deleted.')
  }

  // Coolify's Developer view: one KEY=value line per variable.
  function format(list: EnvironmentVariable[]): string {
    return list.map((v) => `${v.name}=${v.value}`).join('\n')
  }

  // Coolify's parseEnvFormatToArray: blank lines and lines starting with #
  // are skipped; a quoted value ends at its closing quote, an unquoted one at
  // whitespace followed by #. Unlike Coolify a line without = is an error,
  // not dropped, and names are checked as the API checks them.
  function parse(text: string): { list: { name: string; value: string }[]; errors: string[] } {
    const list: { name: string; value: string }[] = []
    const errors: string[] = []
    const seen = new Set<string>()
    text.split('\n').forEach((raw, i) => {
      const line = raw.replace(/\r$/, '')
      if (line.trim() === '' || line.startsWith('#')) return
      const at = line.indexOf('=')
      if (at < 0) {
        errors.push(`Line ${i + 1}: no "=" between the name and the value.`)
        return
      }
      const key = line.slice(0, at).trim()
      const rest = line.slice(at + 1)
      let v: string
      const quote = rest[0] === '"' || rest[0] === "'" ? rest[0] : ''
      const close = quote ? rest.indexOf(quote, 1) : -1
      if (quote && close > 0) v = rest.slice(1, close)
      else if (quote) v = rest.slice(1)
      else {
        const comment = /\s+#/.exec(rest)
        v = comment ? rest.slice(0, comment.index).trimEnd() : rest
      }
      if (!validName.test(key)) errors.push(`Line ${i + 1}: "${key}" is not a valid variable name (letters, digits and _, not starting with a digit).`)
      else if (seen.has(key)) errors.push(`Line ${i + 1}: ${key} is set twice.`)
      seen.add(key)
      list.push({ name: key, value: v })
    })
    return { list, errors }
  }

  const developerDirty = $derived(loaded && developer !== format(vars))

  async function submitDeveloper() {
    if (saving) return
    const parsed = parse(developer)
    developerErrors = parsed.errors
    if (parsed.errors.length > 0) return
    saving = true
    // A variable that stays keeps where it reaches; a new one is runtime only.
    const message = await replace(
      parsed.list.map((p) => {
        const old = vars.find((v) => v.name === p.name)
        return { ...p, build: old?.build ?? false, runtime: old?.runtime ?? true }
      }),
    )
    saving = false
    if (message) {
      developerErrors = [message]
      return
    }
    toast.success('Environment variables updated.')
  }

  function switchView(next: 'normal' | 'dev') {
    if (view === next) return
    view = next
    developer = format(vars)
    developerErrors = []
  }
</script>

{#snippet flags()}
  <div class="grid gap-4 border-t border-border pt-4 sm:grid-cols-2">
    <Select
      label="Build time"
      helper="Make this variable available during the Docker build process. Useful for build secrets and dependencies."
      bind:value={build}
    >
      <option value={true}>Available during build</option>
      <option value={false}>Not available during build</option>
    </Select>
    <Select label="Runtime" helper="Make this variable available in the running container at runtime." bind:value={runtime}>
      <option value={true}>Available in the container</option>
      <option value={false}>Not available in the container</option>
    </Select>
  </div>
  {#if build}
    <p class="text-xs text-muted-foreground">
      Build variables are handed to the Dockerfile's <code>ARG</code>s and end up in the image's history; keep secrets runtime only.
    </p>
  {/if}
{/snippet}

{#snippet chips(v: EnvironmentVariable)}
  {#if v.build}<Badge variant="outline">Buildtime</Badge>{/if}
  {#if v.runtime}<Badge variant="outline">Runtime</Badge>{/if}
{/snippet}

{#snippet masked(v: EnvironmentVariable)}
  {@const shown = revealed.includes(v.name)}
  <span class="min-w-0 flex-1 truncate font-mono text-xs text-muted-foreground" data-testid={`variable-value-${v.name}`}>
    {#if !shown}••••••••{:else if v.value === ''}<span class="italic">(empty)</span>{:else}{v.value}{/if}
  </span>
{/snippet}

<SettingsGroup id="environment-variables-section" label={title} hint={helper} wide>
  {#snippet actions()}
    {#if projectAccess.can('manage_applications') && loaded}
      <div class="flex items-center" role="group" aria-label="View">
        {#each [['normal', 'Normal view'], ['dev', 'Developer view']] as const as [mode, label] (mode)}
          <button
            type="button"
            onclick={() => switchView(mode)}
            class={buttonVariants({
              variant: 'ghost',
              size: 'sm',
              class: ['text-xs', view === mode ? 'bg-accent text-foreground' : 'text-muted-foreground'].join(' '),
            })}
            aria-pressed={view === mode}
          >
            {label}
          </button>
        {/each}
      </div>
    {/if}
  {/snippet}

  {#if !projectAccess.can('see_secrets')}
    <p class="text-sm text-muted-foreground">Values are hidden (they need the See secrets permission).</p>
  {:else if view === 'dev'}
    <form
      class="flex w-full flex-col gap-4"
      onsubmit={(e) => {
        e.preventDefault()
        submitDeveloper()
      }}
    >
      <Callout type="info" title="Note">
        Inline comments with space before # (e.g., <code class="font-mono">KEY=value #comment</code>) are stripped.
      </Callout>
      <Textarea
        rows={10}
        monospace
        class="whitespace-pre-wrap"
        label="Production"
        bind:value={developer}
        spellcheck={false}
        oninput={() => (developerErrors = [])}
      />
      {#each developerErrors as message (message)}<FieldError error={message} />{/each}
      <UnsavedBar
        dirty={developerDirty}
        {saving}
        onsave={submitDeveloper}
        onreset={() => {
          developer = format(vars)
          developerErrors = []
        }}
      />
    </form>
  {:else}
    <p class="text-sm text-muted-foreground">
      Manage this resource's environment variables below. Values are stored encrypted and apply on the next deploy.
    </p>
    <CollectionToolbar ariaLabel="Environment variables controls">
      {#snippet search()}
        <SearchField bind:value={searchText} label="Search environment variables" oninput={() => (page = 1)} />
      {/snippet}
      {#snippet controls()}
        <FilterPopover bind:selected={filters} groups={filterGroups} label="Filter environment variables" clearLabel="Reset filters" onchange={() => (page = 1)} />
        <SortPopover bind:value={sort} options={sortOptions} label="Sort environment variables" onchange={() => (page = 1)} />
      {/snippet}
      {#snippet actions()}
        {#if projectAccess.can('manage_applications')}
          <button type="button" class={buttonVariants({ variant: 'outline', size: 'sm' })} onclick={openAdd} disabled={!loaded}>
            <Icon name="plus" class="size-4" />
            Add
          </button>
        {/if}
      {/snippet}
    </CollectionToolbar>

    <div id="environment-table-section" class="scroll-mt-28 overflow-hidden rounded-md border border-border">
      {#if !loaded}
        <div class="flex min-h-40 items-center justify-center p-4">
          {#if loadError}<p class="text-sm text-destructive">{loadError}</p>{:else}<Spinner text="Loading environment variables..." />{/if}
        </div>
      {:else if searching && rows.length === 0}
        <div class="p-4"><Empty size="sm" title="No environment variables found" description="No variables match your search." /></div>
      {:else if rows.length > 0}
        {#each pageRows as v (v.name)}
          <div class="flex flex-wrap items-center gap-x-3 gap-y-1 border-b border-border px-4 py-2 last:border-b-0" data-testid={`variable-row-${v.name}`}>
            {#if projectAccess.can('manage_applications')}
              <button
                type="button"
                class="max-w-full min-w-0 truncate text-left font-mono text-sm hover:underline sm:max-w-64"
                title={v.name}
                onclick={() => openEdit(v)}>{v.name}</button
              >
            {:else}
              <span class="max-w-full min-w-0 truncate font-mono text-sm sm:max-w-64" title={v.name}>{v.name}</span>
            {/if}
            {@render masked(v)}
            <div class="flex items-center gap-1">
              {@render chips(v)}
              <button
                type="button"
                class={buttonVariants({ variant: 'ghost', size: 'icon-sm', class: 'text-muted-foreground' })}
                title={revealed.includes(v.name) ? 'Hide value' : 'Show value'}
                aria-label={`${revealed.includes(v.name) ? 'Hide' : 'Show'} ${v.name}`}
                aria-pressed={revealed.includes(v.name)}
                onclick={() => toggleReveal(v.name)}
              >
                <Icon name={revealed.includes(v.name) ? 'eye-off' : 'eye'} class="size-3.5" />
              </button>
              {#if projectAccess.can('manage_applications')}
                <button
                  type="button"
                  class={buttonVariants({ variant: 'ghost', size: 'icon-sm', class: 'text-muted-foreground' })}
                  title="Edit environment variable"
                  aria-label={`Edit ${v.name}`}
                  onclick={() => openEdit(v)}
                >
                  <Pencil class="size-3.5" />
                </button>
              {/if}
            </div>
          </div>
        {/each}
        <ClientPagination bind:page bind:pageSize total={rows.length} storageKey="bakery.page-size.environment-variables" />
      {:else}
        <div class="p-4">
          <Empty
            size="sm"
            title="No environment variables"
            description={projectAccess.can('manage_applications') ? 'Add your first variable with the + Add button above.' : undefined}
            icon="variables"
          />
        </div>
      {/if}
    </div>
  {/if}
</SettingsGroup>

{#if projectAccess.can('see_secrets') && view === 'normal' && inherited.length > 0}
  <!-- The Bakery's own: what the Project and the Environment hand down. -->
  <SettingsGroup
    id="inherited-variables-section"
    label="Shared variables"
    hint="Inherited from the environment and the project. The application's own variables win."
    wide
  >
    <div class="overflow-hidden rounded-md border border-border">
      {#each inherited as v (v.from + v.name)}
        <div class="flex flex-wrap items-center gap-x-3 gap-y-1 border-b border-border px-4 py-2 last:border-b-0">
          <span class={['max-w-full min-w-0 truncate font-mono text-sm sm:max-w-64', v.overridden && 'line-through opacity-60']} title={v.name}
            >{v.name}</span
          >
          <span class="min-w-0 flex-1 text-xs text-muted-foreground capitalize">{v.from}</span>
          <div class="flex items-center gap-1">
            {#if v.overridden}<Badge variant="secondary">Overridden</Badge>{/if}
            {@render chips(v)}
          </div>
        </div>
      {/each}
    </div>
  </SettingsGroup>
{/if}

{#if projectAccess.can('manage_applications')}
  <Modal title="New Environment Variable" variant="none" closeOutside={false} bind:open={adding}>
    <form class="flex w-full flex-col gap-4" onsubmit={submitAdd}>
      <Input placeholder="NODE_ENV" label="Name" required bind:value={name} />
      <Input placeholder="production" label="Value" type="password" bind:value />
      {@render flags()}
      <FieldError error={formError} />
      <div class="flex justify-end border-t border-border pt-4">
        <Button type="submit" loading={saving}>Add variable</Button>
      </div>
    </form>
  </Modal>

  <Modal title="Edit environment variable" variant="none" closeOutside={false} bind:open={editing}>
    <form class="flex w-full flex-col gap-4" onsubmit={submitEdit}>
      <Input label="Name" required bind:value={name} />
      <Input label="Value" type="password" bind:value />
      {@render flags()}
      <FieldError error={formError} />
      <div class="flex flex-wrap items-center justify-between gap-2 border-t border-border pt-4">
        <div>
          <ConfirmationModal
            title="Confirm Environment Variable Deletion?"
            buttonTitle="Delete"
            variant="error"
            actions={['The selected environment variable will be permanently deleted.']}
            confirmationText={editingName}
            confirmationLabel="Please confirm the execution of the actions by entering the Environment Variable Name below"
            shortConfirmationLabel="Environment Variable Name"
            step2ButtonText="Permanently Delete"
            onconfirm={remove}
          />
        </div>
        <div class="ml-auto flex flex-wrap gap-2">
          <Button type="submit" variant="highlighted" loading={saving} disabled={!editDirty}>Update variable</Button>
        </div>
      </div>
    </form>
  </Modal>
{/if}
