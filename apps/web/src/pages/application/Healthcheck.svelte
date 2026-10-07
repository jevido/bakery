<script lang="ts">
  // Coolify's Healthcheck page (resources/views/livewire/project/shared/health-checks.blade.php,
  // app/Livewire/Project/Shared/HealthChecks.php; Apache-2.0, see NOTICE):
  // Enable (behind a confirmation) and Disable save at once, as Coolify's
  // toggleHealthcheck; the request and timing fields wait for the unsaved
  // bar. Only the fields The Bakery's Healthcheck has: a GET on the path, at
  // the Application's port from inside the container, healthy on 2xx or 3xx.
  // Left out: the check type (container command), method, scheme, host,
  // port, expected code and response text.
  import { untrack } from 'svelte'
  import { api, ApiError } from '../../lib/api'
  import { projectAccess } from '../../lib/projectAccess.svelte'
  import type { Application, HealthCheck } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import Input from '../../lib/ui/Input.svelte'
  import SettingsSection from '../../lib/ui/SettingsSection.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import UnsavedBar from '../../lib/ui/UnsavedBar.svelte'
  import { applicationInput } from './applicationInput'

  let {
    application,
    status,
    onchange,
  }: { application: Application; status: string | null; onchange: (a: Application) => void } = $props()

  const canUpdate = $derived(projectAccess.can('manage_applications'))
  const saved = $derived(application.health_check)

  let path = $state('')
  // Number inputs bind numbers (null when empty); compared as text.
  let interval = $state<string | number | null>('')
  let timeout = $state<string | number | null>('')
  let retries = $state<string | number | null>('')
  let startPeriod = $state<string | number | null>('')
  let errors = $state<Record<string, string>>({})
  let saving = $state(false)

  function reset() {
    const h = saved
    path = h.path
    interval = String(h.interval)
    timeout = String(h.timeout)
    retries = String(h.retries)
    startPeriod = String(h.start_period)
    errors = {}
  }

  $effect(() => {
    void application.id
    untrack(reset)
  })

  const text = (v: string | number | null) => (v == null ? '' : String(v))

  const dirty = $derived(
    canUpdate &&
      (path !== saved.path ||
        text(interval) !== String(saved.interval) ||
        text(timeout) !== String(saved.timeout) ||
        text(retries) !== String(saved.retries) ||
        text(startPeriod) !== String(saved.start_period)),
  )

  async function put(health_check: HealthCheck) {
    onchange(
      (await api<{ application: Application }>('PATCH', `/applications/${application.id}`, { ...applicationInput(application), health_check }))
        .application,
    )
  }

  async function save() {
    if (saving || !dirty) return
    saving = true
    errors = {}
    try {
      await put({
        enabled: saved.enabled,
        path,
        interval: Number(interval),
        timeout: Number(timeout),
        retries: Number(retries),
        start_period: Number(startPeriod),
      })
      reset()
      toast.success('Healthcheck updated.')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      errors = Object.keys(err.errors).length ? err.errors : { 'health_check.path': err.message }
    } finally {
      saving = false
    }
  }

  // Saves only the switch: fields being edited stay in the unsaved bar.
  async function toggle() {
    const enabled = !saved.enabled
    try {
      await put({ ...saved, enabled })
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      toast.error('Healthcheck not updated', err.message)
      throw err
    }
    // The running container was started without the check; a Restart
    // (a restart Deployment) puts it through it.
    if (enabled && status?.startsWith('running'))
      toast.info('Healthcheck has been enabled. A restart is required to apply the new settings.')
    else toast.success(`Healthcheck ${enabled ? 'enabled' : 'disabled'}.`)
  }

  const fieldError = (key: string) => errors[`health_check.${key}`]
</script>

<form
  class="application-settings-form flex flex-col gap-6"
  onsubmit={(e) => {
    e.preventDefault()
    save()
  }}
>
  {#if canUpdate}
    <UnsavedBar {dirty} {saving} onsave={save} onreset={reset} />
  {/if}

  <SettingsSection
    id="healthcheck-configuration-section"
    title="Healthcheck"
    helper="Define how The Bakery determines whether this application is ready to receive traffic."
  >
    {#snippet actions()}
      {#if !saved.enabled}
        <ConfirmationModal
          title="Enable healthcheck?"
          buttonTitle="Enable"
          variant="highlighted"
          disabled={!canUpdate}
          actions={['Enable healthcheck for this resource.']}
          warningMessage="If the healthcheck fails, your application will become inaccessible: a new deployment only takes traffic once the check passes."
          step2ButtonText="Enable healthcheck"
          confirmWithText={false}
          onconfirm={toggle}
        />
      {:else}
        <Button disabled={!canUpdate} onclick={() => toggle().catch(() => {})}>Disable</Button>
      {/if}
    {/snippet}
    <p class="text-[13px] leading-5 text-neutral-600 dark:text-fg-dim">
      {#if saved.enabled}
        Enabled. Each new container is checked before it takes traffic; a deployment whose container never turns healthy fails and the
        running one stays.
      {:else}
        Disabled. A new container takes traffic as soon as it starts.
      {/if}
    </p>
  </SettingsSection>

  <SettingsSection
    id="healthcheck-request-section"
    title="HTTP request"
    helper="The Bakery sends a GET request from inside the container, to the port the application listens on, and evaluates the response. The image needs curl or wget."
  >
    <Input
      label="Path"
      bind:value={path}
      error={fieldError('path')}
      placeholder="/health"
      required
      disabled={!canUpdate}
      helper="Healthy when it answers with a 2xx or 3xx status."
    />
  </SettingsSection>

  <SettingsSection
    id="healthcheck-timing-section"
    title="Timing and retries"
    helper="Control how quickly healthchecks start, repeat, and fail."
  >
    <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <Input
        type="number"
        min="1"
        label="Interval (seconds)"
        bind:value={interval}
        error={fieldError('interval')}
        placeholder="5"
        required
        disabled={!canUpdate}
      />
      <Input
        type="number"
        min="1"
        label="Timeout (seconds)"
        bind:value={timeout}
        error={fieldError('timeout')}
        placeholder="5"
        required
        disabled={!canUpdate}
      />
      <Input
        type="number"
        min="1"
        label="Retries"
        bind:value={retries}
        error={fieldError('retries')}
        placeholder="10"
        required
        disabled={!canUpdate}
      />
      <Input
        type="number"
        min="0"
        label="Start period (seconds)"
        bind:value={startPeriod}
        error={fieldError('start_period')}
        placeholder="0"
        required
        disabled={!canUpdate}
      />
    </div>
  </SettingsSection>
</form>
