<script lang="ts">
  // Keys & Tokens as a Paperclip settings page (MIT, see NOTICE): The Bakery
  // has only API Tokens, so the one-item Keys & Tokens sidebar Coolify draws
  // (resources/views/components/security/settings-layout.blade.php,
  // Apache-2.0, see NOTICE) goes; the settings sidebar already links here.
  // `page` and `SecurityPage` stay so old /security/{page} links still route.
  import { KeyRound } from '@lucide/svelte'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import SettingsPage from '../../lib/settings/SettingsPage.svelte'
  import type { SecurityPage } from '../../lib/router.svelte'
  import ApiTokens from './ApiTokens.svelte'

  let { page }: { page: SecurityPage } = $props()

  $effect(() => {
    void page
    breadcrumb.set({ label: 'Keys & Tokens' })
  })
</script>

<SettingsPage icon={KeyRound} title="Keys & Tokens">
  <ApiTokens />
</SettingsPage>
