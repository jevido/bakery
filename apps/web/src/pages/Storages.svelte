<script lang="ts">
  import { breadcrumb } from '../lib/breadcrumb.svelte'
  import S3Storages from '../lib/S3Storages.svelte'
  import { session } from '../lib/session.svelte'

  $effect(() => breadcrumb.set({ label: 'S3 Storage' }))
</script>

<h1>S3 Storage</h1>

{#if !session.isAdmin}
  <p class="muted">S3 storages are managed by admins.</p>
{:else}
  <S3Storages />
{/if}
