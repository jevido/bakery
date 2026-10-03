<script lang="ts">
  import { session } from './session.svelte'
  import { ApiError } from './api'
  import Field from './Field.svelte'
  import type { ServiceInput } from './types'

  // Creates a Service from a pasted compose file: the Docker Compose card of
  // the New Resource page. Catalog templates are created from their own cards.
  let {
    onsubmit,
    oncancel,
  }: {
    onsubmit: (input: ServiceInput) => Promise<void>
    oncancel?: () => void
  } = $props()

  let name = $state('')
  let compose = $state('')
  let errors = $state<Record<string, string>>({})
  let message = $state('')
  let busy = $state(false)

  let composeLines = $derived(errors.compose ? errors.compose.split('\n') : [])

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    errors = {}
    message = ''
    try {
      await onsubmit({ name, compose })
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      errors = err.errors
      const shown = ['name', 'compose', 'domains']
      if (!Object.keys(err.errors).some((k) => shown.includes(k))) message = err.message
    } finally {
      busy = false
    }
  }
</script>

<form class="form" onsubmit={submit}>
  <!-- A viewer sees the values and cannot change them. -->
  <fieldset class="contents" disabled={!session.canWrite}>
  <Field label="Name (optional, generated when empty)" bind:value={name} error={errors.name} />
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
  {#if errors.domains}<p class="error">{errors.domains}</p>{/if}
  {#if message}<p class="error">{message}</p>{/if}
  </fieldset>
  {#if session.canWrite}
  <div class="actions">
    {#if oncancel}<button type="button" onclick={oncancel}>Cancel</button>{/if}
    <button class="primary" disabled={busy}>Create service</button>
  </div>
  {/if}
</form>

<style>
  .form {
    display: grid;
    gap: 0.8rem;
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
