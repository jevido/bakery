<script lang="ts">
  // Coolify's Persistent Storage (resources/views/livewire/project/service/storage.blade.php,
  // shared/storages/all.blade.php and show.blade.php, app/Livewire/Project/Service/Storage.php
  // and Shared/Storages/*.php; Apache-2.0, see NOTICE): a resource's volumes.
  // With onsave (an Application) each row is editable and Add mount →
  // Volume mount adds one; containers mount them from the next deployment.
  // Without it (a Database's own data volume) the rows are read-only and
  // have no actions column, as Coolify shows a Database's default volume.
  // Left out: File, Host file and Directory mounts (bind mounts), the
  // Volumes/Files/Directories tabs, volume backups and the preview suffix.
  import { untrack } from 'svelte'
  import { ApiError } from '../../lib/api'
  import Icon from '../../lib/Icon.svelte'
  import { session } from '../../lib/session.svelte'
  import type { Storage } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import FieldError from '../../lib/ui/FieldError.svelte'
  import Input from '../../lib/ui/Input.svelte'
  import Modal from '../../lib/ui/Modal.svelte'
  import SettingsSection from '../../lib/ui/SettingsSection.svelte'
  import { toast } from '../../lib/ui/toast.svelte'

  let {
    storages,
    onsave,
    helper = 'Mount volumes to preserve data between deployments. A volume is named after its storage, so a renamed storage starts empty; the data stays under the old name.',
  }: {
    storages: Storage[]
    /** Saves the whole list; throws ApiError when the API refuses it. */
    onsave?: (list: Storage[]) => Promise<void>
    helper?: string
  } = $props()

  const canUpdate = $derived(session.can('manage_applications') && !!onsave)

  // The rows being edited, as typed; reset whenever the saved list changes.
  let forms = $state<Storage[]>([])
  let rowErrors = $state<string[]>([])
  $effect(() => {
    const list = storages
    untrack(() => {
      forms = list.map((s) => ({ ...s }))
      rowErrors = list.map(() => '')
    })
  })

  let menuOpen = $state(false)
  let adding = $state(false)
  let name = $state('')
  let mountPath = $state('')
  let addError = $state('')
  let saving = $state(false)

  // Saves the list; the API's message, when it refuses, is returned.
  async function save(list: Storage[]): Promise<string> {
    try {
      await onsave?.(list)
      return ''
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      return err.errors.storages ?? err.message
    }
  }

  function openAdd() {
    menuOpen = false
    name = ''
    mountPath = ''
    addError = ''
    adding = true
  }

  async function add(e: SubmitEvent) {
    e.preventDefault()
    if (saving) return
    saving = true
    addError = await save([...storages, { name: name.trim(), mount_path: mountPath.trim() }])
    saving = false
    if (addError) return
    adding = false
    toast.success('Storage added.', 'Redeploy to mount it.')
  }

  async function update(e: SubmitEvent, index: number) {
    e.preventDefault()
    if (saving) return
    saving = true
    const f = forms[index]
    const message = await save(storages.map((s, i) => (i === index ? { name: f.name.trim(), mount_path: f.mount_path.trim() } : s)))
    saving = false
    if (message) {
      rowErrors[index] = message
      return
    }
    toast.success('Storage updated.', 'Redeploy to apply the change.')
  }

  async function remove(index: number) {
    const message = await save(storages.filter((_, i) => i !== index))
    if (message) {
      toast.error('Storage not deleted', message)
      throw new Error(message)
    }
    toast.success('Storage deleted.', 'Its volume and data are kept.')
  }

  const grid = $derived(onsave ? 'volumes-table-grid' : 'volumes-table-grid-readonly')

  const changed = (i: number) => forms[i] && (forms[i].name !== storages[i]?.name || forms[i].mount_path !== storages[i]?.mount_path)
</script>

<svelte:window onclick={() => (menuOpen = false)} />

