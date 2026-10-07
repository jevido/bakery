<script lang="ts">
  // Coolify's Service Domains page (resources/views/livewire/project/service/domains.blade.php,
  // partials/domain-table.blade.php, edit-domain.blade.php and
  // app/Livewire/Project/Service/Domains.php, Apache-2.0, see NOTICE): the
  // toolbar with search and Add domain, one table per public Component and one
  // Domain settings dialog per edit. Each save sends the Component's whole
  // list; the Proxy serves it at once, without a deploy.
  // Left out, as The Bakery has nothing behind them: DNS checks, Cloudflare,
  // suggested Domains, the required-port warning, the HTTP → HTTPS setting,
  // search engine indexing and the www redirect (a Domain is a hostname
  // served over HTTPS on the Component's port).
  import { api, ApiError } from '../../lib/api'
  import Icon from '../../lib/Icon.svelte'
  import { projectAccess } from '../../lib/projectAccess.svelte'
  import type { Component, Service } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import Callout from '../../lib/ui/Callout.svelte'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import Input from '../../lib/ui/Input.svelte'
  import Modal from '../../lib/ui/Modal.svelte'
  import Select from '../../lib/ui/Select.svelte'
  import { toast } from '../../lib/ui/toast.svelte'

  let { service, onchange }: { service: Service; onchange: (s: Service) => void } = $props()

  const canUpdate = $derived(projectAccess.can('manage_applications'))
  const publicComponents = $derived(service.components.filter((c) => c.public))
  const privateComponents = $derived(service.components.filter((c) => !c.public))
  const domainCount = $derived(publicComponents.reduce((n, c) => n + c.domains.length, 0))

  let search = $state('')

  let adding = $state(false)
  let newComponent = $state('')
  let newHost = $state('')
  let addError = $state('')

  let editing = $state(false)
  let editingComponent = $state('')
  let editingIndex = $state(-1)
  let editingHost = $state('')
  let editError = $state('')

  let saving = $state(false)

  function headline(s: string): string {
    return s
      .replace(/[_-]+/g, ' ')
      .trim()
      .split(/\s+/)
      .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
      .join(' ')
  }

  const groupMatches = (c: Component) => {
    const q = search.trim().toLowerCase()
    return !q || `${headline(c.name)} ${c.domains.join(' ')}`.toLowerCase().includes(q)
  }
  const anyMatch = $derived(publicComponents.some(groupMatches))

  const component = (name: string) => publicComponents.find((c) => c.name === name)

  // Saves a Component's list; the API's message, when it refuses, is returned.
  async function saveDomains(name: string, list: string[]): Promise<string> {
    try {
      onchange((await api<{ service: Service }>('PATCH', `/services/${service.id}`, { domains: { [name]: list } })).service)
      return ''
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      return err.errors.domains ?? err.message
    }
  }

  // The generated Domain is the Component's own default, so Generate only
  // offers it while no Domain of the Service has it already.
  function generated(name: string, except = ''): string {
    const g = component(name)?.generated_domain ?? ''
    return g && !service.components.some((c) => c.domains.some((d) => d === g && d !== except)) ? g : ''
  }

  const regenerated = $derived(editing ? generated(editingComponent, component(editingComponent)?.domains[editingIndex]) : '')

  function openAdd() {
    newComponent = publicComponents[0]?.name ?? ''
    newHost = ''
    addError = ''
    adding = true
  }

  async function add(e: SubmitEvent) {
    e.preventDefault()
    const c = component(newComponent)
    if (saving || !c) return
    saving = true
    addError = await saveDomains(c.name, [...c.domains, newHost.trim()])
    saving = false
    if (addError) return
    adding = false
    toast.success('Domain added.')
  }

  function openEdit(name: string, index: number) {
    editingComponent = name
    editingIndex = index
    editingHost = component(name)?.domains[index] ?? ''
    editError = ''
    editing = true
  }

  async function update(e: SubmitEvent) {
    e.preventDefault()
    const c = component(editingComponent)
    if (saving || !c) return
    const host = editingHost.trim().toLowerCase()
    if (host !== c.domains[editingIndex]) {
      saving = true
      editError = await saveDomains(c.name, c.domains.map((d, i) => (i === editingIndex ? host : d)))
      saving = false
      if (editError) return
    }
    editing = false
    toast.success('Domain updated.')
  }

  async function remove(name: string, index: number) {
    const c = component(name)
    if (!c) return
    const message = await saveDomains(name, c.domains.filter((_, i) => i !== index))
    if (message) {
      toast.error('Domain not removed', message)
      throw new Error(message)
    }
    toast.success('Domain removed.')
  }
