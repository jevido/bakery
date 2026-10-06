<script lang="ts">
  // Coolify's Service General page
  // (resources/views/livewire/project/service/stack-form.blade.php, the
  // Compose resources block of configuration.blade.php, resource-card.blade.php
  // and project/shared/resource-details.blade.php;
  // app/Livewire/Project/Service/StackForm.php; Apache-2.0, see NOTICE), with
  // only what The Bakery has: no Network section, no "Preview generated
  // Compose" or "Validate" (the Compose file is checked when it is saved), and
  // a Component's modal that shows instead of edits, since its image lives in
  // the Compose file. Name, description and a Member's variables save
  // together through the unsaved bar, as Coolify's one form does.
  import { untrack } from 'svelte'
  import { api, ApiError } from '../../lib/api'
  import Icon from '../../lib/Icon.svelte'
  import { href, servicePath } from '../../lib/router.svelte'
  import { session } from '../../lib/session.svelte'
  import type { Component, Environment, Service, ServiceTemplate } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import CopyButton from '../../lib/ui/CopyButton.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import Helper from '../../lib/ui/Helper.svelte'
  import Input from '../../lib/ui/Input.svelte'
  import Modal from '../../lib/ui/Modal.svelte'
  import SettingsSection from '../../lib/ui/SettingsSection.svelte'
  import StatusBadge, { type StatusType } from '../../lib/ui/StatusBadge.svelte'
  import Textarea from '../../lib/ui/Textarea.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import UnsavedBar from '../../lib/ui/UnsavedBar.svelte'

  let {
    service,
    environment,
    onchange,
  }: { service: Service; environment: Environment | null; onchange: (s: Service) => void } = $props()

  const hiddenValue = 'Hidden (viewers cannot see it)'
  const appliesOnRestart = 'The change applies on the next Restart.'
  const canUpdate = $derived(session.can('manage_applications'))

  // --- Service details and Service configuration: one form.
  let name = $state('')
  let description = $state('')
  let values = $state<Record<string, string>>({})
  let errors = $state<Record<string, string>>({})
  let saving = $state(false)

  function reset() {
    name = service.name
    description = service.description ?? ''
    values = Object.fromEntries((service.variables ?? []).filter((v) => !v.magic && !v.hidden).map((v) => [v.name, v.value]))
    errors = {}
  }

  $effect(() => {
    void service.id
    untrack(reset)
  })

  const variables = $derived(service.variables ?? [])
  const changedValues = $derived(
    Object.fromEntries(
      variables.filter((v) => !v.magic && !v.hidden && values[v.name] !== undefined && values[v.name] !== v.value).map((v) => [v.name, values[v.name]]),
    ),
  )
  const dirty = $derived(
    canUpdate &&
      (name !== service.name || description !== (service.description ?? '') || Object.keys(changedValues).length > 0),
  )

  async function save() {
    if (saving || !dirty) return
    saving = true
    errors = {}
    const variablesChanged = Object.keys(changedValues).length > 0
    try {
      const r = await api<{ service: Service }>('PATCH', `/services/${service.id}`, {
        name,
        description,
        ...(variablesChanged ? { variables: changedValues } : {}),
      })
      onchange(r.service)
      untrack(reset)
      toast.success('Service saved.', variablesChanged ? appliesOnRestart : '')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      errors = Object.keys(err.errors).length ? err.errors : { name: err.message }
    } finally {
      saving = false
    }
  }

  // StackForm's fields: "{component} · {name}" when a Component uses the
  // variable, the variable's name otherwise.
  const fieldLabel = (v: { name: string; components: string[] }) => (v.components?.length ? `${v.components[0]} · ${v.name}` : v.name)

  // --- Edit Compose file.
  let composeOpen = $state(false)
  let composeDraft = $state('')
  let composeErrors = $state<string[]>([])
  let composeSaving = $state(false)

  function openCompose(show: () => void) {
    composeDraft = service.compose ?? ''
    composeErrors = []
    show()
  }

  async function saveCompose() {
    if (composeSaving) return
    composeSaving = true
    composeErrors = []
    try {
      const r = await api<{ service: Service }>('PATCH', `/services/${service.id}`, { compose: composeDraft })
      onchange(r.service)
      untrack(reset)
      composeOpen = false
      toast.success('Service saved.', appliesOnRestart)
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      composeErrors = (err.errors.compose ?? err.errors.domains ?? err.message).split('\n')
    } finally {
      composeSaving = false
    }
  }

  // --- Compose resources.
  type ViewMode = 'table' | 'grid'
  const viewKey = 'service-compose-resources-view'
  let viewMode = $state<ViewMode>(localStorage.getItem(viewKey) === 'grid' ? 'grid' : 'table')
  function setViewMode(mode: ViewMode) {
    viewMode = mode
    localStorage.setItem(viewKey, mode)
  }
  const toggleIdle =
    'text-neutral-400 hover:bg-neutral-100 hover:text-black dark:text-fg-faint dark:hover:bg-white/[0.06] dark:hover:text-fg'

  // The template's documentation, for the Documentation button.
  let docsUrl = $state('')
  $effect(() => {
    const key = service.template
    docsUrl = ''
    if (!key) return
    api<{ templates: ServiceTemplate[] }>('GET', '/service-templates')
      .then((r) => (docsUrl = r.templates.find((t) => t.key === key)?.docs_url ?? ''))
      .catch(() => {})
  })

  function headline(s: string): string {
    return s
      .replace(/[_-]+/g, ' ')
      .trim()
      .split(/\s+/)
      .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
      .join(' ')
  }

  // resource-card's badge: running is success; starting, restarting and
  // degraded are warning; anything else is error, in formatContainerStatus's
  // words.
  function componentStatus(status: string): { status: string; type: StatusType } {
    const type: StatusType = status.includes('running')
      ? 'success'
      : /starting|restarting|degraded/.test(status)
        ? 'warning'
        : 'error'
    return { status: headline(status), type }
  }

  let openComponent = $state<string | null>(null)
  const domainsHref = $derived(href(servicePath(service, 'domains')))
  const tableColumns = 'grid-cols-[minmax(14rem,1fr)_minmax(12rem,1fr)_12rem_5rem]'
  const iconBadge = 'flex shrink-0 items-center justify-center rounded-lg bg-neutral-100 text-neutral-500 dark:bg-white/[0.06] dark:text-fg-dim'
