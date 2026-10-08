<script lang="ts">
  // Keys & Tokens as a Paperclip settings page (MIT, see NOTICE), with its
  // two pages as line tabs, as Paperclip's Inbox draws its tabs: API Tokens
  // (Coolify's, whose one-item Keys & Tokens sidebar in
  // resources/views/components/security/settings-layout.blade.php,
  // Apache-2.0, see NOTICE, goes; the settings sidebar already links here)
  // and Desktops, the desktop apps signed in as the person.
  import { KeyRound } from '@lucide/svelte'
  import * as Tabs from '$lib/components/ui/tabs'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import SettingsPage from '../../lib/settings/SettingsPage.svelte'
  import { go, securityPages, securityPath, type SecurityPage } from '../../lib/router.svelte'
  import ApiTokens from './ApiTokens.svelte'
  import Desktops from './Desktops.svelte'

  let { page }: { page: SecurityPage } = $props()

  const labels: Record<SecurityPage, string> = { 'api-tokens': 'API Tokens', desktops: 'Desktops' }

  $effect(() => {
    void page
    breadcrumb.set({ label: 'Keys & Tokens' })
  })
</script>

<SettingsPage icon={KeyRound} title="Keys & Tokens">
  <Tabs.Root value={page} onValueChange={(v) => go(securityPath(v as SecurityPage))}>
    <Tabs.List variant="line" class="justify-start">
      {#each securityPages as p (p)}
        <Tabs.Trigger value={p}>{labels[p]}</Tabs.Trigger>
      {/each}
    </Tabs.List>
  </Tabs.Root>
  {#if page === 'desktops'}
    <Desktops />
  {:else}
    <ApiTokens />
  {/if}
</SettingsPage>
