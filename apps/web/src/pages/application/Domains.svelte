<script lang="ts">
  // Coolify's Domains page (resources/views/livewire/project/application/domains.blade.php,
  // partials/domain-row.blade.php and app/Livewire/Project/Application/Domains.php,
  // Apache-2.0, see NOTICE): the toolbar with search and Add domain, the
  // table of domains and one Domain settings dialog per edit, with the www
  // redirect. Domains save through the Application, the redirect through the
  // routing settings; both apply at once, without a deploy.
  // Left out, as The Bakery has nothing behind them: DNS checks, Cloudflare,
  // the HTTP → HTTPS setting, search engine indexing, and the protocol, port
  // and path of a domain (a Domain is a hostname served over HTTPS).
  import { untrack } from 'svelte'
  import { api, ApiError } from '../../lib/api'
  import { buttonVariants } from '@bakery/ui/components/ui/button'
  import Icon from '../../lib/Icon.svelte'
  import { projectAccess } from '../../lib/projectAccess.svelte'
  import type { Application, Redirect, RouteSettings, Server } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import Callout from '../../lib/ui/Callout.svelte'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import Input from '../../lib/ui/Input.svelte'
  import Modal from '../../lib/ui/Modal.svelte'
  import SearchField from '../../lib/SearchField.svelte'
  import Select from '../../lib/ui/Select.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import { applicationInput } from './applicationInput'

  let {
    application,
    server,
    onchange,
  }: { application: Application; server: Server | null; onchange: (a: Application) => void } = $props()

  const canUpdate = $derived(projectAccess.can('manage_applications'))
  const domains = $derived(application.domains)

  let routing = $state.raw<RouteSettings | null>(null)
  let search = $state('')

  let adding = $state(false)
  let newHost = $state('')
  let addError = $state('')

  let editing = $state(false)
  let editingIndex = $state(-1)
  let editingHost = $state('')
  let editingRedirect = $state<Redirect>('both')
  let editError = $state('')

  let saving = $state(false)

  $effect(() => {
    const id = application.id
    untrack(() => (routing = null))
    api<{ routing: RouteSettings }>('GET', `/applications/${id}/routing`)
      .then((r) => (routing = r.routing))
      .catch((e) => toast.error('Proxy settings not loaded', e.message))
  })

  const redirect = $derived<Redirect>(routing?.redirect ?? 'both')

  // What a Redirect adds for each Domain, as the proxy renders it: a
  // counterpart that is one of the Domains itself is served, not redirected.
  function counterparts(list: string[], mode: Redirect): Map<string, string> {
    const out = new Map<string, string>()
    for (const d of list) {
      let c = ''
      if (mode === 'non-www' && !d.startsWith('www.')) c = 'www.' + d
      if (mode === 'www' && d.startsWith('www.')) c = d.slice(4)
      if (c && c.includes('.') && !list.includes(c)) out.set(d, c)
    }
    return out
  }

  const rowCounterparts = $derived(counterparts(domains, redirect))
  const editingCounterparts = $derived(
    editing
      ? [...counterparts(domains.map((d, i) => (i === editingIndex ? editingHost.trim().toLowerCase() : d)), editingRedirect)]
      : [],
  )

  const directionLabel = (mode: Redirect) => (mode === 'www' ? 'non-www → www' : 'www → non-www')

  const matches = (d: string) => !search.trim() || d.includes(search.trim().toLowerCase())
  const anyMatch = $derived(domains.some(matches))

  // Saves the list; the API's message, when it refuses, is returned.
  async function saveDomains(list: string[]): Promise<string> {
    try {
      onchange((await api<{ application: Application }>('PATCH', `/applications/${application.id}`, { ...applicationInput(application), domains: list })).application)
      return ''
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      return err.errors.domains ?? err.message
    }
  }

  function openAdd() {
    newHost = ''
    addError = ''
    adding = true
  }

  async function add(e: SubmitEvent) {
    e.preventDefault()
    if (saving) return
    saving = true
    addError = await saveDomains([...domains, newHost.trim()])
    saving = false
    if (addError) return
    adding = false
    toast.success('Domain added.')
  }

  function openEdit(index: number) {
    editingIndex = index
    editingHost = domains[index]
    editingRedirect = redirect
    editError = ''
    editing = true
  }

  async function update(e: SubmitEvent) {
    e.preventDefault()
    if (saving || !routing) return
    saving = true
    editError = ''
    try {
      const host = editingHost.trim().toLowerCase()
      if (host !== domains[editingIndex]) {
        editError = await saveDomains(domains.map((d, i) => (i === editingIndex ? host : d)))
        if (editError) return
      }
      if (editingRedirect !== routing.redirect) {
        try {
          routing = (
            await api<{ routing: RouteSettings }>('PUT', `/applications/${application.id}/routing`, {
              redirect: editingRedirect,
              response_headers: routing.response_headers,
              // An empty password keeps the stored one.
              basic_auth: { enabled: routing.basic_auth.enabled, username: routing.basic_auth.username, password: '' },
            })
          ).routing
        } catch (err) {
          if (!(err instanceof ApiError)) throw err
          editError = err.errors.redirect ?? err.message
          return
        }
      }
      editing = false
      toast.success('Domain updated.')
    } finally {
      saving = false
    }
  }

  async function remove(index: number) {
    const message = await saveDomains(domains.filter((_, i) => i !== index))
    if (message) {
      toast.error('Domain not removed', message)
      throw new Error(message)
    }
    toast.success('Domain removed.')
  }

  // The Bakery's generated domain is the Application's own default, so
  // Generate only offers it while no row has it already.
  const canGenerate = (except = -1) => !domains.some((d, i) => i !== except && d === application.generated_domain)
