<script lang="ts">
  // Coolify's Storages page (resources/views/livewire/storage/index.blade.php,
  // Apache-2.0, see NOTICE): "S3 Storage" with "New storage" in its header.
  //
  // Laid out as Paperclip's settings pages (ui/src/pages/CompanySettings.tsx,
  // MIT, see NOTICE): a PageHeader with the add action; the list and form
  // live in S3Storages, which this page only gates by permission.
  import { breadcrumb } from '../lib/breadcrumb.svelte'
  import Icon from '../lib/Icon.svelte'
  import PageHeader from '../lib/PageHeader.svelte'
  import S3Storages from '../lib/S3Storages.svelte'
  import { session } from '../lib/session.svelte'
  import type { S3Storage } from '../lib/types'
  import Button from '../lib/ui/Button.svelte'
  import Empty from '../lib/ui/Empty.svelte'

  let editing = $state<S3Storage | 'new' | null>(null)

  $effect(() => breadcrumb.set({ label: 'S3 Storage' }))
</script>

<div class="chrome w-full space-y-4">
  <PageHeader title="S3 Storage">
    {#snippet actions()}
      {#if session.can('manage_servers')}
        <Button variant="highlighted" onclick={() => (editing = 'new')}>
          <Icon name="plus" class="size-3.5" />
          Add
        </Button>
      {/if}
    {/snippet}
  </PageHeader>

  {#if !session.can('manage_servers')}
    <Empty
      title="S3 Storage needs the Manage servers permission"
      description="Ask someone in this guild who has it to add or change S3 storage destinations."
      icon="storages"
    />
  {:else}
    <S3Storages bind:editing />
  {/if}
</div>
