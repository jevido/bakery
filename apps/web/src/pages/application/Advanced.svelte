<script lang="ts">
  // Coolify's Application Advanced page (resources/views/livewire/project/application/advanced.blade.php,
  // app/Livewire/Project/Application/Advanced.php; Apache-2.0, see NOTICE),
  // with only the settings The Bakery has: Deployment → Auto deploy, saved at
  // once as Coolify's instantSave, and under Proxy the custom response
  // headers (The Bakery's own; Coolify's nearest home for them), behind the
  // unsaved-changes bar. The routing settings' redirect and Basic
  // Authentication are sent back as loaded.
  import { untrack } from 'svelte'
  import { api, ApiError } from '../../lib/api'
  import { projectAccess } from '../../lib/projectAccess.svelte'
  import SettingsGroup from '../../lib/settings/SettingsGroup.svelte'
  import { scrollToPendingSettingsSection } from '../../lib/settingsSection.svelte'
  import type { Application, RouteSettings, Webhook } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import FieldError from '../../lib/ui/FieldError.svelte'
  import FieldLabel from '../../lib/ui/FieldLabel.svelte'
  import { Input } from '$lib/components/ui/input'
  import Select from '../../lib/ui/Select.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import UnsavedBar from '../../lib/ui/UnsavedBar.svelte'

  let { application }: { application: Application } = $props()

  type HeaderRow = { key: number; name: string; value: string }

  const gitBased = $derived(application.build_pack !== 'dockerimage')
  const canUpdate = $derived(projectAccess.can('manage_applications'))

  let webhook = $state.raw<Webhook | null>(null)
  let routing = $state.raw<RouteSettings | null>(null)
  let headers = $state<HeaderRow[]>([])
  let errors = $state<Record<string, string>>({})
  let saving = $state(false)
  let nextKey = 0

  function reset() {
    headers = (routing?.response_headers ?? []).map((h) => ({ key: nextKey++, ...h }))
    errors = {}
  }

  $effect(() => {
    const id = application.id
    untrack(() => {
      webhook = null
      routing = null
      reset()
    })
    api<{ routing: RouteSettings }>('GET', `/applications/${id}/routing`)
      .then((r) => {
        routing = r.routing
        reset()
      })
      .catch((e) => toast.error('Proxy settings not loaded', e.message))
    // The Webhook comes with its secret, which a viewer may not read.
    if (projectAccess.can('see_secrets'))
      api<{ webhook: Webhook }>('GET', `/applications/${id}/webhook`)
        .then((r) => (webhook = r.webhook))
        .catch(() => {})
  })

  $effect(scrollToPendingSettingsSection)

  const filled = $derived(headers.filter((h) => h.name.trim() !== '').map((h) => ({ name: h.name.trim(), value: h.value })))
  const dirty = $derived(
    canUpdate &&
      !!routing &&
      JSON.stringify(filled) !== JSON.stringify(routing.response_headers.map((h) => ({ name: h.name, value: h.value }))),
  )

  async function autoDeployChanged(autoDeploy: boolean) {
    try {
      webhook = (await api<{ webhook: Webhook }>('PATCH', `/applications/${application.id}/webhook`, { auto_deploy: autoDeploy })).webhook
      toast.success('Settings saved.')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      toast.error('Settings not saved', err.message)
    }
  }

  async function save() {
    if (!routing || saving || !dirty) return
    saving = true
    errors = {}
    try {
      const r = await api<{ routing: RouteSettings }>('PUT', `/applications/${application.id}/routing`, {
        redirect: routing.redirect,
        response_headers: filled,
        basic_auth: { enabled: routing.basic_auth.enabled, username: routing.basic_auth.username, password: '' },
      })
      routing = r.routing
      reset()
      toast.success('Settings saved.')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      errors = Object.keys(err.errors).length ? err.errors : { response_headers: err.message }
    } finally {
      saving = false
    }
  }
</script>

<div class="space-y-8">
  {#if gitBased}
    <SettingsGroup id="advanced-deployment-section" label="Deployment" hint="Automatic deployments from Git webhooks.">
      {#if projectAccess.can('see_secrets')}
        <Select
          label="Auto deploy"
          value={webhook ? String(webhook.auto_deploy) : ''}
          onchange={(e) => autoDeployChanged(e.currentTarget.value === 'true')}
          helper="Automatically deploy new commits based on Git webhooks."
          disabled={!canUpdate || !webhook}
        >
          <option value="true">Deploy on push (webhooks)</option>
          <option value="false">Manual deployments only</option>
        </Select>
      {:else}
        <p class="text-sm text-muted-foreground">The webhook settings are hidden for viewers.</p>
      {/if}
    </SettingsGroup>
  {/if}

  <form
    onsubmit={(e) => {
      e.preventDefault()
      save()
    }}
  >
    {#if canUpdate}
      <UnsavedBar {dirty} {saving} onsave={save} onreset={reset} />
    {/if}
    <SettingsGroup id="advanced-proxy-section" label="Proxy" hint="How the proxy serves traffic for this application.">
      <FieldLabel
        label="Response headers"
        for="response-header-0"
        helper="Set on every response, replacing a header of the same name the application sends. Applies at once, without a deploy."
      />
      <div class="flex flex-col gap-2">
        {#each headers as h, i (h.key)}
          <div class="flex items-center gap-2">
            <Input
              id={`response-header-${i}`}
              class="min-w-0 flex-1"
              aria-label="Header name"
              bind:value={h.name}
              placeholder="X-Frame-Options"
              autocomplete="off"
              disabled={!canUpdate}
            />
            <Input
              class="min-w-0 flex-1"
              aria-label="Header value"
              bind:value={h.value}
              placeholder="DENY"
              autocomplete="off"
              disabled={!canUpdate}
            />
            {#if canUpdate}
              <Button aria-label="Remove header" onclick={() => (headers = headers.filter((x) => x.key !== h.key))}>Remove</Button>
            {/if}
          </div>
        {:else}
          <p class="text-sm text-muted-foreground">No custom response headers.</p>
        {/each}
      </div>
      <FieldError error={errors.response_headers} />
      {#if canUpdate && headers.length < 20}
        <Button onclick={() => headers.push({ key: nextKey++, name: '', value: '' })}>Add header</Button>
      {/if}
    </SettingsGroup>
  </form>
</div>