</script>

<div id="application-domains-section" class="flex flex-col gap-4">
  {#if !canUpdate}
    <Callout type="danger" title="Insufficient permissions">
      You don't have permission to manage domains. Contact your guild's admin for access.
    </Callout>
  {/if}

  {#if server?.kind === 'remote'}
    <Callout type="info" title="Remote server">
      This application runs on {server.name}. Its domains must point at {server.host}, where that server's proxy serves them.
    </Callout>
  {/if}

  <div class="flex flex-wrap items-end gap-2">
    <div class="min-w-0 flex-1">
      <h2 id="domains-section" class="text-xs font-medium tracking-wide text-muted-foreground uppercase">Domains</h2>
      <p class="mt-1 text-sm text-muted-foreground">
        {domains.length} domain{domains.length === 1 ? '' : 's'}
      </p>
    </div>
    <div class="ml-auto flex w-full flex-wrap items-center gap-2 sm:w-auto">
      <div class="min-w-0 flex-1 sm:w-64 sm:flex-none">
        <SearchField bind:value={search} label="Search domains" />
      </div>
      {#if canUpdate}
        <Button variant="highlighted" onclick={openAdd}>
          <Icon name="plus" class="size-3.5" />
          Add domain
        </Button>
      {/if}
    </div>
  </div>

  <div id="domains-table-section" class="scroll-mt-28 overflow-hidden rounded-md border border-border">
    <div
      class="hidden grid-cols-[minmax(0,1fr)_10rem_7rem_5rem] gap-3 border-b border-border bg-muted/30 px-4 py-2 text-xs font-medium text-muted-foreground md:grid"
    >
      <span>Domain</span>
      <span>Domain redirect</span>
      <span>Internal port</span>
      <span class="text-right">Actions</span>
    </div>
    {#each domains as d, index (d)}
      {@const counterpart = rowCounterparts.get(d)}
      {#if matches(d)}
        <div
          class="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3 gap-y-1 border-b border-border px-4 py-2 text-sm transition-colors last:border-b-0 hover:bg-accent/50 md:grid-cols-[minmax(0,1fr)_10rem_7rem_5rem]"
        >
          <div class="flex min-w-0 items-center gap-2">
            <Icon name="globe" class="size-4 shrink-0 text-muted-foreground" />
            <a
              href={application.public_urls[index]}
              target="_blank"
              rel="noopener noreferrer"
              class="min-w-0 flex-1 truncate text-foreground underline decoration-border underline-offset-2 hover:decoration-foreground"
              title={application.public_urls[index]}
            >
              {application.public_urls[index]}
            </a>
            {#if index === 0}
              <span class="shrink-0 rounded-full border border-border bg-muted/50 px-2 py-0.5 text-xs text-muted-foreground" title="The primary domain">
                Primary
              </span>
            {/if}
          </div>

          <div class="hidden truncate text-muted-foreground md:block" title={counterpart ? `${counterpart} → ${d}` : 'Domain redirect'}>
            <span>{counterpart ? directionLabel(redirect) : 'Disabled'}</span>
          </div>
          <div class="hidden font-mono text-xs text-muted-foreground md:block" title="The application's port">
            <span aria-label={`Internal port ${application.port}`}>{application.port}</span>
          </div>

          <div class="flex items-center justify-end gap-1 md:order-last">
            {#if canUpdate}
              <button
                type="button"
                class={buttonVariants({ variant: 'ghost', size: 'icon-sm', class: 'size-8 text-muted-foreground' })}
                title="Domain settings"
                aria-label={`Settings for ${d}`}
                onclick={() => openEdit(index)}
              >
                <Icon name="settings" class="size-3.5" />
              </button>
              {#if domains.length > 1}
                <ConfirmationModal
                  title="Remove domain?"
                  buttonTitle="Remove"
                  variant="error"
                  actions={['This domain will be removed from the application.', 'The proxy stops serving it at once.']}
                  confirmWithText={false}
                  step2ButtonText="Remove domain"
                  onconfirm={() => remove(index)}
                >
                  {#snippet trigger(show)}
                    <button
                      type="button"
                      class={buttonVariants({ variant: 'ghost', size: 'icon-sm', class: 'size-8 text-destructive hover:text-destructive' })}
                      title="Remove domain"
                      aria-label={`Remove ${d}`}
                      onclick={show}
                    >
                      <Icon name="trash" class="size-3.5" />
                    </button>
                  {/snippet}
                </ConfirmationModal>
              {:else}
                <!-- An Application always has a Domain: without one it would get the generated one back. -->
                <button
                  type="button"
                  class={buttonVariants({ variant: 'ghost', size: 'icon-sm', class: 'size-8 text-destructive' })}
                  title="An application keeps at least one domain; change this one instead"
                  aria-label={`Remove ${d}`}
                  disabled
                >
                  <Icon name="trash" class="size-3.5" />
                </button>
              {/if}
            {/if}
          </div>

          <div class="col-span-2 flex gap-3 text-xs text-muted-foreground md:hidden" aria-label="Domain routing summary">
            <span>{counterpart ? directionLabel(redirect) : 'No redirects'}</span>
            <span>Port {application.port}</span>
          </div>
        </div>
      {/if}
    {/each}
    {#if !anyMatch}
      <div class="px-4 py-8">
        <Empty size="sm" title="No domains found" description="No domain matches your search." icon="search" />
      </div>
    {/if}
  </div>
</div>

{#if canUpdate}
  <Modal title="Add domain" variant="none" closeOutside={false} bind:open={adding}>
    <form class="flex flex-col gap-4" onsubmit={add}>
      <Input
        label="Domain"
        bind:value={newHost}
        placeholder="app.example.com"
        required
        error={addError}
        helper="A hostname without protocol or path; The Bakery serves it over HTTPS and gets its certificate."
      />
      <div class="flex flex-wrap items-center justify-between gap-2 pt-2">
        {#if canGenerate()}
          <Button onclick={() => (newHost = application.generated_domain)}>Generate domain</Button>
        {:else}
          <span></span>
        {/if}
        <Button type="submit" variant="highlighted" loading={saving}>Save</Button>
      </div>
    </form>
  </Modal>

  <Modal title="Domain settings" variant="none" bind:open={editing}>
    <form class="flex flex-col gap-4" onsubmit={update}>
      <Input label="Domain" bind:value={editingHost} placeholder="app.example.com" required error={editError} />
      <div class="flex flex-col gap-4 border-t border-border pt-4">
        <Select label="www redirect" helper="Applies to all domains for this application." bind:value={editingRedirect}>
          <option value="both">No redirect</option>
          <option value="www">Redirect to www</option>
          <option value="non-www">Redirect to non-www</option>
        </Select>
        {#if editingCounterparts.length > 0}
          <ul class="flex flex-col gap-1 font-mono text-xs text-muted-foreground" aria-label="Redirects added">
            {#each editingCounterparts as [to, from] (from)}<li>{from} → {to}</li>{/each}
          </ul>
        {:else if editingRedirect !== 'both'}
          <p class="text-xs text-muted-foreground">No domain has a counterpart to redirect for this choice.</p>
        {/if}
      </div>
      <div class="flex flex-wrap items-center justify-between gap-2 border-t border-border pt-4">
        {#if canGenerate(editingIndex)}
          <Button onclick={() => (editingHost = application.generated_domain)}>Regenerate hostname</Button>
        {:else}
          <span></span>
        {/if}
        <Button type="submit" variant="highlighted" loading={saving} disabled={!routing}>Save</Button>
      </div>
    </form>
  </Modal>
{/if}
