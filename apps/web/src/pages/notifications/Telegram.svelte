<script lang="ts">
  // Coolify's Telegram page (resources/views/livewire/notifications/telegram.blade.php,
  // app/Livewire/Notifications/Telegram.php, Apache-2.0, see NOTICE), with
  // the Forum topics section of its threaded event grid.
  import Input from '../../lib/ui/Input.svelte'
  import ChannelPage from './ChannelPage.svelte'
  import type { KindPageProps } from './channelForm'

  let props: KindPageProps = $props()
</script>

<ChannelPage kind="telegram" title="Telegram" description="Deliver guild notifications through a Telegram bot and chat." threaded {...props}>
  {#snippet fields({ form, errors, channel })}
    <div class="grid gap-4 sm:grid-cols-2">
      <Input
        type="password"
        label="Bot API token"
        helper="Create a bot with BotFather to obtain this token."
        bind:value={form.bot_token}
        error={errors.bot_token}
        placeholder={channel?.settings.has_bot_token ? 'Saved' : ''}
        required={!channel?.settings.has_bot_token}
        autocomplete="new-password"
        data-testid="telegram-token"
      />
      <Input
        type="password"
        label="Chat ID"
        helper="Add the bot to your chat, then enter that chat ID."
        bind:value={form.chat_id}
        error={errors.chat_id}
        required
        autocomplete="new-password"
        data-testid="telegram-chat-id"
      />
    </div>
  {/snippet}
</ChannelPage>
