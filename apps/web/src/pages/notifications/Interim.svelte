<script lang="ts">
  // The Email, Telegram, Pushover and ntfy settings as the earlier
  // Notifications page had them, on the shared channel page, until each gets
  // its own port of Coolify's page.
  import Input from '../../lib/ui/Input.svelte'
  import Select from '../../lib/ui/Select.svelte'
  import type { ChannelKind } from '../../lib/types'
  import ChannelPage from './ChannelPage.svelte'
  import { kindLabels, type KindPageProps } from './channelForm'

  let { kind, ...props }: KindPageProps & { kind: Extract<ChannelKind, 'email' | 'telegram' | 'pushover' | 'ntfy'> } = $props()

  const descriptions = {
    email: 'Send team notifications by email through your own SMTP server.',
    telegram: 'Send team notifications to a Telegram chat through a bot.',
    pushover: 'Send team notifications to your devices through Pushover.',
    ntfy: 'Send team notifications to an ntfy topic.',
  }
  const saved = (set: boolean | undefined) => (set ? 'Saved' : '')
</script>

{#key kind}
  <ChannelPage {kind} title={kindLabels[kind]} description={descriptions[kind]} {...props}>
    {#snippet fields({ form, errors, channel })}
      <div class="grid gap-4 lg:grid-cols-2">
        {#if kind === 'email'}
          <Input label="SMTP server" bind:value={form.host} error={errors.host} placeholder="smtp.example.com" required />
          <Input label="Port" type="number" bind:value={form.port} error={errors.port} required />
          <Select label="Security" bind:value={form.security} error={errors.security}>
            <option value="starttls">STARTTLS</option>
            <option value="tls">TLS</option>
            <option value="none">None</option>
          </Select>
          <Input label="Username" bind:value={form.username} error={errors.username} />
          <Input
            label="Password"
            type="password"
            bind:value={form.password}
            error={errors.password}
            autocomplete="new-password"
            placeholder={saved(channel?.settings.has_password)}
          />
          <Input label="From address" type="email" bind:value={form.from} error={errors.from} placeholder="bakery@example.com" required />
          <div class="lg:col-span-2">
            <Input label="Recipients" bind:value={form.to} error={errors.to} placeholder="ops@example.com" required />
          </div>
        {:else if kind === 'telegram'}
          <Input
            label="Bot token"
            type="password"
            bind:value={form.bot_token}
            error={errors.bot_token}
            autocomplete="new-password"
            placeholder={saved(channel?.settings.has_bot_token) || '123456:ABC-DEF…'}
            required={!channel?.settings.has_bot_token}
          />
          <Input label="Chat ID" bind:value={form.chat_id} error={errors.chat_id} placeholder="-1001234567890 or @channel" required />
        {:else if kind === 'pushover'}
          <Input
            label="User key"
            type="password"
            bind:value={form.user_key}
            error={errors.user_key}
            autocomplete="new-password"
            placeholder={saved(channel?.settings.has_user_key)}
            required={!channel?.settings.has_user_key}
          />
          <Input
            label="API token"
            type="password"
            bind:value={form.api_token}
            error={errors.api_token}
            autocomplete="new-password"
            placeholder={saved(channel?.settings.has_api_token)}
            required={!channel?.settings.has_api_token}
          />
        {:else}
          <Input label="Server URL" bind:value={form.url} error={errors.url} placeholder="https://ntfy.sh" />
          <Input label="Topic" bind:value={form.topic} error={errors.topic} required />
          <Input
            label="Access token"
            type="password"
            bind:value={form.token}
            error={errors.token}
            autocomplete="new-password"
            placeholder={saved(channel?.settings.has_token) || 'Optional'}
          />
        {/if}
      </div>
    {/snippet}
  </ChannelPage>
{/key}
