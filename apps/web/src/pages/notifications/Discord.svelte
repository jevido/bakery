<script lang="ts">
  // Coolify's Discord page (resources/views/livewire/notifications/discord.blade.php,
  // app/Livewire/Notifications/Discord.php, Apache-2.0, see NOTICE).
  import Input from '../../lib/ui/Input.svelte'
  import Select from '../../lib/ui/Select.svelte'
  import ChannelPage from './ChannelPage.svelte'
  import { storedHost, type KindPageProps } from './channelForm'

  let props: KindPageProps = $props()
</script>

<ChannelPage kind="discord" title="Discord" description="Send guild notifications to a Discord channel through an incoming webhook." {...props}>
  {#snippet fields({ form, errors, channel, instant })}
    <div class="grid gap-4 sm:grid-cols-2">
      <Select
        label="Critical event mention"
        helper="Mention @here when a critical event occurs."
        value={String(form.ping)}
        onchange={(e) => instant({ ping: e.currentTarget.value === 'true' })}
        error={errors.ping}
        data-testid="discord-ping"
      >
        <option value="true">Mention @here</option>
        <option value="false">Do not mention</option>
      </Select>
      <div class="sm:col-span-2">
        <Input
          type="password"
          label="Webhook URL"
          helper="Create an incoming webhook in your Discord server settings."
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
