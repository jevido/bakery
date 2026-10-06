<script lang="ts">
  // Coolify's Slack page (resources/views/livewire/notifications/slack.blade.php,
  // app/Livewire/Notifications/Slack.php, Apache-2.0, see NOTICE).
  import Input from '../../lib/ui/Input.svelte'
  import ChannelPage from './ChannelPage.svelte'
  import { storedHost, type KindPageProps } from './channelForm'

  let props: KindPageProps = $props()
</script>

<ChannelPage kind="slack" title="Slack" description="Send guild notifications to Slack through an incoming webhook." {...props}>
  {#snippet fields({ form, errors, channel })}
    <div class="grid gap-4 lg:grid-cols-2">
      <div class="lg:col-span-2">
        <Input
          type="password"
          label="Webhook URL"
          helper="Create an incoming webhook in your Slack app settings."
          bind:value={form.url}
          error={errors.url}
          placeholder={storedHost(channel)}
          required={!channel?.settings.url_host}
          autocomplete="new-password"
          data-testid="webhook-url"
        />
      </div>
    </div>
  {/snippet}
</ChannelPage>
