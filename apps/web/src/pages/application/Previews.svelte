<script lang="ts">
  // Coolify's Preview Deployments (resources/views/livewire/project/application/previews.blade.php,
  // preview/form.blade.php and preview-domains.blade.php, with
  // app/Livewire/Project/Application/Previews.php; Apache-2.0, see NOTICE):
  // Preview settings with Enable/Disable preview deployments, the URL
  // template and each Preview with its links, logs and actions. The Bakery
  // learns of pull requests from the webhook rather than loading them from
  // GitHub, so the Previews it lists are those the webhook opened. Its URL
  // template is fixed (pr-<number>.<domain>), forks never get a Preview, and
  // a Preview is redeployed or deleted but not stopped or rebuilt.
  import { api, ApiError } from '../../lib/api'
  import Icon from '../../lib/Icon.svelte'
  import { applicationPath, href } from '../../lib/router.svelte'
  import { projectAccess } from '../../lib/projectAccess.svelte'
  import type { Application, Preview, Webhook } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import Input from '../../lib/ui/Input.svelte'
  import SettingsSection from '../../lib/ui/SettingsSection.svelte'
  import StatusBadge from '../../lib/ui/StatusBadge.svelte'
  import TableDropdown from '../../lib/ui/TableDropdown.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import { deploymentStatus } from './DeploymentHistory.svelte'

  let { application, onchange }: { application: Application; onchange?: () => void } = $props()

  let previews = $state.raw<Preview[]>([])
  let loaded = $state(false)
  let webhook = $state.raw<Webhook | null>(null)
  let toggling = $state(false)
  let token = $state('')
  let savingToken = $state(false)

  const open = $derived(previews.filter((p) => p.state === 'open'))
  const deploying = $derived(previews.some((p) => p.latest_deployment?.active))

  async function load() {
    previews = (await api<{ previews: Preview[] }>('GET', `/applications/${application.id}/previews`)).previews
    loaded = true
  }

  $effect(() => {
    previews = []
    loaded = false
    webhook = null
    load().catch((err) => toast.error('Previews not loaded', err.message))
    // The switch and token live on the Webhook, which carries its secret.
    if (projectAccess.can('see_secrets')) {
      api<{ webhook: Webhook }>('GET', `/applications/${application.id}/webhook`)
        .then((r) => (webhook = r.webhook))
        .catch(() => {})
    }
  })

  // Quickly while a Preview deploys, slower so one opened by a pull request
  // shows up by itself.
  $effect(() => {
    const t = setInterval(() => load().catch(() => {}), deploying ? 3000 : 5000)
    return () => clearInterval(t)
  })

  async function patchWebhook(body: { previews?: boolean; git_host_token?: string }) {
    webhook = (await api<{ webhook: Webhook }>('PATCH', `/applications/${application.id}/webhook`, body)).webhook
  }

  async function togglePreviewDeployments() {
    if (!webhook) return
    toggling = true
    try {
      await patchWebhook({ previews: !webhook.previews })
      toast.success(webhook.previews ? 'Preview deployments enabled.' : 'Preview deployments disabled.')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      toast.error('Preview deployments not changed', err.message)
    } finally {
      toggling = false
    }
  }

  async function saveToken(value: string) {
    savingToken = true
    try {
      await patchWebhook({ git_host_token: value })
      token = ''
      toast.success(value ? 'Git host token saved.' : 'Git host token removed.')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      toast.error('Git host token not saved', err.message)
    } finally {
      savingToken = false
    }
  }

  async function deploy(p: Preview) {
    try {
      await api('POST', `/applications/${application.id}/previews/${p.number}/deploy`)
      toast.success('Preview deployment started.')
      await load()
      onchange?.()
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      toast.error('Preview deployment not started', err.message)
    }
  }

  async function remove(p: Preview) {
    try {
      await api('DELETE', `/applications/${application.id}/previews/${p.number}`)
      toast.success('Preview deployment deleted.')
      await load()
      onchange?.()
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      toast.error('Preview deployment not deleted', err.message)
    }
  }

  // Coolify's hidden confirmation triggers, clicked from the Actions menu.
  const openModal = (id: string) => document.getElementById(id)?.click()
</script>

