<script lang="ts">
  import { api } from './api'
  import { session } from './session.svelte'
  import StatusBadge from './StatusBadge.svelte'
  import type { Preview, Webhook } from './types'

  let { applicationId, onchange }: { applicationId: number; onchange?: () => void } = $props()

  let previews = $state.raw<Preview[]>([])
  let webhook = $state.raw<Webhook | null>(null)
  let error = $state('')
  let busy = $state(false)
  let token = $state('')

  let deploying = $derived(previews.some((p) => p.latest_deployment?.active))

  async function load() {
    previews = (await api<{ previews: Preview[] }>('GET', `/applications/${applicationId}/previews`)).previews
  }

  $effect(() => {
    previews = []
    webhook = null
    error = ''
    load().catch((e) => (error = e.message))
    // The switch and token live on the Webhook, which carries its secret.
    if (session.canSeeSecrets) {
      api<{ webhook: Webhook }>('GET', `/applications/${applicationId}/webhook`)
        .then((r) => (webhook = r.webhook))
        .catch((e) => (error = e.message))
    }
  })

  // Quickly while a Preview deploys, slower so one opened by a pull request
  // shows up by itself.
  $effect(() => {
    const t = setInterval(() => load().catch(() => {}), deploying ? 3000 : 5000)
    return () => clearInterval(t)
  })

  async function run(f: () => Promise<void>) {
    busy = true
    error = ''
    try {
      await f()
    } catch (e) {
      error = e instanceof Error ? e.message : String(e)
    } finally {
      busy = false
    }
  }

  function setWebhook(body: { previews?: boolean; git_host_token?: string }) {
    return run(async () => {
      webhook = (await api<{ webhook: Webhook }>('PATCH', `/applications/${applicationId}/webhook`, body)).webhook
      token = ''
    })
  }

  function redeploy(p: Preview) {
    return run(async () => {
      await api('POST', `/applications/${applicationId}/previews/${p.number}/deploy`)
      await load()
      onchange?.()
    })
  }

  function remove(p: Preview) {
    if (!confirm(`Remove the preview of pull request #${p.number}? Its container, data and route go; it comes back when the pull request is pushed to or reopened.`)) return
    return run(async () => {
      await api('DELETE', `/applications/${applicationId}/previews/${p.number}`)
      await load()
      onchange?.()
    })
  }
</script>

<section class="previews">
  {#if error}<p class="error">{error}</p>{/if}

  {#if webhook}
    <label class="toggle">
      <input
        type="checkbox"
        data-testid="previews-switch"
        checked={webhook.previews}
        disabled={busy}
        onchange={(e) => setWebhook({ previews: e.currentTarget.checked })}
      />
      Preview deployments: every pull request into the branch gets its own copy on <span class="mono">pr-&lt;number&gt;.&lt;domain&gt;</span>
    </label>
    <form
      class="token"
      onsubmit={(e) => {
        e.preventDefault()
        setWebhook({ git_host_token: token })
      }}
    >
      <span class="muted">Git host token</span>
      {#if webhook.has_git_host_token}
        <span data-testid="token-saved">saved</span>
        <button type="button" disabled={busy} onclick={() => setWebhook({ git_host_token: '' })}>Remove</button>
      {:else}
        <input type="password" autocomplete="off" placeholder="token with access to the repository's issues" bind:value={token} />
        <button disabled={busy || !token}>Save token</button>
      {/if}
    </form>
    <p class="muted small">
      With a token, Bakery keeps one comment on each pull request with its preview's link. The webhook (on the Source tab)
      must also send pull request events: on GitHub "Pull requests", on Gitea and Forgejo "Pull request" and "Pull request
      synchronized", on GitLab "Merge request events". Pull requests from forks get no preview.
    </p>
  {/if}

  {#if previews.length === 0}
    <p class="muted">No previews yet.</p>
  {:else}
    <table>
      <thead>
        <tr><th>Pull request</th><th>Preview</th><th>Status</th><th></th></tr>
      </thead>
      <tbody>
        {#each previews as p (p.number)}
          <tr class={{ closed: p.state === 'closed' }} data-testid="preview-row">
            <td>
              <a href={p.url} target="_blank" rel="noreferrer">#{p.number}</a>
              {p.title}
              <div class="muted small mono">{p.branch}</div>
            </td>
            <td>
              {#if p.state === 'open' && p.public_url}
                <a class="mono" href={p.public_url} target="_blank" rel="noreferrer">{p.domain}</a>
              {:else}
                <span class="muted">closed</span>
              {/if}
            </td>
            <td>
              {#if p.latest_deployment}<StatusBadge status={p.latest_deployment.status} />{/if}
            </td>
            <td class="actions">
              {#if session.canWrite && p.state === 'open'}
                <button disabled={busy || p.latest_deployment?.status === 'queued'} onclick={() => redeploy(p)}>Redeploy</button>
                <button class="danger" disabled={busy} onclick={() => remove(p)}>Delete</button>
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
</section>

<style>
  .previews {
    display: grid;
    gap: 0.75rem;
  }
  .toggle {
    display: flex;
    gap: 0.5rem;
    align-items: center;
  }
  .token {
    display: flex;
    gap: 0.75rem;
    align-items: center;
  }
  .token input {
    flex: 1;
    max-width: 24rem;
  }
  .small {
    font-size: 0.85rem;
  }
  .actions {
    text-align: right;
    white-space: nowrap;
  }
  .closed {
    opacity: 0.6;
  }
</style>
