<script lang="ts">
  // The Bakery's own ntfy page, in the shape of Coolify's Notifications pages
  // (resources/views/livewire/notifications/*.blade.php, Apache-2.0, see NOTICE).
  import Input from '../../lib/ui/Input.svelte'
  import ChannelPage from './ChannelPage.svelte'
  import type { KindPageProps } from './channelForm'

  let props: KindPageProps = $props()
</script>

<ChannelPage kind="ntfy" title="ntfy" description="Send guild notifications to an ntfy topic." {...props}>
  {#snippet fields({ form, errors, channel })}
    <div class="grid gap-4 lg:grid-cols-2">
      <Input
        label="Server URL"
        helper="The ntfy server to publish to. Empty uses https://ntfy.sh."
        bind:value={form.url}
        error={errors.url}
        placeholder="https://ntfy.sh"
        data-testid="ntfy-url"
      />
      <Input label="Topic" helper="The topic your devices subscribe to." bind:value={form.topic} error={errors.topic} required data-testid="ntfy-topic" />
      <div class="lg:col-span-2">
        <Input
          type="password"
          label="Access token"
          helper="Optional. Needed when the topic is protected."
          bind:value={form.token}
          error={errors.token}
          placeholder={channel?.settings.has_token ? 'Saved' : 'Optional'}
          autocomplete="new-password"
          data-testid="ntfy-token"
        />
      </div>
    </div>
  {/snippet}
</ChannelPage>