<div class="chrome flex flex-col gap-6">
  <SettingsSection id="preview-settings-section" title="Preview settings" helper="Automatic pull request deployments and who can trigger them.">
    {#snippet actions()}
      {#if projectAccess.can('manage_applications') && webhook}
        {#if webhook.previews}
          <Button loading={toggling} onclick={togglePreviewDeployments}>Disable preview deployments</Button>
        {:else}
          <Button variant="highlighted" loading={toggling} onclick={togglePreviewDeployments}>Enable preview deployments</Button>
        {/if}
      {/if}
    {/snippet}

    <div class="flex flex-col gap-4">
      <p class="text-[13px] leading-5 text-neutral-500 dark:text-fg-dim">
        Every pull request into <span class="font-mono">{application.git_branch}</span> gets its own deployment. The webhook (under
        <a class="font-medium text-coollabs underline-offset-2 hover:underline" href={href(applicationPath(application, 'webhooks'))}
          >Webhooks</a
        >) must also send pull request events. Pull requests from forks get no preview.
      </p>
      {#if webhook}
        {#if webhook.has_git_host_token}
          <div
            class="flex items-center justify-between gap-3 rounded-lg bg-neutral-50 px-3 py-2.5 ring-1 ring-neutral-200 dark:bg-white/[0.05] dark:ring-white/[0.07]"
          >
            <span class="text-[12px] text-neutral-500 dark:text-fg-dim">Git host token</span>
            <span class="flex items-center gap-2">
              <span class="text-[12px] font-medium text-neutral-900 dark:text-fg" data-testid="token-saved">Saved</span>
              {#if projectAccess.can('manage_applications')}<Button loading={savingToken} onclick={() => saveToken('')}>Remove</Button>{/if}
            </span>
          </div>
        {:else if projectAccess.can('manage_applications')}
          <form
            class="grid gap-4 md:grid-cols-[minmax(0,1fr)_auto] md:items-end"
            onsubmit={(e) => {
              e.preventDefault()
              saveToken(token)
            }}
          >
            <Input
              type="password"
              label="Git host token"
              bind:value={token}
              autocomplete="new-password"
              placeholder="A token with access to the repository's issues"
              helper="With a token, The Bakery keeps one comment on each pull request with its preview's link."
            />
            <Button type="submit" loading={savingToken} disabled={!token}>Save token</Button>
          </form>
        {/if}
      {/if}
    </div>
  </SettingsSection>

  <SettingsSection id="preview-template-section" title="Preview URL template" helper="How The Bakery generates domains for pull request deployments.">
    <div
      class="flex items-center justify-between gap-3 rounded-lg bg-neutral-100 px-3 py-2.5 ring-1 ring-neutral-200 dark:bg-white/[0.04] dark:ring-white/[0.07]"
    >
      <span class="text-[13px] text-neutral-500 dark:text-fg-dim">Generated pattern</span>
      <code class="text-right font-mono text-xs break-all text-neutral-700 dark:text-fg">{'pr-{{pr_id}}.{{domain}}'}</code>
    </div>
  </SettingsSection>

  <SettingsSection
    id="preview-deployments-section"
    title="Preview deployments"
    helper="Manage domains, deployments, logs, and lifecycle actions for configured previews."
    flush
  >
    {#each open as p (p.number)}
      {@const status = p.latest_deployment ? deploymentStatus(p.latest_deployment) : null}
      <section class="border-b border-neutral-200 p-4 last:border-b-0 dark:border-white/[0.07]" data-testid="preview-row">
        <div class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
          <div class="flex min-w-0 items-center gap-3">
            <div
              class="flex size-9 shrink-0 items-center justify-center rounded-lg bg-neutral-100 font-mono text-xs font-semibold text-neutral-600 ring-1 ring-neutral-200 dark:bg-white/[0.05] dark:text-fg-dim dark:ring-white/[0.07]"
            >
              #{p.number}
            </div>
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <h4 class="text-sm font-semibold text-black dark:text-fg">Preview #{p.number}</h4>
                {#if status}<StatusBadge status={status.label} type={status.type} />{/if}
              </div>
              <p class="mt-0.5 truncate text-[12px] text-neutral-500 dark:text-fg-dim">
                {p.title} · <span class="font-mono">{p.branch}</span>
              </p>
            </div>
          </div>

          <div class="flex shrink-0 flex-wrap items-center gap-2 lg:justify-end">
            <TableDropdown role="menu" panelClass="w-56! min-w-56!">
              {#snippet trigger({ open, toggle })}
                <button type="button" class="button gap-1.5" title="Preview links" aria-expanded={open} aria-haspopup="menu" onclick={toggle}>
                  <Icon name="external-link" class="size-3.5 opacity-70" />
                  Links
                  <Icon name="chevron-down" class="size-3 opacity-55" />
                </button>
              {/snippet}
              {#snippet children(close)}
                {#if p.public_url}
                  <a target="_blank" rel="noreferrer" class="listbox-option justify-start! gap-2.5!" href={p.public_url} onclick={close} role="menuitem">
                    <Icon name="external-link" class="size-3.5 opacity-70" />
                    <span class="min-w-0 truncate">Open preview</span>
                  </a>
                {/if}
                {#if p.url}
                  <a target="_blank" rel="noreferrer" class="listbox-option justify-start! gap-2.5!" href={p.url} onclick={close} role="menuitem">
                    <Icon name="external-link" class="size-3.5 opacity-70" />
                    <span class="min-w-0 truncate">Open pull request</span>
                  </a>
                {/if}
              {/snippet}
            </TableDropdown>

            {#if p.latest_deployment}
              <a class="button gap-1.5" href={href(`${applicationPath(application, 'deployment')}/${p.latest_deployment.id}`)} title="Preview deployment logs">
                <Icon name="terminal" class="size-3.5 opacity-70" />
                Logs
              </a>
            {/if}

            {#if (projectAccess.can('deploy') || projectAccess.can('manage_applications'))}
              <TableDropdown role="menu" panelClass="w-52! min-w-52!">
                {#snippet trigger({ open, toggle })}
                  <button type="button" class="button gap-1.5" title="Preview actions" aria-expanded={open} aria-haspopup="menu" onclick={toggle}>
                    Actions
                    <span class={['inline-flex transition-transform', open && 'rotate-180']}><Icon name="chevron-down" class="size-3 opacity-55" /></span>
                  </button>
                {/snippet}
                {#snippet children(close)}
                  <button
                    type="button"
                    class="listbox-option justify-start! gap-2.5!"
                    role="menuitem"
                    disabled={p.latest_deployment?.status === 'queued'}
                    onclick={() => {
                      close()
                      deploy(p)
                    }}
                  >
                    <Icon name="play-circle" class="size-3.5 opacity-70" />
                    {p.latest_deployment ? 'Redeploy' : 'Deploy'}
                  </button>
                  <button
                    type="button"
                    class="listbox-option justify-start! gap-2.5! text-error!"
                    role="menuitem"
                    onclick={() => {
                      close()
                      openModal(`preview-delete-trigger-${p.number}`)
                    }}
                  >
                    <Icon name="trash" class="size-3.5" />
                    Delete
                  </button>
                {/snippet}
              </TableDropdown>
            {/if}
          </div>

          {#if projectAccess.can('manage_applications')}
            <div class="hidden" aria-hidden="true">
              <ConfirmationModal
                title="Delete preview deployment?"
                buttonTitle="Delete"
                variant="error"
                actions={[
                  'All containers for this preview deployment will be stopped and permanently deleted.',
                  'It comes back when the pull request is pushed to or reopened.',
                ]}
                confirmationText={p.domain}
                confirmationLabel="Enter the preview deployment name to confirm deletion"
                shortConfirmationLabel="Preview deployment name"
                onconfirm={() => remove(p)}
              >
                {#snippet trigger(show)}
                  <button id={`preview-delete-trigger-${p.number}`} type="button" onclick={show}>Delete</button>
                {/snippet}
              </ConfirmationModal>
            </div>
          {/if}
        </div>

        <div class="mt-4 border-t border-neutral-200 pt-4 dark:border-white/[0.07]">
          <p class="mb-3 text-[13px] text-neutral-500 dark:text-fg-dim">1 domain</p>
          <div class="application-settings-section-body is-flush overflow-visible">
            <div class="flex items-center gap-3 px-4 py-3">
              <Icon name="globe" class="size-4 shrink-0 text-neutral-400 dark:text-fg-faint" />
              {#if p.public_url}
                <a class="min-w-0 truncate font-mono text-[13px] text-black hover:underline dark:text-fg" href={p.public_url} target="_blank" rel="noreferrer"
                  >{p.domain}</a
                >
              {:else}
                <span class="min-w-0 truncate font-mono text-[13px] text-neutral-500 dark:text-fg-dim">{p.domain}</span>
              {/if}
            </div>
          </div>
        </div>
      </section>
    {:else}
      {#if loaded}
        <Empty
          title="No preview deployments"
          description="Open a pull request into the branch to create an isolated deployment."
          icon="eye"
        />
      {/if}
    {/each}
  </SettingsSection>
</div>
