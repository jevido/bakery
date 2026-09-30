<script lang="ts">
  import { api, ApiError } from './api'
  import Field from './Field.svelte'
  import type { ServiceInput, ServiceTemplate } from './types'

  // Creates a Service from the template catalog or from a pasted compose
  // file.
  let {
    onsubmit,
    oncancel,
  }: {
    onsubmit: (input: ServiceInput) => Promise<void>
    oncancel?: () => void
  } = $props()

  let mode = $state<'template' | 'compose'>('template')
  let templates = $state.raw<ServiceTemplate[]>([])
  let loadError = $state('')
  let search = $state('')
  let picked = $state<string | null>(null)
  let name = $state('')
  let compose = $state('')
  let errors = $state<Record<string, string>>({})
  let message = $state('')
  let busy = $state(false)

  api<{ templates: ServiceTemplate[] }>('GET', '/service-templates')
    .then((r) => (templates = r.templates))
    .catch((e) => (loadError = e.message))

  let shown = $derived.by(() => {
    const q = search.trim().toLowerCase()
    if (!q) return templates
    return templates.filter((t) => [t.name, t.description, ...t.tags].some((s) => s.toLowerCase().includes(q)))
  })
  let composeLines = $derived(errors.compose ? errors.compose.split('\n') : [])

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    errors = {}
    message = ''
    try {
      if (mode === 'template') {
        if (!picked) {
          message = 'Pick a template.'
          return
        }
        await onsubmit({ name, template: picked })
      } else {
        await onsubmit({ name, compose })
      }
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      errors = err.errors
      const shown = ['name', 'compose', 'domains', 'template']
      if (!Object.keys(err.errors).some((k) => shown.includes(k))) message = err.message
    } finally {
      busy = false
    }
  }
</script>

<form class="form" onsubmit={submit}>
  <div class="tabs" role="tablist">
    <button type="button" role="tab" aria-selected={mode === 'template'} class={[mode === 'template' && 'selected']} onclick={() => (mode = 'template')}>
      From a template
    </button>
    <button type="button" role="tab" aria-selected={mode === 'compose'} class={[mode === 'compose' && 'selected']} onclick={() => (mode = 'compose')}>
      Compose file
    </button>
  </div>

  {#if mode === 'template'}
    <input type="search" placeholder="Search templates" aria-label="Search templates" bind:value={search} />
    {#if loadError}
      <p class="error">{loadError}</p>
    {:else}
      <div class="grid">
        {#each shown as t (t.key)}
          <label class={['template', picked === t.key && 'selected']}>
            <input type="radio" name="template" value={t.key} bind:group={picked} />
            <strong>{t.name}</strong>
            <span class="muted">{t.description}</span>
            <span class="tags">
              {#each t.tags as tag (tag)}<span class="tag">{tag}</span>{/each}
              <a href={t.docs_url} target="_blank" rel="noreferrer">docs</a>
            </span>
          </label>
        {:else}
          <p class="muted">No template matches.</p>
        {/each}
      </div>
    {/if}
    <Field label="Name (optional, the template's by default)" bind:value={name} error={errors.name} />
  {:else}
    <Field label="Name" bind:value={name} error={errors.name} required />
    <label class="compose">
      <span>compose.yml</span>
      <textarea
        bind:value={compose}
        rows="14"
        spellcheck="false"
        placeholder={'services:\n  web:\n    image: docker.io/traefik/whoami:v1.10\n    environment:\n      - SERVICE_FQDN_WEB_80'}
        required
      ></textarea>
    </label>
    {#if composeLines.length > 0}
      <ul class="error lines">
        {#each composeLines as line, i (i)}<li>{line}</li>{/each}
      </ul>
    {/if}
    <p class="muted small">
      Only <code>image:</code> services and named volumes. <code>SERVICE_FQDN_&lt;NAME&gt;_&lt;PORT&gt;</code> in a
      service's environment gives it a domain; <code>SERVICE_PASSWORD_&lt;X&gt;</code> and
      <code>SERVICE_USER_&lt;X&gt;</code> are generated for you.
    </p>
  {/if}
  {#if errors.domains}<p class="error">{errors.domains}</p>{/if}
  {#if errors.template}<p class="error">{errors.template}</p>{/if}
  {#if message}<p class="error">{message}</p>{/if}
  <div class="actions">
    {#if oncancel}<button type="button" onclick={oncancel}>Cancel</button>{/if}
    <button class="primary" disabled={busy}>Create service</button>
  </div>
</form>

<style>
  .form {
    display: grid;
    gap: 0.8rem;
  }
  .tabs {
    display: flex;
    gap: 0.4rem;
  }
  .tabs .selected {
    font-weight: 600;
    border-color: currentColor;
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(14rem, 1fr));
    gap: 0.6rem;
  }
  .template {
    display: grid;
    gap: 0.3rem;
    align-content: start;
    padding: 0.7rem;
    border: 1px solid var(--border, #8884);
    border-radius: 0.4rem;
    cursor: pointer;
  }
  .template.selected {
    border-color: currentColor;
  }
  .template input {
    position: absolute;
    opacity: 0;
    pointer-events: none;
  }
  .template:has(input:focus-visible) {
    outline: 2px solid currentColor;
  }
  .tags {
    display: flex;
    flex-wrap: wrap;
    gap: 0.3rem;
    align-items: center;
    font-size: 0.75rem;
  }
  .tag {
    padding: 0 0.4rem;
    border: 1px solid var(--border, #8884);
    border-radius: 999px;
  }
  .compose {
    display: grid;
    gap: 0.3rem;
  }
  textarea {
    font-family: ui-monospace, monospace;
    font-size: 0.85rem;
  }
  .lines {
    margin: 0;
    padding-left: 1.2rem;
    font-family: ui-monospace, monospace;
    font-size: 0.8rem;
  }
  .small {
    font-size: 0.8rem;
  }
  p {
    margin: 0;
  }
  .actions {
    display: flex;
    gap: 0.5rem;
    justify-content: flex-end;
  }
</style>
