<script lang="ts">
  // Coolify's Pushover page (resources/views/livewire/notifications/pushover.blade.php,
  // app/Livewire/Notifications/Pushover.php, Apache-2.0, see NOTICE).
  import Input from '../../lib/ui/Input.svelte'
  import ChannelPage from './ChannelPage.svelte'
  import type { KindPageProps } from './channelForm'

  let props: KindPageProps = $props()
</script>

<ChannelPage kind="pushover" title="Pushover" description="Deliver guild alerts through your Pushover application." {...props}>
  {#snippet fields({ form, errors, channel })}
    <div class="grid gap-4 sm:grid-cols-2">
      <Input
        type="password"
        label="User key"
        helper="Find this in the Pushover dashboard."
        bind:value={form.user_key}
        error={errors.user_key}
        placeholder={channel?.settings.has_user_key ? 'Saved' : ''}
        required={!channel?.settings.has_user_key}
        autocomplete="new-password"
        data-testid="pushover-user-key"
      />
      <Input
        type="password"
        label="API token"
        helper="Create an application in Pushover to generate this token."
        bind:value={form.api_token}
        error={errors.api_token}
        placeholder={channel?.settings.has_api_token ? 'Saved' : ''}
        required={!channel?.settings.has_api_token}
        autocomplete="new-password"
        data-testid="pushover-api-token"
      />
    </div>
  {/snippet}
</ChannelPage>
