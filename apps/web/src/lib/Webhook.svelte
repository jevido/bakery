<script lang="ts">
  import { session } from './session.svelte'
  import { api } from './api'
  import CopyButton from './CopyButton.svelte'
  import type { Webhook } from './types'

  let { applicationId }: { applicationId: number } = $props()

  let webhook = $state.raw<Webhook | null>(null)
  let error = $state('')
  let revealed = $state(false)
  let busy = $state(false)

  // The API is served from the dashboard's own origin (the Vite proxy in
  // development, the Dashboard Route on a server).
  let url = $derived(webhook ? location.origin + webhook.path : '')

  $effect(() => {
    webhook = null
    error = ''
    // The Webhook comes with its secret, which a viewer may not read.
    if (!session.canSeeSecrets) return
    api<{ webhook: Webhook }>('GET', `/applications/${applicationId}/webhook`)
      .then((r) => (webhook = r.webhook))
      .catch((e) => (error = e.message))
  })

  async function change(method: string, path: string, body?: unknown) {
    busy = true
    error = ''
    try {
      webhook = (await api<{ webhook: Webhook }>(method, path, body)).webhook
    } catch (e) {
      error = e instanceof Error ? e.message : String(e)
    } finally {
      busy = false
    }
  }

  function rotate() {
    if (!confirm('Rotate the secret? Pushes stop deploying until you paste the new secret into the git host.')) return
    revealed = true
    change('POST', `/applications/${applicationId}/webhook/secret`)
  }
</script>

<section class="webhook">
  <h3>Webhook</h3>
  {#if error}<p class="error">{error}</p>{/if}
  {#if !session.canSeeSecrets}
    <p class="muted">The webhook and its secret are hidden for viewers.</p>
  {:else if webhook}
    <label class="toggle">
      <input
        type="checkbox"
        checked={webhook.auto_deploy}
        disabled={busy}
        onchange={(e) => change('PATCH', `/applications/${applicationId}/webhook`, { auto_deploy: e.currentTarget.checked })}
      />
      Auto-deploy: every push to the branch starts a deployment
    </label>
    <div class="row">
      <span class="muted">URL</span>
      <code class="mono" data-testid="webhook-url">{url}</code>
      <CopyButton text={url} label="Copy URL" />
    </div>
    <div class="row">
      <span class="muted">Secret</span>
      <code class="mono" data-testid="webhook-secret">{revealed ? webhook.secret : '•'.repeat(24)}</code>
      <span class="actions">
        <button type="button" onclick={() => (revealed = !revealed)}>{revealed ? 'Hide' : 'Show'}</button>
        <CopyButton text={webhook.secret} label="Copy secret" />
        <button type="button" onclick={rotate} disabled={busy}>Rotate secret</button>
      </span>
    </div>
    <details>
      <summary>How to set this up</summary>
      <ul>
        <li><strong>GitHub:</strong> Settings → Webhooks → Add webhook. Payload URL and Secret from above, content type <code>application/json</code>, "Just the push event".</li>
        <li><strong>Gitea / Forgejo:</strong> Settings → Webhooks → Add webhook → Gitea (or Forgejo). Target URL and Secret from above, POST, content type JSON, push events.</li>
        <li><strong>GitLab:</strong> Settings → Webhooks. URL from above, the secret as Secret token, trigger "Push events".</li>
      </ul>
      <p class="muted">The git host must be able to reach this URL. Pushes to other branches are ignored.</p>
    </details>
  {:else if !error}
    <p class="muted">Loading…</p>
  {/if}
</section>

<style>
  .webhook {
    display: grid;
    gap: 0.6rem;
    margin-top: 1.5rem;
  }
  h3 {
    margin: 0;
  }
  .toggle {
    display: flex;
    gap: 0.5rem;
    align-items: center;
  }
  .row {
    display: grid;
    grid-template-columns: 4rem minmax(0, 1fr) auto;
    gap: 0.75rem;
    align-items: center;
  }
  .row code {
    overflow-wrap: anywhere;
  }
  .actions {
    display: flex;
    gap: 0.5rem;
  }
  ul {
    margin: 0.5rem 0;
    padding-left: 1.2rem;
    display: grid;
    gap: 0.3rem;
  }
</style>
