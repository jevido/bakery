<script lang="ts">
  // Coolify's Webhook page (resources/views/livewire/notifications/webhook.blade.php,
  // app/Livewire/Notifications/Webhook.php, Apache-2.0, see NOTICE), with The
  // Bakery's signing secret below the URL.
  import Input from '../../lib/ui/Input.svelte'
  import ChannelPage from './ChannelPage.svelte'
  import { storedHost, type KindPageProps } from './channelForm'

  let props: KindPageProps = $props()
</script>

<ChannelPage kind="webhook" title="Webhook" description="Send JSON event payloads to your own HTTP endpoint." {...props}>
  {#snippet fields({ form, errors, channel })}
    <div class="grid gap-4 lg:grid-cols-2">
      <div class="lg:col-span-2">
        <Input
          type="password"
          label="Webhook URL"
          helper="The Bakery sends POST requests to this HTTP or HTTPS endpoint."
          bind:value={form.url}
          error={errors.url}
          placeholder={storedHost(channel)}
          required={!channel?.settings.url_host}
          autocomplete="new-password"
          data-testid="webhook-url"
        />
      </div>
      <div class="lg:col-span-2">
        <Input
          type="password"
          label="Signing secret"
          helper="Optional. With a secret, each request carries the header X-Bakery-Signature: sha256=… , the HMAC-SHA256 of the body."
          bind:value={form.secret}
          error={errors.secret}
          placeholder={channel?.settings.has_secret ? 'Saved' : 'Optional'}
          autocomplete="new-password"
          data-testid="webhook-secret"
        />
      </div>
    </div>
  {/snippet}
</ChannelPage>
