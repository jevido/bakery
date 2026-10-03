<script lang="ts">
  // Every component of lib/ui in its variants, for comparing with Coolify's
  // Blade components in both themes. Dev builds only: the router answers
  // `notfound` for #/dev/components in production and App.svelte imports
  // this page behind the same check.
  import Button from '../../lib/ui/Button.svelte'
  import Callout from '../../lib/ui/Callout.svelte'
  import Checkbox from '../../lib/ui/Checkbox.svelte'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import Dropdown from '../../lib/ui/Dropdown.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import Helper from '../../lib/ui/Helper.svelte'
  import Input from '../../lib/ui/Input.svelte'
  import Modal from '../../lib/ui/Modal.svelte'
  import SectionHeading from '../../lib/ui/SectionHeading.svelte'
  import Select from '../../lib/ui/Select.svelte'
  import StatusBadge, { containerStatus } from '../../lib/ui/StatusBadge.svelte'
  import Textarea from '../../lib/ui/Textarea.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'

  $effect(() => breadcrumb.set({ label: 'Components' }))

  let name = $state('whoami')
  let password = $state('s3cret-value')
  let dockerfile = $state('FROM nginx:alpine\nCOPY . /usr/share/nginx/html')
  let buildPack = $state('nixpacks')
  let autoDeploy = $state(true)
  let instantSaves = $state(0)
  let deleted = $state<string[] | null>(null)
  let loading = $state(false)
  let modalOpen = $state(false)

  function fakeSave() {
    loading = true
    setTimeout(() => {
      loading = false
      toast.success('Saved', 'The configuration was saved.')
    }, 800)
  }
</script>