</script>

<div id="service-domains-section" class="domains-overview-container chrome flex flex-col gap-4">
  {#if !canUpdate}
    <Callout type="danger" title="Insufficient permissions">
      You don't have permission to manage domains. Contact your guild's admin for access.
    </Callout>
  {/if}

  <div class="flex flex-wrap items-center gap-2">
    <div class="min-w-0 flex-1">
      <h2 id="domains-section">Domains</h2>
      <p class="text-[13px] text-neutral-500 dark:text-fg-dim">
        {domainCount} domain{domainCount === 1 ? '' : 's'} across {publicComponents.length} service{publicComponents.length === 1 ? '' : 's'}
      </p>
    </div>
    <div class="ml-auto flex flex-wrap items-center gap-2">
      {#if publicComponents.length > 0}
        <div class="relative w-full sm:w-64">
          <Icon
            name="search"
            class="pointer-events-none absolute top-1/2 left-2.5 z-10 size-3.5 -translate-y-1/2 text-neutral-400 dark:text-fg-faint"
          />
          <input
            type="search"
            bind:value={search}
            aria-label="Search services or domains"
            class="input h-8! w-full pl-8! text-[13px]!"
            placeholder="Search services or domains"
          />
        </div>
        {#if canUpdate}
          <button type="button" class="button button-highlighted" onclick={openAdd}>
            <Icon name="plus" class="size-3.5" />
            Add domain
          </button>
        {/if}
      {/if}
    </div>
  </div>

  {#if publicComponents.length === 0}
    <div class="application-settings-section-body mt-1 w-full scroll-mt-28">
      <Empty
        size="sm"
        title="No application services"
        description="No component of this service is public. Domains can only be assigned to a component with SERVICE_FQDN_<NAME>_<PORT> in the compose file."
        icon="globe"
      />
    </div>
  {:else}
    <div class="flex flex-col gap-3">
      {#each publicComponents as c (c.name)}
        {#if groupMatches(c)}
          <section id={`service-domain-group-${c.name}`} class="application-settings-section-body is-flush overflow-visible">
            <div
              class="flex w-full flex-wrap items-center gap-3 rounded-t-lg border-b border-neutral-200 bg-neutral-50 px-4 py-3 dark:border-white/10 dark:bg-white/[0.04]"
            >
              <span class="min-w-0 flex-1 truncate text-sm font-medium text-black dark:text-white">{headline(c.name)}</span>
            </div>
            <div class="data-table w-full">
              <div class="data-table-header service-domains-overview-grid is-service">
                <span>Domain</span>
                <span>Internal port</span>
                <span class="text-right">Actions</span>
              </div>
              {#each c.domains as d, index (d)}
                <div class="env-table-item">
                  <div class="data-table-row service-domains-overview-grid is-service">
                    <div class="flex min-w-0 items-center gap-2">
                      <Icon name="globe" class="size-4 shrink-0 text-neutral-400 dark:text-fg-faint" />
                      <a
                        href={`https://${d}`}
                        target="_blank"
                        rel="noopener noreferrer"
                        class="min-w-0 flex-1 truncate text-[13px] text-black underline decoration-neutral-300 underline-offset-2 hover:decoration-coollabs dark:text-fg dark:decoration-white/20"
                        title={`https://${d}`}
                      >
                        https://{d}
                      </a>
                      {#if index === 0}<span class="table-badge shrink-0" title="The primary domain">Primary</span>{/if}
                    </div>

                    <div class="service-domain-detail" title="The component's port">
                      <span aria-label={`Internal port ${c.port}`}>{c.port}</span>
                    </div>

                    <div class="service-domain-mobile-summary" aria-label="Domain routing summary">
                      <span>Port {c.port}</span>
                    </div>

                    <div class="service-domain-actions flex items-center justify-end gap-1">
                      {#if canUpdate}
                        <button
                          type="button"
                          class="icon-button shrink-0"
                          title="Domain settings"
                          aria-label={`Settings for ${d}`}
                          onclick={() => openEdit(c.name, index)}
                        >
                          <Icon name="settings" class="size-3.5" />
                        </button>
                        {#if c.domains.length > 1}
                          <ConfirmationModal
                            title="Remove domain?"
                            buttonTitle="Remove"
                            variant="error"
                            actions={['This domain will be removed from the service application.', 'The proxy stops serving it at once.']}
                            confirmWithText={false}
                            step2ButtonText="Remove domain"
                            onconfirm={() => remove(c.name, index)}
                          >
                            {#snippet trigger(show)}
                              <button
                                type="button"
                                class="icon-button shrink-0 text-red-500 hover:text-red-600 dark:text-red-400 dark:hover:text-red-300"
                                title="Remove domain"
                                aria-label={`Remove ${d}`}
                                onclick={show}
                              >
                                <Icon name="trash" class="size-3.5" />
                              </button>
                            {/snippet}
                          </ConfirmationModal>
                        {:else}
                          <!-- A public Component always has a Domain: the Proxy routes it by one. -->
                          <button
                            type="button"
                            class="icon-button shrink-0 text-red-500 dark:text-red-400"
                            title="A public component keeps at least one domain; change this one instead"
                            aria-label={`Remove ${d}`}
                            disabled
                          >
                            <Icon name="trash" class="size-3.5" />
                          </button>
                        {/if}
                      {/if}
                    </div>
                  </div>
                </div>
              {/each}
            </div>
          </section>
        {/if}
      {/each}
      {#if !anyMatch}
        <div class="px-4 py-8">
          <Empty size="sm" title="No domains found" description="No service or domain matches your search." icon="search" />
        </div>
      {/if}
    </div>
  {/if}

  {#each privateComponents as c (c.name)}
    <p class="text-[13px] text-neutral-500 dark:text-fg-dim">
      {headline(c.name)}: only reachable by the other components of this service, as <span class="font-mono">{c.name}</span>.
    </p>
  {/each}
</div>

{#if canUpdate}
  <Modal title="Add domain" variant="none" closeOutside={false} bind:open={adding}>
    <form class="application-settings-form flex flex-col gap-4" onsubmit={add}>
      <Select
        label="Service application"
        helper="Domain will be assigned to this compose service application."
        required
        bind:value={newComponent}
      >
        {#each publicComponents as c (c.name)}
          <option value={c.name}>{c.name}{c.image ? ` (${c.image})` : ''}</option>
        {/each}
      </Select>
      <Input
        label="Domain"
        bind:value={newHost}
        placeholder="app.example.com"
        required
        error={addError}
        helper="A hostname without protocol or path; The Bakery serves it over HTTPS and gets its certificate."
      />
      <div class="flex flex-wrap items-center justify-between gap-2 pt-2">
        {#if generated(newComponent)}
          <Button onclick={() => (newHost = generated(newComponent))}>Generate domain</Button>
        {:else}
          <span></span>
        {/if}
        <Button type="submit" variant="highlighted" loading={saving}>Save</Button>
      </div>
    </form>
  </Modal>

  <Modal title="Domain settings" variant="none" bind:open={editing}>
    <form class="application-settings-form flex flex-col gap-4" onsubmit={update}>
      <div class="w-full">
        <Input label="Service application" value={editingComponent} readonly />
        <p class="mt-1 text-[12px] text-neutral-500 dark:text-fg-dim">
          Domains stay on the service they were added to. Remove and re-add to move.
        </p>
      </div>
      <Input label="Domain" bind:value={editingHost} placeholder="app.example.com" required error={editError} />
      <div class="flex flex-wrap items-center justify-between gap-2 border-t border-neutral-200 pt-4 dark:border-white/10">
        {#if regenerated}
          <Button onclick={() => (editingHost = regenerated)}>Regenerate hostname</Button>
        {:else}
          <span></span>
        {/if}
        <Button type="submit" variant="highlighted" loading={saving}>Save</Button>
      </div>
    </form>
  </Modal>
{/if}
