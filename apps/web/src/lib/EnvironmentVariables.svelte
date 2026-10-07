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
  import { api, ApiError } from './api'
  import Icon from './Icon.svelte'
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
  import SettingsSection from './ui/SettingsSection.svelte'
  import Spinner from './ui/Spinner.svelte'
  import TableDropdown from './ui/TableDropdown.svelte'
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

  let search = $state('')
  let filters = $state<('buildtime' | 'runtime')[]>([])
  let sort = $state<'default' | 'name_asc' | 'name_desc'>('default')
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

  const filterLabels = { buildtime: 'Buildtime', runtime: 'Runtime' } as const
  const activeFilterText = $derived(filters.map((f) => filterLabels[f]).join(', '))

  const rows = $derived.by(() => {
    const q = search.trim().toLowerCase()
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
  const searching = $derived(search.trim() !== '' || filters.length > 0)

  function toggleFilter(f: 'buildtime' | 'runtime') {
    filters = filters.includes(f) ? filters.filter((x) => x !== f) : [...filters, f]
    page = 1
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

  function switchView() {
    view = view === 'normal' ? 'dev' : 'normal'
    developer = format(vars)
    developerErrors = []
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

{#snippet flags()}
  <div class="grid gap-4 border-t border-neutral-200 pt-4 sm:grid-cols-2 dark:border-white/[0.07]">
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
    <p class="text-[12px] text-neutral-500 dark:text-fg-dim">
      Build variables are handed to the Dockerfile's <code>ARG</code>s and end up in the image's history; keep secrets runtime only.
    </p>
  {/if}
{/snippet}

<div class="chrome flex flex-col gap-4">
  <SettingsSection id="environment-variables-section" {title} {helper}>
    {#snippet actions()}
      {#if projectAccess.can('manage_applications') && loaded}
        <Button onclick={switchView}>{view === 'normal' ? 'Developer view' : 'Normal view'}</Button>
      {/if}
    {/snippet}
    {#if !projectAccess.can('see_secrets')}
      <p class="text-sm text-neutral-500 dark:text-fg-dim">Values are hidden (they need the See secrets permission).</p>
    {:else if view === 'normal'}
      <p class="text-sm text-neutral-500 dark:text-fg-dim">
        Manage this resource's environment variables below. Values are stored encrypted and apply on the next deploy.
      </p>
    {:else}
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
          class="font-sans whitespace-pre-wrap"
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
    {/if}
  </SettingsSection>

  {#if projectAccess.can('see_secrets') && view === 'normal'}
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
            disabled={!loaded}
          />
          <div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-2.5">
            <Icon name="search" class="size-3.5 text-neutral-400 dark:text-fg-faint" />
          </div>
        </div>
      </div>
      <div class="flex flex-wrap items-center gap-2 sm:ml-auto">
        <div class="table-filter">
          <TableDropdown panelClass="w-44! overflow-hidden! p-0!">
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
                {#each ['buildtime', 'runtime'] as const as f (f)}
                  {@const selected = filters.includes(f)}
                  <button type="button" class="listbox-option" role="option" aria-selected={selected} onclick={() => toggleFilter(f)}>
                    <span>{filterLabels[f]}</span>
                    <span
                      class={[
                        'flex size-4 shrink-0 items-center justify-center rounded-[5px] border',
                        selected
                          ? 'border-coollabs bg-coollabs text-primary-foreground'
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
        {#if projectAccess.can('manage_applications')}
          <button type="button" class="button button-highlighted" onclick={openAdd}>
            <Icon name="plus" class="size-3.5" />
            Add
          </button>
        {/if}
      </div>
    </div>

    {#if !loaded}
      <div class="application-settings-section-body mt-1 flex min-h-40 w-full items-center justify-center">
        {#if loadError}<p class="text-sm text-error">{loadError}</p>{:else}<Spinner text="Loading environment variables..." />{/if}
      </div>
    {:else}
      <div
        id="environment-table-section"
        class={['application-settings-section-body relative mt-1 w-full scroll-mt-28', rows.length > 0 && 'is-flush']}
      >
        {#if searching && rows.length === 0}
          <Empty size="sm" title="No environment variables found" description="No variables match your search." />
        {:else if rows.length > 0}
          <div class="data-table w-full">
            <div class="environment-table-scroll relative">
              <div class="data-table-header env-table-grid">
                <span>Name</span>
                <span class="text-center">Buildtime</span>
                <span class="text-center">Runtime</span>
                <span></span>
              </div>
              {#each pageRows as v (v.name)}
                <div class="env-table-item">
                  <div class="data-table-row env-table-grid">
                    <div class="flex min-w-0 items-center gap-2">
                      <button
                        type="button"
                        class="env-key-label min-w-0 truncate text-left font-mono text-[13px] text-black dark:text-fg"
                        title={v.name}
                        onclick={() => openEdit(v)}
                      >
                        {v.name}
                      </button>
                    </div>
                    {@render check(v.build)}
                    {@render check(v.runtime)}
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
            <ClientPagination bind:page bind:pageSize total={rows.length} storageKey="bakery.page-size.environment-variables" />
          </div>
        {:else}
          <Empty
            size="sm"
            title="No environment variables"
            description={projectAccess.can('manage_applications') ? 'Add your first variable with the + Add button above.' : undefined}
            icon="variables"
          />
        {/if}
      </div>
    {/if}

    {#if inherited.length > 0}
      <!-- The Bakery's own: what the Project and the Environment hand down. -->
      <SettingsSection
        id="inherited-variables-section"
        title="Shared variables"
        helper="Inherited from the environment and the project. The application's own variables win."
        flush
      >
        <div class="data-table w-full">
          <div class="data-table-header env-table-grid-inherited">
            <span>Name</span>
            <span>From</span>
            <span class="text-center">Buildtime</span>
            <span class="text-center">Runtime</span>
          </div>
          {#each inherited as v (v.from + v.name)}
            <div class="env-table-item">
              <div class="data-table-row env-table-grid-inherited">
                <div class="flex min-w-0 items-center gap-2">
                  <span class={['min-w-0 truncate font-mono text-[13px]', v.overridden && 'line-through opacity-60']} title={v.name}>{v.name}</span>
                  {#if v.overridden}<span class="table-badge shrink-0">Overridden</span>{/if}
                </div>
                <span class="text-[13px] text-neutral-500 capitalize dark:text-fg-dim">{v.from}</span>
                {@render check(v.build)}
                {@render check(v.runtime)}
              </div>
            </div>
          {/each}
        </div>
      </SettingsSection>
    {/if}
  {/if}
</div>

{#if projectAccess.can('manage_applications')}
  <Modal title="New Environment Variable" variant="none" closeOutside={false} bind:open={adding}>
    <form class="flex w-full flex-col gap-4" onsubmit={submitAdd}>
      <Input placeholder="NODE_ENV" label="Name" required bind:value={name} />
      <Input placeholder="production" label="Value" type="password" bind:value />
      {@render flags()}
      <FieldError error={formError} />
      <div class="flex justify-end border-t border-neutral-200 pt-4 dark:border-white/[0.07]">
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
      <div class="flex flex-wrap items-center justify-between gap-2 border-t border-neutral-200 pt-4 dark:border-white/[0.07]">
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
