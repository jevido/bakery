<script lang="ts">
  // What every Notifications page of Coolify (resources/views/livewire/notifications/*.blade.php,
  // app/Livewire/Notifications/*.php, Apache-2.0, see NOTICE) shares: the
  // unsaved bar and the channel's settings group with Enable/Disable and
  // Send test, then the Notification events grid. The kind's page gives the
  // fields. The Bakery adds the channel's recent Deliveries below the grid,
  // and the picker when a kind has several channels.
  //
  // A kind with no channel shows the empty form. Saving it stores the channel
  // disabled, as Coolify saves settings without enabling them; Enable stores
  // it enabled. Toggling an event saves at once, as Coolify's toggleEvent
  // does, but from the stored settings, so unsaved edits stay unsaved.
  // An email channel's Send test asks for a recipient first, as Coolify's
  // "Send Test Email" modal does.
  import { untrack, type Snippet } from 'svelte'
  import { api, ApiError } from '../../lib/api'
  import SettingsGroup from '../../lib/settings/SettingsGroup.svelte'
  import { session } from '../../lib/session.svelte'
  import Button from '../../lib/ui/Button.svelte'
  import Input from '../../lib/ui/Input.svelte'
  import Modal from '../../lib/ui/Modal.svelte'
  import type { ChannelKind, EventKind, NotificationChannel } from '../../lib/types'
    import { toast } from '../../lib/ui/toast.svelte'
  import UnsavedBar from '../../lib/ui/UnsavedBar.svelte'
  import ChannelActions from './ChannelActions.svelte'
  import ChannelPicker from './ChannelPicker.svelte'
  import { blankForm, formOf, kindLabels, payload, sameForm, type ChannelForm } from './channelForm'
  import Deliveries from './Deliveries.svelte'
  import EventGrid from './EventGrid.svelte'

  let {
    kind,
    title,
    description,
    channels,
    allNames,
    eventKinds,
    onsaved,
    ondeleted,
    threaded = false,
    fields,
    more,
  }: {
    kind: ChannelKind
    title: string
    description?: string
    /** This kind's channels. */
    channels: NotificationChannel[]
    /** Every channel's name, of any kind; names are unique. */
    allNames: string[]
    eventKinds: EventKind[]
    onsaved: (c: NotificationChannel) => void
    ondeleted: (id: number) => void
    /** Telegram: a forum topic per selected Event kind, below the events grid. */
    threaded?: boolean
    fields: Snippet<[FieldsArgs]>
    /** Further sections of the same form, after the first (Email's SMTP server). */
    more?: Snippet<[FieldsArgs]>
  } = $props()

  type FieldsArgs = {
    form: ChannelForm
    errors: Record<string, string>
    channel: NotificationChannel | null
    /** Saves a setting at once (Coolify's instantSave…), leaving other unsaved edits be. */
    instant: (change: Partial<ChannelForm>) => Promise<void>
  }

  // Coolify's Email page says its own words in the toasts.
  const savedText = $derived(kind === 'email' ? 'Email notifications settings updated.' : 'Settings saved.')
  const testedText = $derived(kind === 'email' ? 'Test Email sent.' : 'Test notification sent.')

  const defaults = $derived(eventKinds.filter((e) => e.default).map((e) => e.kind))

  /** The channel shown; 'new' while an added channel is not saved yet. */
  let selected = $state<number | 'new' | null>(null)
  let pendingName = $state('')
  const channel = $derived(selected === 'new' ? null : (channels.find((c) => c.id === selected) ?? channels[0] ?? null))
  const shown = $derived<number | 'new'>(selected === 'new' ? 'new' : (channel?.id ?? 'new'))

  // Opened with the shown channel's form, so the unsaved bar does not flash in.
  let form = $state<ChannelForm>(untrack(() => (channel ? formOf(channel) : blankForm(defaults))))
  let errors = $state<Record<string, string>>({})
  let saving = $state(false)
  let toggling = $state(false)
  let testing = $state(false)
  let deliveriesVersion = $state(0)

  const base = $derived(channel ? formOf(channel) : blankForm(defaults))
  const dirty = $derived(!sameForm(form, base))

  function reset() {
    form = channel ? formOf(channel) : blankForm(defaults)
    errors = {}
  }

  // A new channel shown (another kind, the picker, a first save) opens its form.
  $effect(() => {
    void [kind, shown, eventKinds]
    untrack(reset)
  })

  /** The name of a first channel: the kind's, unless another channel has it. */
  function firstName(): string {
    const label = kindLabels[kind]
    for (let i = 1; ; i++) {
      const name = i === 1 ? label : `${label} ${i}`
      if (!allNames.includes(name)) return name
    }
  }

  async function store(body: Record<string, unknown>): Promise<NotificationChannel | null> {
    errors = {}
    try {
      const r = channel
        ? await api<{ channel: NotificationChannel }>('PATCH', `/notification-channels/${channel.id}`, { name: channel.name, ...body })
        : await api<{ channel: NotificationChannel }>('POST', '/notification-channels', {
            name: selected === 'new' ? pendingName : firstName(),
            kind,
            ...body,
          })
      onsaved(r.channel)
      return r.channel
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      errors = err.errors
      toast.error(Object.values(err.errors)[0] ?? err.message)
      return null
    }
  }

  async function save() {
    if (saving || !dirty) return
    saving = true
    const enabled = channel ? channel.enabled : false
    const c = await store({ ...payload(form), enabled })
    saving = false
    if (!c) return
    selected = c.id
    form = formOf(c)
    toast.success(savedText)
  }

  async function toggle() {
    toggling = true
    const c = await store({ ...payload(form), enabled: !(channel?.enabled ?? false) })
    toggling = false
    if (!c) return
    selected = c.id
    form = formOf(c)
    toast.success(savedText)
  }

  let askingRecipient = $state(false)
  let recipient = $state('')
  let recipientError = $state('')

  async function test(to?: string) {
    if (!channel) return
    testing = true
    recipientError = ''
    try {
      await api('POST', `/notification-channels/${channel.id}/test`, to ? { recipient: to } : undefined)
      askingRecipient = false
      toast.success(testedText)
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      if (err.errors.recipient) {
        recipientError = err.errors.recipient
        return
      }
      askingRecipient = false
      toast.error(err.message)
    } finally {
      testing = false
      deliveriesVersion++
    }
  }

  async function toggleEvent(event: string) {
    const toggled = (list: string[]) => (list.includes(event) ? list.filter((e) => e !== event) : [...list, event])
    if (!channel) {
      // Nothing is stored yet: the events go with the first save.
      form.events = toggled(form.events)
      return
    }
    const c = await store({ ...payload(formOf(channel)), event_kinds: toggled(channel.event_kinds) })
    if (!c) return
    form.events = [...c.event_kinds]
    toast.success(savedText)
  }

  async function instant(change: Partial<ChannelForm>) {
    Object.assign(form, change)
    if (!channel) return
    const c = await store(payload({ ...formOf(channel), ...change }))
    if (!c) {
      Object.assign(form, Object.fromEntries(Object.keys(change).map((k) => [k, formOf(channel)[k as keyof ChannelForm]])))
      return
    }
    toast.success(savedText)
  }

  async function remove() {
    if (!channel) return
    await api('DELETE', `/notification-channels/${channel.id}`)
    ondeleted(channel.id)
    selected = null
    toast.success('Channel deleted.')
  }

  function add(name: string) {
    pendingName = name
    selected = 'new'
  }