<div class="chrome mx-auto flex max-w-4xl flex-col gap-10 pb-20" data-testid="components-page">
  <div>
    <h1 class="text-2xl font-semibold text-black dark:text-white">Components</h1>
    <p class="mt-1 text-sm text-neutral-500 dark:text-fg-dim">Every component of the kit, in each of its variants.</p>
  </div>

  <section id="buttons">
    <SectionHeading title="Button" subtitle="forms/button: default, highlighted, error, disabled and loading" />
    <div class="flex flex-wrap items-center gap-2">
      <Button>Default</Button>
      <Button variant="highlighted">Highlighted</Button>
      <Button variant="error">Error</Button>
      <Button disabled>Disabled</Button>
      <Button variant="highlighted" {loading} onclick={fakeSave}>Save</Button>
    </div>
  </section>

  <section id="inputs">
    <SectionHeading title="Input, Textarea, Select" subtitle="forms/input, forms/textarea, forms/select" />
    <div class="grid gap-4 md:grid-cols-2">
      <Input label="Name" required bind:value={name} helper="The name of the resource, shown everywhere." />
      <Input label="Password" type="password" bind:value={password} />
      <Input label="Read-only" value="cannot change this" readonly />
      <Input label="With error" value="bad value" error="The value is not valid." />
      <Select label="Build Pack" bind:value={buildPack} helper="How the image is built.">
        <option value="nixpacks">Nixpacks</option>
        <option value="dockerfile">Dockerfile</option>
        <option value="static">Static</option>
      </Select>
      <Input label="Disabled" value="disabled" disabled />
    </div>
    <div class="mt-4">
      <Textarea label="Dockerfile" monospace allowTab rows={4} bind:value={dockerfile} />
    </div>
  </section>

  <section id="checkboxes">
    <SectionHeading title="Checkbox" subtitle="forms/checkbox, with an instant-save callback" />
    <div class="flex max-w-md flex-col">
      <Checkbox
        label="Auto Deploy"
        helper="Deploy on every push to the branch."
        bind:checked={autoDeploy}
        instantSave={(v) => {
          instantSaves++
          toast.success(v ? 'Auto Deploy enabled' : 'Auto Deploy disabled')
        }}
      />
      <Checkbox label="Disabled" disabled />
      <p class="px-2.5 text-xs text-neutral-500 dark:text-fg-dim" data-testid="instant-saves">Instant saves: {instantSaves}</p>
    </div>
  </section>

  <section id="helper">
    <SectionHeading title="Helper" subtitle="helper: hover or tap the icon" />
    <div class="flex items-center gap-2 text-sm">Health check <Helper helper="The path The Bakery calls to see if the container is ready." /></div>
  </section>

  <section id="badges">
    <SectionHeading title="StatusBadge" subtitle="status-badge and status/*" />
    <div class="flex flex-wrap gap-2">
      <StatusBadge {...containerStatus('running:healthy')} />
      <StatusBadge {...containerStatus('running:unknown')} />
      <StatusBadge {...containerStatus('restarting')} />
      <StatusBadge {...containerStatus('degraded:unhealthy')} />
      <StatusBadge {...containerStatus('exited')} />
      <StatusBadge status="Failed" type="error" />
      <StatusBadge status="Refresh" onclick={() => toast.info('Refreshing status')} class="cursor-pointer border-transparent hover:bg-neutral-200 dark:hover:bg-coolgray-300" />
    </div>
  </section>

  <section id="callouts">
    <SectionHeading title="Callout" subtitle="callout: info, warning, danger, success" />
    <div class="flex flex-col gap-3">
      <Callout type="info" title="Info">Containers on this server are managed by The Bakery.</Callout>
      <Callout type="warning" title="Warning">The disk is almost full.</Callout>
      <Callout type="danger" title="Danger" ondismiss={() => toast.info('Dismissed')}>The deployment failed.</Callout>
      <Callout type="success" title="Success">The backup finished.</Callout>
    </div>
  </section>

  <section id="empty">
    <SectionHeading title="Empty" subtitle="empty" />
    <Empty title="No resources found" description="Add an application, a database or a service to this environment." icon="layers">
      <Button variant="highlighted">+ Add Resource</Button>
    </Empty>
  </section>

  <section id="dropdown">
    <SectionHeading title="Dropdown" subtitle="dropdown">
      {#snippet actions()}
        <Dropdown title="Actions" triggerClass="button">
          {#snippet children(close)}
            <button type="button" class="dropdown-item" onclick={() => (close(), toast.info('Restarting'))}>Restart</button>
            <button type="button" class="dropdown-item" onclick={() => (close(), toast.warning('Stopping'))}>Stop</button>
          {/snippet}
        </Dropdown>
      {/snippet}
    </SectionHeading>
  </section>

  <section id="toasts">
    <SectionHeading title="Toast" subtitle="toast: leaves after 4 s, hovering holds it" />
    <div class="flex flex-wrap gap-2">
      <Button onclick={() => toast.success('Deployment started')}>Success</Button>
      <Button onclick={() => toast.info('Status refreshed', 'All containers are running.')}>Info</Button>
      <Button onclick={() => toast.warning('High disk usage')}>Warning</Button>
      <Button onclick={() => toast.error('Deployment failed', 'exit status 1')}>Error</Button>
    </div>
  </section>

  <section id="modals">
    <SectionHeading title="Modal, ConfirmationModal" subtitle="modal-input, modal-confirmation" />
    <div class="flex flex-wrap gap-2">
      <Modal title="New Environment" subtitle="Environments group resources." buttonTitle="+ Add" variant="highlighted" bind:open={modalOpen}>
        <form
          class="flex flex-col gap-4"
          onsubmit={(e) => {
            e.preventDefault()
            modalOpen = false
            toast.success('Environment created')
          }}
        >
          <Input label="Name" required placeholder="staging" />
          <Button type="submit" variant="highlighted">Continue</Button>
        </form>
      </Modal>
      <ConfirmationModal
        title="Confirm Application Deletion?"
        buttonTitle="Delete"
        variant="error"
        checkboxes={[
          { id: 'delete_volumes', label: 'Permanently delete all volumes associated with this resource.', checked: true },
          { id: 'delete_configurations', label: 'Permanently delete all configuration files from the server.', checked: true },
        ]}
        actions={['This application will be deleted.', 'All deployments and their logs will be deleted.']}
        confirmationText="whoami"
        confirmationLabel="Please confirm the execution of the actions by entering the Application Name below"
        shortConfirmationLabel="Application Name"
        onconfirm={(selected) => {
          deleted = selected
          toast.success('Deleted', `whoami (${selected.join(', ') || 'nothing else'})`)
        }}
      />
    </div>
    {#if deleted}
      <p class="mt-2 text-xs text-neutral-500 dark:text-fg-dim" data-testid="deleted">Confirmed with: {deleted.join(', ') || 'none'}</p>
    {/if}
  </section>
</div>
