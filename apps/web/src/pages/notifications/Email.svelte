<script lang="ts">
  // Coolify's Email page (resources/views/livewire/notifications/email.blade.php,
  // app/Livewire/Notifications/Email.php, Apache-2.0, see NOTICE). The
  // channel's Enable/Disable stands for Coolify's "SMTP delivery" select;
  // The Bakery has no instance email service and no Resend, and sends to the
  // channel's Recipients rather than to the team's members.
  import Input from '../../lib/ui/Input.svelte'
  import Select from '../../lib/ui/Select.svelte'
  import SettingsGroup from '../../lib/settings/SettingsGroup.svelte'
  import ChannelPage from './ChannelPage.svelte'
  import type { KindPageProps } from './channelForm'

  let props: KindPageProps = $props()
</script>

<ChannelPage kind="email" title="Email delivery" {...props}>
  {#snippet fields({ form, errors })}
    <div class="grid gap-4 sm:grid-cols-2">
      <Input label="From name" helper="Name used in emails." bind:value={form.from_name} error={errors.from_name} required data-testid="email-from-name" />
      <Input
        label="From address"
        helper="Email address used in emails."
        bind:value={form.from}
        error={errors.from}
        required
        data-testid="email-from"
      />
      <div class="sm:col-span-2">
        <Input
          label="Recipients"
          helper="Notifications go to these addresses. Separate several with commas."
          bind:value={form.to}
          error={errors.to}
          placeholder="ops@example.com"
          required
          data-testid="email-to"
        />
      </div>
    </div>
  {/snippet}
  {#snippet more({ form, errors, channel })}
    <div class="mt-8">
      <SettingsGroup id="email-smtp-server" label="SMTP server" hint="Deliver messages through your own SMTP server.">
        <div class="grid gap-4 sm:grid-cols-2">
          <Input label="Host" bind:value={form.host} error={errors.host} placeholder="smtp.mailgun.org" required data-testid="email-host" />
          <Input label="Port" type="number" bind:value={form.port} error={errors.port} placeholder="587" required data-testid="email-port" />
          <Select label="Encryption" bind:value={form.security} error={errors.security} required data-testid="email-encryption">
            <option value="starttls">StartTLS</option>
            <option value="tls">TLS / SSL</option>
            <option value="none">None</option>
          </Select>
          <Input label="SMTP username" bind:value={form.username} error={errors.username} data-testid="email-username" />
          <Input
            label="SMTP password"
            type="password"
            bind:value={form.password}
            error={errors.password}
            autocomplete="new-password"
            placeholder={channel?.settings.has_password ? 'Saved' : ''}
            data-testid="email-password"
          />
          <Input
            label="Timeout"
            type="number"
            helper="Timeout value for sending emails."
            bind:value={form.timeout}
            error={errors.timeout}
            data-testid="email-timeout"
          />
          <Input
            label="EHLO domain"
            helper="Fully qualified domain sent in the SMTP EHLO command. Uses the system default when empty."
            bind:value={form.ehlo_domain}
            error={errors.ehlo_domain}
            placeholder="bakery.example.com"
            data-testid="email-ehlo"
          />
        </div>
      </SettingsGroup>
    </div>
  {/snippet}
</ChannelPage>
