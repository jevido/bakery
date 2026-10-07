<script lang="ts">
  // Coolify's Notifications layout (resources/views/components/notification/settings-layout.blade.php,
  // Apache-2.0, see NOTICE) as a Paperclip settings page (MIT, see NOTICE):
  // the Channel kinds drawn by ResourceNav as the Server frame draws its
  // sub-pages (the brand icons masked from /svgs/, as the Blade does), and
  // the kind's page. ntfy, The Bakery's own kind, comes last.
  // Notifications are admin-only here; the Layout hides the sidebar link from
  // members, and a member who opens the URL is told so.
  import { api } from '../../lib/api'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import { Bell } from '@lucide/svelte'
  import type { IconName } from '../../lib/Icon.svelte'
  import ResourceNav from '../../lib/ResourceNav.svelte'
  import SettingsPage from '../../lib/settings/SettingsPage.svelte'
  import { notificationPath, type NotificationPage } from '../../lib/router.svelte'
  import { session } from '../../lib/session.svelte'
  import type { EventKind, NotificationChannel } from '../../lib/types'
  import Empty from '../../lib/ui/Empty.svelte'
  import Spinner from '../../lib/ui/Spinner.svelte'
  import { kindLabels, type KindPageProps } from './channelForm'
  import Discord from './Discord.svelte'
  import Email from './Email.svelte'
  import Ntfy from './Ntfy.svelte'
  import Pushover from './Pushover.svelte'
  import Slack from './Slack.svelte'
  import Telegram from './Telegram.svelte'
  import Webhook from './Webhook.svelte'

  let { page }: { page: NotificationPage } = $props()

  const items: { page: NotificationPage; icon?: IconName; brandIcon?: string }[] = [
    { page: 'email', icon: 'mail' },
    { page: 'discord', brandIcon: 'discord' },
    { page: 'telegram', brandIcon: 'telegram' },
    { page: 'slack', brandIcon: 'slack' },
    { page: 'pushover', brandIcon: 'pushover' },
    { page: 'webhook', icon: 'destinations' },
    { page: 'ntfy', icon: 'notifications' },
  ]

  const nav = $derived([
    {
      label: 'Channels',
      items: items.map((i) => ({
        label: kindLabels[i.page],
        path: notificationPath(i.page),
        icon: i.icon,
        brandIcon: i.brandIcon,
        active: i.page === page,
        testid: 'notification-menu-item',
      })),
    },
  ])

  let channels = $state.raw<NotificationChannel[] | null>(null)
  let eventKinds = $state.raw<EventKind[]>([])
  let error = $state('')

  async function load() {
    const [c, e] = await Promise.all([
      api<{ channels: NotificationChannel[] }>('GET', '/notification-channels'),
      api<{ event_kinds: EventKind[] }>('GET', '/notification-event-kinds'),
    ])
    channels = c.channels
    eventKinds = e.event_kinds
  }
  // Each page reads the channels afresh; another tab may have changed them.
  $effect(() => {
    void page
    if (session.can('manage_notifications')) load().catch((e) => (error = e.message))
  })

  const kindProps = $derived<KindPageProps | null>(
    channels && {
      channels: channels.filter((c) => c.kind === page),
      allNames: channels.map((c) => c.name),
      eventKinds,
      onsaved: (saved) => {
        const list = channels ?? []
        channels = list.some((c) => c.id === saved.id) ? list.map((c) => (c.id === saved.id ? saved : c)) : [...list, saved]
      },
      ondeleted: (id) => (channels = (channels ?? []).filter((c) => c.id !== id)),
    },
  )

  $effect(() => {
    void page
    breadcrumb.set({ label: 'Notifications' })
  })
</script>

<SettingsPage icon={Bell} title="Notifications">
  <div class="grid min-w-0 gap-6 xl:grid-cols-[210px_minmax(0,1fr)] xl:gap-8">
    <ResourceNav groups={nav} label="Notification settings" />

    <div class="min-w-0">
      {#if !session.can('manage_notifications')}
        <Empty title="Notifications need the Manage notifications permission" description="Ask someone in this guild who has it to change where The Bakery sends notifications." icon="notifications" />
      {:else if error}
        <p class="text-sm text-destructive">{error}</p>
      {:else if !kindProps}
        <Spinner text="Loading…" />
      {:else if page === 'email'}
        <Email {...kindProps} />
      {:else if page === 'discord'}
        <Discord {...kindProps} />
      {:else if page === 'telegram'}
        <Telegram {...kindProps} />
      {:else if page === 'slack'}
        <Slack {...kindProps} />
      {:else if page === 'pushover'}
        <Pushover {...kindProps} />
      {:else if page === 'webhook'}
        <Webhook {...kindProps} />
      {:else}
        <Ntfy {...kindProps} />
      {/if}
    </div>
  </div>
</SettingsPage>
