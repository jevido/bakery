<script lang="ts">
  // Coolify's Webhooks (resources/views/livewire/project/shared/webhooks.blade.php
  // and app/Livewire/Project/Shared/Webhooks.php, Apache-2.0, see NOTICE):
  // the manual Git webhooks, one section per Git host. The Bakery has one
  // push webhook per Application with one secret that every host signs
  // with, so each section shows the same URL and secret, and the secret is
  // rotated rather than typed. Only the hosts the receiver verifies are
  // listed: GitHub (X-Hub-Signature-256), GitLab (X-Gitlab-Token) and
  // Gitea and Forgejo (their signature headers); Bitbucket is not. Coolify's
  // Deploy webhook (/api/v1/deploy) comes with the /api/v1 API.
  import { api, ApiError } from '../../lib/api'
  import { buttonVariants } from '$lib/components/ui/button'
  import Icon from '../../lib/Icon.svelte'
  import { projectAccess } from '../../lib/projectAccess.svelte'
  import type { Application, Webhook } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import CopyButton from '../../lib/ui/CopyButton.svelte'
  import Input from '../../lib/ui/Input.svelte'
  import SettingsGroup from '../../lib/settings/SettingsGroup.svelte'
  import { toast } from '../../lib/ui/toast.svelte'

  let { application }: { application: Application } = $props()

  let webhook = $state.raw<Webhook | null>(null)

  $effect(() => {
    webhook = null
    // The Webhook comes with its secret, which a viewer may not read.
    if (!projectAccess.can('see_secrets')) return
    api<{ webhook: Webhook }>('GET', `/applications/${application.id}/webhook`)
      .then((r) => (webhook = r.webhook))
      .catch((err) => toast.error('Webhook not loaded', err.message))
  })

  // The API is served from the dashboard's own origin (the Vite proxy in
  // development, the Dashboard Route on a server).
  const url = $derived(location.origin + (webhook?.path ?? `/api/webhooks/applications/${application.id}`))

  const providers = [
    { name: 'GitHub', description: 'Accepts JSON payloads signed with the secret (X-Hub-Signature-256). Send push events, and pull request events for Preview Deployments.' },
    { name: 'GitLab', description: 'Use the same secret as the Secret token when configuring the webhook in GitLab. Send push events, and merge request events for Preview Deployments.' },
    { name: 'Gitea / Forgejo', description: 'Use the same secret when configuring the webhook in Gitea or Forgejo. Send push events, and pull request events for Preview Deployments.' },
  ]

  // Coolify's gitWebhook: the repository's webhook settings, from an https
  // or scp-style URL.
  const settings = $derived.by(() => {
    const u = application.git_url
    const m = u.match(/^https?:\/\/([^/]+)\/(.+?)(?:\.git)?\/?$/) ?? u.match(/^(?:ssh:\/\/)?[^@]+@([^:/]+)[:/](.+?)(?:\.git)?$/)
    return m ? `https://${m[1]}/${m[2]}/settings/hooks` : null
  })

  async function rotate() {
    try {
      webhook = (await api<{ webhook: Webhook }>('POST', `/applications/${application.id}/webhook/secret`)).webhook
      toast.success('Webhook secret rotated.', 'Paste the new secret into the Git host; pushes signed with the old one are refused.')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      toast.error('Webhook secret not rotated', err.message)
    }
  }
</script>

<SettingsGroup
  id="manual-git-webhooks-section"
  label="Manual Git webhooks"
  hint="Configure these endpoints on the Git host so a push deploys the application. The Git host must be able to reach this URL."
  wide
>
  {#snippet actions()}
    {#if settings}
      <a class={buttonVariants({ variant: 'outline', size: 'sm' })} href={settings} target="_blank" rel="noopener noreferrer">
        Repository settings
        <Icon name="external-link" class="size-3.5" />
      </a>
    {/if}
    {#if projectAccess.can('manage_applications') && projectAccess.can('see_secrets')}
      <ConfirmationModal
        title="Rotate webhook secret?"
        buttonTitle="Rotate"
        actions={['A new webhook secret is generated.', 'Pushes stop deploying until the new secret is pasted into the Git host.']}
        confirmWithText={false}
        onconfirm={rotate}
      >
        {#snippet trigger(show)}
          <Button onclick={show}>Rotate secret</Button>
        {/snippet}
      </ConfirmationModal>
    {/if}
  {/snippet}

  <div class="divide-y divide-border rounded-md border border-border">
    {#each providers as provider (provider.name)}
      <section class="px-4 py-4">
        <div class="mb-3">
          <h4 class="text-sm font-medium text-foreground">{provider.name}</h4>
          <p class="mt-0.5 text-xs text-muted-foreground">{provider.description}</p>
        </div>
        <div class="grid gap-3 md:grid-cols-2">
          <CopyButton label="Webhook URL" text={url} testid="webhook-url" />
          {#if projectAccess.can('see_secrets')}
            <Input
              type="password"
              label="Webhook secret"
              value={webhook?.secret ?? ''}
              readonly
              helper={`Must exactly match the secret configured in ${provider.name}.`}
              autocomplete="new-password"
              data-testid="webhook-secret"
            />
          {:else}
            <Input disabled label="Webhook secret" value="Hidden (only administrators can view)" />
          {/if}
        </div>
      </section>
    {/each}
  </div>
</SettingsGroup>