</script>

{#snippet componentActions(c: Component)}
  {#if c.public && c.domains.length > 0 && canUpdate}
    <a class="icon-button" title="Manage domains" aria-label="Manage domains" href={domainsHref}>
      <Icon name="globe" class="size-4" />
    </a>
  {/if}
  <button type="button" class="icon-button" title="Resource settings" aria-label="Resource settings" onclick={() => (openComponent = c.name)}>
    <Icon name="settings" class="size-4" />
  </button>
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

  <SettingsSection id="service-details-section" title="Service details" helper="Manage the identity and Compose configuration for this service.">
    {#snippet actions()}
      <div class="flex items-center gap-2">
        {#if canUpdate}
          <Modal title="Docker Compose" bind:open={composeOpen} closeOutside={false} isLarge>
            {#snippet trigger(show)}
              <Button onclick={() => openCompose(show)}>Edit Compose file</Button>
            {/snippet}
            <Textarea
              label="Compose file"
              bind:value={composeDraft}
              monospace
              allowTab
              rows={24}
              data-testid="compose-file"
            />
            {#if composeErrors.length > 0}
              <ul class="mt-2 list-disc pl-5 font-mono text-xs text-error" data-testid="compose-errors">
                {#each composeErrors as line, i (i)}<li>{line}</li>{/each}
              </ul>
            {/if}
            {#snippet footer()}
              <div class="flex flex-wrap items-center justify-end gap-2">
                <Button variant="highlighted" loading={composeSaving} onclick={saveCompose}>Save changes</Button>
              </div>
            {/snippet}
          </Modal>
        {/if}
        <Modal title="Resource details" buttonTitle="Details">
          <div class="max-h-[70vh] w-full overflow-y-auto pt-1 pr-1">
            <div class="flex flex-col gap-6">
              <div>
                <h3>Resource</h3>
                <div class="grid grid-cols-1 gap-3 pt-2 md:grid-cols-2">
                  <CopyButton label="Name" text={service.name} />
                  <CopyButton label="ID" text={String(service.id)} />
                  {#if service.template}<CopyButton label="Template" text={service.template} />{/if}
                </div>
              </div>
              {#if environment}
                <div>
                  <h3>Environment</h3>
                  <div class="grid grid-cols-1 gap-3 pt-2 md:grid-cols-2">
                    <CopyButton label="Name" text={environment.name} />
                    <CopyButton label="ID" text={String(environment.id)} />
                  </div>
                </div>
                <div>
                  <h3>Project</h3>
                  <div class="grid grid-cols-1 gap-3 pt-2 md:grid-cols-2">
                    <CopyButton label="Name" text={environment.project_name ?? ''} />
                    <CopyButton label="ID" text={String(environment.project_id)} />
                  </div>
                </div>
              {/if}
            </div>
          </div>
        </Modal>
      </div>
    {/snippet}
    <div class="grid gap-4 lg:grid-cols-2">
      <Input label="Service name" bind:value={name} error={errors.name} required placeholder="My WordPress site" disabled={!canUpdate} />
      <Input label="Description" bind:value={description} error={errors.description} disabled={!canUpdate} />
    </div>
  </SettingsSection>

  {#if variables.length > 0}
    <SettingsSection id="service-configuration-section" title="Service configuration" helper="Template-specific values exposed by this service.">
      <div class="grid gap-4 lg:grid-cols-2" data-testid="service-configuration">
        {#each variables as v (v.name)}
          <div>
            <div class="mb-1.5 flex items-center gap-1.5 text-[12px] font-medium">
              <span>{fieldLabel(v)}</span>
              <Helper helper={`Variable name: ${v.name}`} />
            </div>
            {#if v.hidden}
              <Input disabled value={hiddenValue} aria-label={v.name} />
            {:else if v.magic}
              <CopyButton text={v.value} secret testid={`variable-${v.name}`} />
            {:else}
              <Input
                bind:value={() => values[v.name] ?? '', (x) => (values[v.name] = String(x ?? ''))}
                aria-label={v.name}
                placeholder={v.default ?? ''}
                disabled={!canUpdate}
              />
            {/if}
          </div>
        {/each}
      </div>
      {#if errors.variables}<p class="mt-2 text-xs text-error">{errors.variables}</p>{/if}
    </SettingsSection>
  {/if}
</form>

<div class="chrome mt-8" data-testid="compose-resources">
  <div class="mb-3 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
    <div>
      <h2 class="text-base font-semibold text-black dark:text-fg">Compose resources</h2>
      <p class="mt-1 text-sm text-neutral-500 dark:text-fg-dim">Applications and databases defined in this service.</p>
    </div>
    <div class="flex w-full items-center justify-between gap-2 sm:w-auto sm:justify-start">
      <div class="view-toggle">
        <button
          type="button"
          onclick={() => setViewMode('table')}
          class={['flex size-7.5 items-center justify-center rounded-md transition-colors', viewMode === 'table' ? 'control-selected' : toggleIdle]}
          aria-label="Table view"
          aria-pressed={viewMode === 'table'}
          title="Table view"
        >
          <Icon name="unordered-list" class="size-3.5" />
        </button>
        <button
          type="button"
          onclick={() => setViewMode('grid')}
          class={['flex size-7.5 items-center justify-center rounded-md transition-colors', viewMode === 'grid' ? 'control-selected' : toggleIdle]}
          aria-label="Grid view"
          aria-pressed={viewMode === 'grid'}
          title="Grid view"
        >
          <Icon name="grid" class="size-3.5" />
        </button>
      </div>
      {#if docsUrl}
        <a class="button" target="_blank" rel="noreferrer" href={docsUrl}>
          Documentation
          <Icon name="external-link" class="size-4" />
        </a>
      {/if}
    </div>
  </div>

  <div
    class={viewMode === 'grid'
      ? 'grid grid-cols-1 gap-3 sm:grid-cols-2'
      : 'overflow-x-auto rounded-xl border border-neutral-200 bg-white shadow-sm dark:border-white/[0.08] dark:bg-white/[0.05]'}
  >
    {#if service.components.length === 0}
      <div class="application-settings-section overflow-hidden sm:col-span-2">
        <Empty title="No compose resources" description="No applications or databases are defined in this Docker Compose file." icon="grid" />
      </div>
    {:else if viewMode === 'table'}
      <div
        class={[
          'grid min-w-[48rem] gap-3 border-b border-neutral-200 bg-neutral-50 px-4 py-2.5 text-[11px] font-medium text-neutral-500 dark:border-white/[0.08] dark:bg-white/[0.05] dark:text-fg-faint',
          tableColumns,
        ]}
      >
        <div>Resource</div>
        <div>Image</div>
        <div class="justify-self-start">Status</div>
        <div></div>
      </div>
    {/if}

    {#each service.components as c (c.name)}
      {@const badge = componentStatus(c.status)}
      {#if viewMode === 'grid'}
        <div
          class="group flex min-w-0 flex-col overflow-hidden rounded-[10px] border border-neutral-200 bg-white transition-[border-color,background-color,box-shadow] hover:border-neutral-300 hover:shadow-sm dark:border-white/[0.07] dark:bg-surface dark:hover:border-white/[0.12] dark:hover:bg-white/[0.035]"
          data-testid="compose-resource"
        >
          <div class="flex min-w-0 flex-1 items-start gap-3 p-4">
            <div class={['size-9', iconBadge]}><Icon name="grid" class="size-4" /></div>
            <div class="min-w-0 flex-1">
              <div class="flex min-w-0 items-start justify-between gap-3">
                <div class="min-w-0">
                  <div class="truncate text-sm font-semibold text-black dark:text-fg">{headline(c.name)}</div>
                  <div class="mt-0.5 truncate font-mono text-xs text-neutral-500 dark:text-fg-faint">{c.image}</div>
                </div>
                <StatusBadge status={badge.status} type={badge.type} />
              </div>
              {#if c.detail}
                <p class="mt-2 line-clamp-2 text-xs leading-5 text-neutral-500 dark:text-fg-dim">{c.detail}</p>
              {/if}
            </div>
          </div>
          <div
            class="flex items-center justify-end gap-1 border-t border-neutral-200 bg-neutral-50 px-3 py-2 dark:border-white/[0.06] dark:bg-white/[0.02]"
          >
            {@render componentActions(c)}
          </div>
        </div>
      {:else}
        <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
        <div
          role="link"
          tabindex="0"
          aria-label={`Open ${headline(c.name)} settings`}
          onclick={(e) => {
            if (!(e.target as HTMLElement).closest('a, button')) openComponent = c.name
          }}
          onkeydown={(e) => {
            if (e.key === 'Enter' && !(e.target as HTMLElement).closest('a, button')) openComponent = c.name
          }}
          class={[
            'grid min-h-14 min-w-[48rem] cursor-pointer items-center gap-3 border-b border-neutral-200 px-4 py-2.5 last:border-b-0 hover:bg-neutral-50 focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-primary dark:border-white/[0.07] dark:hover:bg-white/[0.025]',
            tableColumns,
          ]}
          data-testid="compose-resource"
        >
          <div class="flex min-w-0 items-center gap-3">
            <div class={['size-8', iconBadge]}><Icon name="grid" class="size-4" /></div>
            <div class="min-w-0">
              <div class="truncate text-[13px] font-semibold text-black dark:text-fg">{headline(c.name)}</div>
              {#if c.detail}<div class="truncate text-xs text-neutral-500 dark:text-fg-dim">{c.detail}</div>{/if}
            </div>
          </div>
          <div class="truncate font-mono text-xs text-neutral-500 dark:text-fg-faint">{c.image}</div>
          <div class="flex flex-wrap items-center gap-1"><StatusBadge status={badge.status} type={badge.type} /></div>
          <div class="flex items-center justify-end gap-1">{@render componentActions(c)}</div>
        </div>
      {/if}
    {/each}
  </div>
</div>

{#each service.components as c (c.name)}
  {@const badge = componentStatus(c.status)}
  <Modal
    title={headline(c.name)}
    subtitle="Identity, image, and public access for this compose application."
    variant="none"
    isLarge
    bind:open={() => openComponent === c.name, (v) => (openComponent = v ? c.name : null)}
  >
    <div class="flex flex-col gap-4" data-testid="component-modal">
      <div class="grid gap-4 lg:grid-cols-2">
        <CopyButton label="Image" text={c.image} />
        <div>
          <span class="mb-1 flex items-center gap-1 text-sm font-medium text-black dark:text-white">Status</span>
          <StatusBadge status={badge.status} type={badge.type} />
        </div>
      </div>
      <p class="text-xs text-neutral-500 dark:text-fg-dim">The image is set in the Compose file; change it with Edit Compose file.</p>
      {#if c.public}
        <div class="flex flex-wrap items-center justify-between gap-2">
          <span class="text-sm text-neutral-600 dark:text-fg-dim">{c.domains.length} {c.domains.length === 1 ? 'domain' : 'domains'} set.</span>
          <a class="button" href={domainsHref} onclick={() => (openComponent = null)}>Manage domains</a>
        </div>
      {:else}
        <p class="text-sm text-neutral-600 dark:text-fg-dim">
          Only reachable by the other components of this service, as <span class="font-mono">{c.name}</span>.
        </p>
      {/if}
    </div>
  </Modal>
{/each}
