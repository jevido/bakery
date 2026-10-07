<script lang="ts">
  // Coolify's Create Team page (resources/views/livewire/team/create.blade.php,
  // Apache-2.0, see NOTICE) as #/guild/new, and Coolify's Select Team page
  // (SelectTeam) for an account in no Guild, which can only make one here,
  // laid out as Paperclip's new-company page (ui/src/pages/CompanySettings.tsx's
  // frame; MIT, see NOTICE).
  import { Plus } from '@lucide/svelte'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import Create from './Create.svelte'

  let { noGuild = false }: { noGuild?: boolean } = $props()

  $effect(() => breadcrumb.set({ label: 'New Guild' }))
</script>

<section class="chrome max-w-2xl space-y-8" data-testid={noGuild ? 'no-guild' : 'guild-new'}>
  <div class="space-y-2">
    <div class="flex items-center gap-2">
      <Plus class="size-5 text-muted-foreground" />
      <h1 class="text-lg font-semibold">{noGuild ? 'You are in no guild' : 'New Guild'}</h1>
    </div>
    <p class="text-sm text-muted-foreground">
      {#if noGuild}
        A guild owns projects, servers and everything in them. Create one, or ask a guild's admin to invite you.
      {:else}
        A guild owns its own projects, servers, notification channels and members. You become its Guild Master.
      {/if}
    </p>
  </div>
  <div class="rounded-xl border border-border p-5">
    <Create />
  </div>
</section>
