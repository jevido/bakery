<script lang="ts">
  // Coolify's Notifications layout (resources/views/components/notification/settings-layout.blade.php,
  // Apache-2.0, see NOTICE): the header below xl, the Notifications sidebar
  // with each Channel kind's icon (the brand ones masked from /svgs/, as the
  // Blade does), and the kind's page. ntfy, The Bakery's own kind, comes last.
  // Notifications are admin-only here; the Layout hides the sidebar link from
  // members, and a member who opens the URL is told so.
  import { api } from '../../lib/api'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import Icon, { type IconName } from '../../lib/Icon.svelte'
  import { href, notificationPath, type NotificationPage } from '../../lib/router.svelte'
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
    if (session.isAdmin) load().catch((e) => (error = e.message))
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

<section class="chrome application-settings-workspace w-full max-w-none">
  <header class="settings-mobile-header xl:hidden">
    <h1 class="settings-mobile-title">Notifications</h1>
    <p class="settings-mobile-description">Configure how your team receives deployment and system alerts.</p>
  </header>
  <div class="grid min-w-0 gap-8 xl:grid-cols-[210px_minmax(0,1fr)] xl:gap-8">
    <aside class="application-settings-navigation min-w-0 xl:self-start">
      <nav
        aria-label="Notification settings"
        class="grid grid-cols-2 gap-0.5 border-y border-neutral-200 py-3 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-1 xl:border-y-0 xl:py-0 dark:border-white/[0.06]"
      >
        <div class="nav-section hidden xl:block">Notifications</div>
        {#each items as item (item.page)}
          <a
            class={['menu-item', item.page === page && 'menu-item-active']}
            href={href(notificationPath(item.page))}
            aria-current={item.page === page ? 'page' : undefined}
            data-testid="notification-menu-item"
          >
            {#if item.brandIcon}
              <span
                class="menu-item-icon bg-current"
                style="mask: url('/svgs/{item.brandIcon}.svg') center / contain no-repeat; -webkit-mask: url('/svgs/{item.brandIcon}.svg') center / contain no-repeat;"
              ></span>
            {:else if item.icon}
              <Icon name={item.icon} class="menu-item-icon" />
            {/if}
            <span class="menu-item-label">{kindLabels[item.page]}</span>
          </a>
        {/each}
      </nav>
    </aside>

    <div class="min-w-0">
      {#if !session.isAdmin}
        <Empty title="Only admins can manage notifications" description="Ask an admin of this instance to change where The Bakery sends notifications." icon="notifications" />
      {:else if error}
        <p class="text-sm text-error">{error}</p>
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
</section>
