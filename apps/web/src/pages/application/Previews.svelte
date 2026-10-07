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
  //
  // The list is Paperclip's EntityRow pattern (as the Environment page and
  // Deployment history are): one row per open Preview, its Deploy/Redeploy
  // and Remove actions as buttons on the row.
  import { buttonVariants } from '$lib/components/ui/button'
  import { Card } from '$lib/components/ui/card'
  import * as DropdownMenu from '$lib/components/ui/dropdown-menu'
  import { api, ApiError } from '../../lib/api'
  import EntityRow from '../../lib/EntityRow.svelte'
  import Icon from '../../lib/Icon.svelte'
  import { applicationPath, href } from '../../lib/router.svelte'
  import { projectAccess } from '../../lib/projectAccess.svelte'
  import SettingsGroup from '../../lib/settings/SettingsGroup.svelte'
  import type { Application, Preview, Webhook } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import Input from '../../lib/ui/Input.svelte'
  import StatusBadge from '../../lib/ui/StatusBadge.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import { deploymentStatus } from './DeploymentHistory.svelte'

  let { application, onchange }: { application: Application; onchange?: () => void } = $props()

  let previews = $state.raw<Preview[]>([])
  let loaded = $state(false)
  let webhook = $state.raw<Webhook | null>(null)
  let toggling = $state(false)
  let token = $state('')
  let savingToken = $state(false)
  let deploying = $state(0)

  const open = $derived(previews.filter((p) => p.state === 'open'))
  const anyDeploying = $derived(previews.some((p) => p.latest_deployment?.active))

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
    const t = setInterval(() => load().catch(() => {}), anyDeploying ? 3000 : 5000)
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
    deploying = p.number
    try {
      await api('POST', `/applications/${application.id}/previews/${p.number}/deploy`)
      toast.success('Preview deployment started.')
      await load()
      onchange?.()
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      toast.error('Preview deployment not started', err.message)
    } finally {
      deploying = 0
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

  // Coolify's hidden confirmation triggers, clicked from the row's Remove button.
  const openModal = (id: string) => document.getElementById(id)?.click()

  const listCard = 'block gap-0 overflow-hidden py-0'
</script>

<div class="chrome flex flex-col gap-6">
  <SettingsGroup id="preview-settings-section" label="Preview settings" hint="Automatic pull request deployments and who can trigger them." wide>
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
      <p class="text-sm text-muted-foreground">
        Every pull request into <span class="font-mono">{application.git_branch}</span> gets its own deployment. The webhook (under
        <a class="font-medium text-primary underline-offset-2 hover:underline" href={href(applicationPath(application, 'webhooks'))}>Webhooks</a>) must
        also send pull request events. Pull requests from forks get no preview.
      </p>
      {#if webhook}
        {#if webhook.has_git_host_token}
          <div class="flex items-center justify-between gap-3 rounded-lg bg-muted/50 px-3 py-2.5 ring-1 ring-border">
            <span class="text-xs text-muted-foreground">Git host token</span>
            <span class="flex items-center gap-2">
              <span class="text-xs font-medium text-foreground" data-testid="token-saved">Saved</span>
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
  </SettingsGroup>

  <SettingsGroup id="preview-template-section" label="Preview URL template" hint="How The Bakery generates domains for pull request deployments." wide>
    <div class="flex items-center justify-between gap-3 rounded-lg bg-muted/50 px-3 py-2.5 ring-1 ring-border">
      <span class="text-sm text-muted-foreground">Generated pattern</span>
      <code class="text-right font-mono text-xs break-all text-foreground">{'pr-{{pr_id}}.{{domain}}'}</code>
    </div>
  </SettingsGroup>

  <SettingsGroup
    id="preview-deployments-section"
    label="Preview deployments"
    hint="Manage domains, deployments, logs, and lifecycle actions for configured previews."
    wide
  >
    {#if open.length === 0}
      {#if loaded}
        <Card class={listCard}>
          <Empty size="sm" title="No preview deployments" description="Open a pull request into the branch to create an isolated deployment." icon="eye" />
        </Card>
      {/if}
    {:else}
      <Card class={listCard}>
        {#each open as p (p.number)}
          {@const status = p.latest_deployment ? deploymentStatus(p.latest_deployment) : null}
          <EntityRow
            title={`Preview #${p.number}`}
            subtitle={`${p.title} · ${p.branch} · ${p.domain}`}
            reserveSubtitleSpace
            data-testid="preview-row"
          >
            {#snippet leading()}
              <span class="flex size-8 shrink-0 items-center justify-center rounded-md bg-muted font-mono text-xs font-semibold text-muted-foreground">
                #{p.number}
              </span>
            {/snippet}
            {#snippet trailing()}
              {#if status}<StatusBadge status={status.label} type={status.type} />{/if}

              <DropdownMenu.Root>
                <DropdownMenu.Trigger class={buttonVariants({ variant: 'outline', size: 'sm' })} title="Preview links">
                  <Icon name="external-link" class="size-3.5 opacity-70" />
                  Links
                  <Icon name="chevron-down" class="size-3 opacity-55" />
                </DropdownMenu.Trigger>
                <DropdownMenu.Content align="end" class="w-56">
                  {#if p.public_url}
                    <DropdownMenu.Item>
                      {#snippet child({ props })}
                        <a {...props} target="_blank" rel="noreferrer" href={p.public_url}>
                          <Icon name="external-link" class="size-3.5 opacity-70" />
                          <span class="min-w-0 truncate">Open preview</span>
                        </a>
                      {/snippet}
                    </DropdownMenu.Item>
                  {/if}
                  {#if p.url}
                    <DropdownMenu.Item>
                      {#snippet child({ props })}
                        <a {...props} target="_blank" rel="noreferrer" href={p.url}>
                          <Icon name="external-link" class="size-3.5 opacity-70" />
                          <span class="min-w-0 truncate">Open pull request</span>
                        </a>
                      {/snippet}
                    </DropdownMenu.Item>
                  {/if}
                  {#if !p.public_url && !p.url}
                    <DropdownMenu.Item disabled>No links available</DropdownMenu.Item>
                  {/if}
                </DropdownMenu.Content>
              </DropdownMenu.Root>

              {#if p.latest_deployment}
                <a
                  class={buttonVariants({ variant: 'outline', size: 'sm' })}
                  href={href(`${applicationPath(application, 'deployment')}/${p.latest_deployment.id}`)}
                  title="Preview deployment logs"
                >
                  <Icon name="terminal" class="size-3.5 opacity-70" />
                  Logs
                </a>
              {/if}

              {#if projectAccess.can('deploy')}
                <button
                  type="button"
                  class={buttonVariants({ variant: 'outline', size: 'sm' })}
                  disabled={p.latest_deployment?.status === 'queued' || deploying === p.number}
                  onclick={() => deploy(p)}
                >
                  {p.latest_deployment ? 'Redeploy' : 'Deploy'}
                </button>
              {/if}

              {#if projectAccess.can('manage_applications')}
                <button
                  type="button"
                  class={buttonVariants({ variant: 'destructive', size: 'sm' })}
                  onclick={() => openModal(`preview-delete-trigger-${p.number}`)}
                >
                  Remove
                </button>
              {/if}
            {/snippet}
          </EntityRow>

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
        {/each}
      </Card>
    {/if}
  </SettingsGroup>
</div>
