<script lang="ts">
  // Coolify's Storages list and form (resources/views/livewire/storage/index.blade.php
  // and storage/form.blade.php, app/Livewire/Storage/*.php; Apache-2.0, see
  // NOTICE): every field, the connection test and delete, with description
  // left out (S3Storage has none here).
  //
  // Laid out as Paperclip's company settings (ui/src/pages/CompanySettings.tsx,
  // MIT, see NOTICE): the Storages as EntityRows in one bordered card, the
  // add/edit form as a SettingsGroup below it.
  import { Card } from '$lib/components/ui/card'
  import { api, ApiError } from './api'
  import EntityRow from './EntityRow.svelte'
  import Icon from './Icon.svelte'
  import PageSkeleton from './PageSkeleton.svelte'
  import ProjectTile from './ProjectTile.svelte'
  import SettingsGroup from './settings/SettingsGroup.svelte'
  import type { S3Storage, S3StorageInput } from './types'
  import Button from './ui/Button.svelte'
  import Callout from './ui/Callout.svelte'
  import ConfirmationModal from './ui/ConfirmationModal.svelte'
  import Empty from './ui/Empty.svelte'
  import Input from './ui/Input.svelte'

  let { editing = $bindable(null) }: { editing?: S3Storage | 'new' | null } = $props()

  let storages = $state.raw<S3Storage[] | null>(null)
  let loadError = $state('')

  let form = $state<S3StorageInput>(blank())
  let errors = $state<Record<string, string>>({})
  let message = $state('')
  let check = $state<{ ok: boolean; message?: string } | null>(null)
  let busy = $state(false)

  function blank(): S3StorageInput {
    return { name: '', endpoint: '', region: '', bucket: '', prefix: '', access_key: '', secret_key: '' }
  }

  async function load() {
    const r = await api<{ s3_storages: S3Storage[] }>('GET', '/s3-storages')
    storages = r.s3_storages
  }
  load().catch((e) => (loadError = e.message))

  // Resets the form whenever `editing` changes, however it changed: a click
  // on a row, Storages' "Add", or Cancel/Save closing it.
  $effect(() => {
    const e = editing
    form = e === 'new' ? blank() : e ? { ...e, secret_key: '' } : blank()
    errors = {}
    message = ''
    check = null
  })

  function show(err: unknown) {
    if (!(err instanceof ApiError)) throw err
    errors = err.errors
    message = Object.keys(err.errors).length === 0 ? err.message : ''
  }

  async function test() {
    busy = true
    errors = {}
    message = ''
    check = null
    try {
      const id = editing && editing !== 'new' ? editing.id : undefined
      check = await api<{ ok: boolean; message?: string }>('POST', '/s3-storages/check', { ...form, id })
    } catch (err) {
      show(err)
    } finally {
      busy = false
    }
  }

  async function save(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    errors = {}
    message = ''
    try {
      if (editing === 'new') await api('POST', '/s3-storages', form)
      else if (editing) await api('PATCH', `/s3-storages/${editing.id}`, form)
      editing = null
      await load()
    } catch (err) {
      show(err)
    } finally {
      busy = false
    }
  }

  async function remove(s: S3Storage) {
    try {
      await api('DELETE', `/s3-storages/${s.id}`)
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      loadError = err.message
      return
    }
    if (editing !== 'new' && editing?.id === s.id) editing = null
    await load()
  }
</script>

{#if loadError}
  <p class="text-sm text-destructive">{loadError}</p>
{:else if storages === null}
  <PageSkeleton />
{:else}
  {#if storages.length === 0}
    {#if editing === null}
      <Empty title="No S3 storage yet" description="Add an S3-compatible destination to store backups outside your servers." icon="storages" />
    {/if}
  {:else}
    <Card class="block gap-0 overflow-hidden py-0">
      {#each storages as s (s.id)}
        <EntityRow
          title={s.name}
          subtitle={`${s.endpoint} · ${s.bucket}`}
          onclick={() => (editing = s)}
          selected={editing !== 'new' && editing?.id === s.id}
          data-testid="s3-storage"
        >
          {#snippet leading()}<ProjectTile size="sm" icon="storages" />{/snippet}
          {#snippet trailing()}
            <ConfirmationModal
              title="Delete the S3 storage {s.name}?"
              actions={['This S3 storage will be removed. Backups already uploaded stay in the bucket.']}
              confirmWithText={false}
              step2ButtonText="Delete"
              onconfirm={() => remove(s)}
            >
              {#snippet trigger(show)}
                <button
                  type="button"
                  onclick={(e) => {
                    e.stopPropagation()
                    show()
                  }}
                  class="inline-flex size-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-destructive/10 hover:text-destructive"
                  aria-label="Delete {s.name}"
                >
                  <Icon name="trash" class="size-3.5" />
                </button>
              {/snippet}
            </ConfirmationModal>
          {/snippet}
        </EntityRow>
      {/each}
    </Card>
  {/if}

  {#if editing !== null}
    <form onsubmit={save}>
      <SettingsGroup id="s3-storage-form" label={editing === 'new' ? 'Add S3 storage' : `Edit ${editing.name}`}>
        {#snippet actions()}
          <Button type="button" onclick={test} loading={busy} data-testid="s3-storage-test">
            <Icon name="check-circle" class="size-3.5" />
            Test connection
          </Button>
        {/snippet}

        <Input label="Name" bind:value={form.name} error={errors.name} required />
        <Input label="Endpoint" bind:value={form.endpoint} error={errors.endpoint} placeholder="https://s3.eu-west-1.amazonaws.com" required />
        <div class="grid gap-4 sm:grid-cols-2">
          <Input label="Region" bind:value={form.region} error={errors.region} placeholder="us-east-1" />
          <Input label="Bucket" bind:value={form.bucket} error={errors.bucket} required />
        </div>
        <Input label="Prefix" helper="A folder in the bucket, optional." bind:value={form.prefix} error={errors.prefix} />
        <div class="grid gap-4 sm:grid-cols-2">
          <Input label="Access key" bind:value={form.access_key} error={errors.access_key} autocomplete="off" required />
          <Input
            label="Secret key"
            type="password"
            bind:value={form.secret_key}
            error={errors.secret_key}
            autocomplete="new-password"
            placeholder={editing !== 'new' ? 'unchanged' : ''}
            required={editing === 'new'}
          />
        </div>

        {#if check}
          <div data-testid="s3-storage-check">
            <Callout type={check.ok ? 'success' : 'danger'} title={check.ok ? 'Connected' : 'Connection failed'}>
              {check.ok ? 'The Bakery can reach the bucket.' : check.message}
            </Callout>
          </div>
        {/if}
        {#if message}<p class="text-sm text-destructive">{message}</p>{/if}

        <div class="flex justify-end gap-2 border-t border-border pt-4">
          <Button type="button" onclick={() => (editing = null)}>Cancel</Button>
          <Button type="submit" variant="highlighted" loading={busy} data-testid="s3-storage-save">Save</Button>
        </div>
      </SettingsGroup>
    </form>
  {/if}
{/if}