<div class="chrome">
  <SettingsSection
    id="storage-mounts-section"
    title="Persistent storage"
    {helper}
    flush
  >
    {#snippet actions()}
      {#if canUpdate && storages.length < 10}
        <div class="relative">
          <button
            type="button"
            class="button button-highlighted"
            aria-haspopup="menu"
            aria-expanded={menuOpen}
            onclick={(e) => {
              e.stopPropagation()
              menuOpen = !menuOpen
            }}
          >
            <Icon name="plus" class="size-3.5" />
            Add mount
            <Icon name="chevron-down" class="size-3 opacity-55" />
          </button>
          {#if menuOpen}
            <div role="menu" class="listbox-panel right-auto! left-0! z-[90]! w-52! min-w-52! sm:right-0! sm:left-auto!">
              <button type="button" class="listbox-option justify-start! gap-2.5!" role="menuitem" onclick={openAdd}>
                <Icon name="storages" class="size-3.5 shrink-0 opacity-70" />
                Volume mount
              </button>
            </div>
          {/if}
        </div>
      {/if}
    {/snippet}

    {#if storages.length === 0}
      <div class="p-4">
        <Empty size="sm" title="No persistent storage" description="Add a volume mount to preserve data between deployments." icon="storages" />
      </div>
    {:else}
      <div class="data-table w-full">
        <div class="data-table-header {grid}">
          <span>Volume Name</span>
          <span>Destination Path</span>
          {#if onsave}<span class="text-right">Actions</span>{/if}
        </div>
        {#each storages as s, i (s.name)}
          {#if canUpdate && forms[i]}
            <form class="env-table-item" onsubmit={(e) => update(e, i)}>
              <div class="data-table-row volumes-table-grid">
                <div class="volumes-cell-name min-w-0">
                  <span class="volumes-mobile-label">Volume Name</span>
                  <Input aria-label="Volume Name" bind:value={forms[i].name} required />
                </div>
                <div class="volumes-cell-dest min-w-0">
                  <span class="volumes-mobile-label">Destination Path</span>
                  <Input aria-label="Destination Path" bind:value={forms[i].mount_path} required placeholder="/path/in/container" />
                </div>
                <div class="volumes-cell-actions flex flex-nowrap items-center justify-end gap-1.5">
                  <Button type="submit" class="px-2.5! text-xs!" disabled={!changed(i)}>Update</Button>
                  <ConfirmationModal
                    title="Confirm persistent storage deletion?"
                    buttonTitle="Delete"
                    variant="error"
                    actions={[
                      'This storage is removed from the application; containers from the next deployment no longer mount it.',
                      `Its volume and the data in it are kept: adding a storage named ${s.name} again brings the data back. Only deleting the application removes its volumes.`,
                    ]}
                    warningMessage="Running containers keep the mount until the next deployment."
                    confirmationText={s.name}
                    confirmationLabel="Please confirm the execution of the actions by entering the Storage Name below"
                    shortConfirmationLabel="Storage Name"
                    onconfirm={() => remove(i)}
                  />
                </div>
              </div>
              {#if rowErrors[i]}<div class="px-4 pb-3"><FieldError error={rowErrors[i]} /></div>{/if}
            </form>
          {:else}
            <div class="env-table-item">
              <div class="data-table-row {grid} text-[13px] text-neutral-700 dark:text-fg-dim">
                <div class="volumes-cell-name min-w-0">
                  <span class="volumes-mobile-label">Volume Name</span>
                  <span class="min-w-0 truncate text-[13px] font-medium text-neutral-950 dark:text-fg" title={s.name}>{s.name}</span>
                </div>
                <div class="volumes-cell-dest min-w-0">
                  <span class="volumes-mobile-label">Destination Path</span>
                  <span class="block min-w-0 truncate text-[13px] text-neutral-950 dark:text-fg" title={s.mount_path}>{s.mount_path}</span>
                </div>
                {#if onsave}<span class="text-right text-neutral-400 dark:text-fg-faint">—</span>{/if}
              </div>
            </div>
          {/if}
        {/each}
      </div>
    {/if}
  </SettingsSection>
</div>

{#if canUpdate}
  <Modal title="Add volume mount" variant="none" bind:open={adding}>
    <form class="flex w-full flex-col gap-4" onsubmit={add}>
      <p class="text-[13px] leading-5 text-neutral-500 dark:text-fg-dim">Mount a Podman volume inside the container.</p>
      <Input placeholder="pv-name" label="Name" required bind:value={name} helper="Volume name: lowercase letters, digits and -." />
      <Input placeholder="/tmp/root" label="Destination Path" required bind:value={mountPath} helper="Directory inside the container." />
      <FieldError error={addError} />
      <div class="flex justify-end pt-2">
        <Button type="submit" loading={saving}>Add volume</Button>
      </div>
    </form>
  </Modal>
{/if}
