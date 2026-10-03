<script lang="ts">
  import { session } from './session.svelte'
  import { api } from './api'
  import CopyButton from './CopyButton.svelte'
  import type { Application } from './types'

  let {
    application,
    onchange,
  }: {
    application: Application
    /** Called with the Application carrying its new key. */
    onchange: (a: Application) => void
  } = $props()

  let busy = $state(false)
  let error = $state('')

  async function regenerate() {
    if (!confirm('Generate a new deploy key? The current one stops working once you remove it from the repository.')) return
    busy = true
    error = ''
    try {
      const r = await api<{ application: Application }>('POST', `/applications/${application.id}/deploy-key`)
      onchange(r.application)
    } catch (e) {
      error = e instanceof Error ? e.message : String(e)
    } finally {
      busy = false
    }
  }
</script>

<section class="deploy-key">
  <h3>Deploy key</h3>
  <p class="muted">
    Add this public key to the repository as a <strong>read-only deploy key</strong> (GitHub: Settings → Deploy keys; GitLab:
    Settings → Repository → Deploy keys; Gitea/Forgejo: Settings → Deploy keys). The Bakery clones with it.
  </p>
  <pre class="mono key" data-testid="deploy-key">{application.deploy_key_public}</pre>
  <div class="actions">
    <CopyButton text={application.deploy_key_public} label="Copy public key" />
    {#if session.canWrite}<button type="button" onclick={regenerate} disabled={busy}>Regenerate</button>{/if}
  </div>
  {#if error}<p class="error">{error}</p>{/if}
</section>

<style>
  .key {
    white-space: pre-wrap;
    word-break: break-all;
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 0.6rem 0.75rem;
    font-size: 0.85rem;
  }
  .actions {
    display: flex;
    gap: 0.5rem;
  }
</style>
