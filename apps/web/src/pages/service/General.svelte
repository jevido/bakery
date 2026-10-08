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
  import { buttonVariants } from '@bakery/ui/components/ui/button'
  import { Card } from '@bakery/ui/components/ui/card'
  import Icon from '../../lib/Icon.svelte'
  import { href, servicePath } from '../../lib/router.svelte'
  import { projectAccess } from '../../lib/projectAccess.svelte'
  import SettingsGroup from '../../lib/settings/SettingsGroup.svelte'
  import type { Component, Environment, Service, ServiceTemplate } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import CopyButton from '../../lib/ui/CopyButton.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import EntityRow from '@bakery/ui/EntityRow.svelte'
  import Helper from '../../lib/ui/Helper.svelte'
  import Input from '../../lib/ui/Input.svelte'
  import Modal from '../../lib/ui/Modal.svelte'
  import StatusBadge, { type StatusType } from '@bakery/ui/StatusBadge.svelte'
  import Textarea from '../../lib/ui/Textarea.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import UnsavedBar from '../../lib/ui/UnsavedBar.svelte'
  import ViewToggle from '../../lib/ViewToggle.svelte'

  let {
    service,
    environment,
    onchange,
  }: { service: Service; environment: Environment | null; onchange: (s: Service) => void } = $props()

  const hiddenValue = 'Hidden (viewers cannot see it)'
  const appliesOnRestart = 'The change applies on the next Restart.'
  const canUpdate = $derived(projectAccess.can('manage_applications'))

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
  const iconBadge = 'flex shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground'
</script>

{#snippet componentActions(c: Component)}
  {#if c.public && c.domains.length > 0 && canUpdate}
    <a
      class={buttonVariants({ variant: 'ghost', size: 'icon-sm', class: 'size-8 text-muted-foreground' })}
      title="Manage domains"
      aria-label="Manage domains"
      href={domainsHref}
    >
      <Icon name="globe" class="size-4" />
    </a>
  {/if}
  <button
    type="button"
    class={buttonVariants({ variant: 'ghost', size: 'icon-sm', class: 'size-8 text-muted-foreground' })}
    title="Resource settings"
    aria-label="Resource settings"
    onclick={() => (openComponent = c.name)}
  >
    <Icon name="settings" class="size-4" />
  </button>
{/snippet}

<form
  class="chrome flex flex-col gap-6"
  onsubmit={(e) => {
    e.preventDefault()
    save()
  }}
>
  {#if canUpdate}
    <UnsavedBar {dirty} {saving} onsave={save} onreset={reset} />
  {/if}

  <SettingsGroup id="service-details-section" label="Service details" hint="Manage the identity and Compose configuration for this service.">
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
  </SettingsGroup>

  {#if variables.length > 0}
    <SettingsGroup id="service-configuration-section" label="Service configuration" hint="Template-specific values exposed by this service.">
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
    </SettingsGroup>
  {/if}
</form>

<div class="chrome mt-8 max-w-4xl" data-testid="compose-resources">
  <div class="mb-3 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
    <div>
      <h2 class="text-sm font-medium text-foreground">Compose resources</h2>
      <p class="mt-1 text-xs text-muted-foreground">Applications and databases defined in this service.</p>
    </div>
    <div class="flex w-full items-center justify-between gap-2 sm:w-auto sm:justify-start">
      <ViewToggle value={viewMode} onchange={setViewMode} />
      {#if docsUrl}
        <a class={buttonVariants({ variant: 'outline', size: 'sm' })} target="_blank" rel="noreferrer" href={docsUrl}>
          Documentation
          <Icon name="external-link" class="size-3.5" />
        </a>
      {/if}
    </div>
  </div>

  {#if service.components.length === 0}
    <Card class="block py-0">
      <Empty title="No compose resources" description="No applications or databases are defined in this Docker Compose file." icon="grid" />
    </Card>
  {:else if viewMode === 'table'}
    <Card class="block gap-0 overflow-hidden py-0" data-testid="compose-resource-list">
      {#each service.components as c (c.name)}
        {@const badge = componentStatus(c.status)}
        <EntityRow
          title={headline(c.name)}
          subtitle={c.detail}
          reserveSubtitleSpace
          onclick={() => (openComponent = c.name)}
          data-testid="compose-resource"
        >
          {#snippet leading()}<div class={['size-8', iconBadge]}><Icon name="grid" class="size-4" /></div>{/snippet}
          {#snippet trailing()}
            <span class="hidden truncate font-mono text-xs text-muted-foreground sm:inline">{c.image}</span>
            <StatusBadge status={badge.status} type={badge.type} />
            {@render componentActions(c)}
          {/snippet}
        </EntityRow>
      {/each}
    </Card>
  {:else}
    <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
      {#each service.components as c (c.name)}
        {@const badge = componentStatus(c.status)}
        <Card class="gap-0 overflow-hidden py-0" data-testid="compose-resource">
          <div class="flex min-w-0 items-start gap-3 p-4">
            <div class={['size-9', iconBadge]}><Icon name="grid" class="size-4" /></div>
            <div class="min-w-0 flex-1">
              <div class="flex min-w-0 items-start justify-between gap-3">
                <div class="min-w-0">
                  <div class="truncate text-sm font-semibold text-foreground">{headline(c.name)}</div>
                  <div class="mt-0.5 truncate font-mono text-xs text-muted-foreground">{c.image}</div>
                </div>
                <StatusBadge status={badge.status} type={badge.type} />
              </div>
              {#if c.detail}
                <p class="mt-2 line-clamp-2 text-xs leading-5 text-muted-foreground">{c.detail}</p>
              {/if}
            </div>
          </div>
          <div class="flex items-center justify-end gap-1 border-t border-border bg-muted/30 px-3 py-2">
            {@render componentActions(c)}
          </div>
        </Card>
      {/each}
    </div>
  {/if}
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
          <span class="mb-1 flex items-center gap-1 text-sm font-medium text-foreground">Status</span>
          <StatusBadge status={badge.status} type={badge.type} />
        </div>
      </div>
      <p class="text-xs text-muted-foreground">The image is set in the Compose file; change it with Edit Compose file.</p>
      {#if c.public}
        <div class="flex flex-wrap items-center justify-between gap-2">
          <span class="text-sm text-muted-foreground">{c.domains.length} {c.domains.length === 1 ? 'domain' : 'domains'} set.</span>
          <a class={buttonVariants({ size: 'sm' })} href={domainsHref} onclick={() => (openComponent = null)}>Manage domains</a>
        </div>
      {:else}
        <p class="text-sm text-muted-foreground">
          Only reachable by the other components of this service, as <span class="font-mono">{c.name}</span>.
        </p>
      {/if}
    </div>
  </Modal>
{/each}
