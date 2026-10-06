<script lang="ts">
  // Coolify's Create Team page (resources/views/livewire/team/create.blade.php,
  // Apache-2.0, see NOTICE) as #/guild/new, and Coolify's Select Team page
  // (SelectTeam) for an account in no Guild, which can only make one here.
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import Create from './Create.svelte'

  let { noGuild = false }: { noGuild?: boolean } = $props()

  $effect(() => breadcrumb.set({ label: 'New Guild' }))
</script>

<section class="chrome mx-auto flex w-full max-w-xl flex-col gap-6" data-testid={noGuild ? 'no-guild' : 'guild-new'}>
  <header>
    <h1 class="text-xl font-semibold text-black dark:text-fg">{noGuild ? 'You are in no guild' : 'New Guild'}</h1>
    <p class="mt-1 text-sm text-neutral-500 dark:text-fg-dim">
      {#if noGuild}
        A guild owns projects, servers and everything in them. Create one, or ask a guild's admin to invite you.
      {:else}
        A guild owns its own projects, servers, notification channels and members. You become its admin.
      {/if}
    </p>
  </header>
  <Create />
</section>