</script>

<div class="flex flex-col gap-8">
  {#if channels.length >= 2 || (channels.length === 1 && selected === 'new')}
    <ChannelPicker {channels} selected={shown} {pendingName} taken={allNames} onselect={(id) => (selected = id)} onadd={add} ondelete={remove} />
  {/if}

  <form
    id="{kind}-channel-form"
    onsubmit={(e) => {
      e.preventDefault()
      save()
    }}
    data-testid="channel-form"
  >
    <UnsavedBar {dirty} {saving} onsave={save} onreset={reset} />
    <SettingsGroup id="{kind}-settings" label={title} hint={description}>
      {#snippet actions()}
        <ChannelActions
          enabled={channel?.enabled ?? false}
          busy={toggling}
          {testing}
          ontoggle={toggle}
          plainWhenDisabled={kind === 'email'}
          ontest={() => {
            if (kind !== 'email') return test()
            recipientError = ''
            // Coolify starts the recipient at the signed-in user's address.
            recipient ||= session.member?.email ?? ''
            askingRecipient = true
          }}
        />
      {/snippet}
      {@render fields({ form, errors, channel, instant })}
    </SettingsGroup>
    {@render more?.({ form, errors, channel, instant })}
  </form>

  <EventGrid
    channel={kind}
    {eventKinds}
    selected={form.events}
    ontoggle={toggleEvent}
    {threaded}
    bind:threadIds={form.thread_ids}
    threadErrors={errors.thread_ids}
    formId="{kind}-channel-form"
  />

  {#if channel}
    <Deliveries channelId={channel.id} {eventKinds} version={deliveriesVersion} />
  {/if}

  {#if channels.length === 1 && selected !== 'new'}
    <ChannelPicker {channels} selected={shown} {pendingName} taken={allNames} onselect={(id) => (selected = id)} onadd={add} ondelete={remove} />
  {/if}
</div>

{#if kind === 'email'}
  <Modal title="Send Test Email" variant="none" bind:open={askingRecipient}>
    <form
      class="flex w-full flex-col gap-4"
      onsubmit={(e) => {
        e.preventDefault()
        test(recipient)
      }}
      data-testid="test-email-form"
    >
      <Input label="Recipient" bind:value={recipient} placeholder="test@example.com" required error={recipientError} data-testid="test-recipient" />
      <div class="flex justify-end border-t border-border pt-4">
        <Button type="submit" variant="highlighted" loading={testing}>Send email</Button>
      </div>
    </form>
  </Modal>
{/if}
