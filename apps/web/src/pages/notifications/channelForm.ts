import type { ChannelKind, EventKind, NotificationChannel } from '../../lib/types'

/** The Channel kinds in Coolify's sidebar order, ntfy (The Bakery's own) last. */
export const kindLabels: Record<ChannelKind, string> = {
  email: 'Email',
  discord: 'Discord',
  telegram: 'Telegram',
  slack: 'Slack',
  pushover: 'Pushover',
  webhook: 'Webhook',
  ntfy: 'ntfy',
}

/**
 * A Notification channel's settings as typed on its page. Secrets start
 * empty: the API keeps a stored secret that is sent empty.
 */
export type ChannelForm = {
  host: string
  port: string
  security: string
  username: string
  password: string
  from: string
  from_name: string
  to: string
  timeout: string
  ehlo_domain: string
  url: string
  ping: boolean
  bot_token: string
  chat_id: string
  thread_ids: Record<string, string>
  user_key: string
  api_token: string
  topic: string
  token: string
  secret: string
  events: string[]
}

export function blankForm(events: string[]): ChannelForm {
  return {
    host: '',
    port: '587',
    security: 'starttls',
    username: '',
    password: '',
    from: '',
    from_name: '',
    to: '',
    timeout: '',
    ehlo_domain: '',
    url: '',
    ping: false,
    bot_token: '',
    chat_id: '',
    thread_ids: {},
    user_key: '',
    api_token: '',
    topic: '',
    token: '',
    secret: '',
    events: [...events],
  }
}

/** The form of a stored channel, as its page opens it. */
export function formOf(c: NotificationChannel): ChannelForm {
  const s = c.settings
  return {
    ...blankForm(c.event_kinds),
    host: s.host ?? '',
    port: String(s.port ?? ''),
    security: s.security ?? 'starttls',
    username: s.username ?? '',
    from: s.from ?? '',
    from_name: s.from_name ?? '',
    to: (s.to ?? []).join(', '),
    timeout: s.timeout ? String(s.timeout) : '',
    ehlo_domain: s.ehlo_domain ?? '',
    // Only ntfy's server URL is shown; the other kinds' URLs are secret.
    url: c.kind === 'ntfy' ? (s.url ?? '') : '',
    ping: s.ping,
    chat_id: s.chat_id ?? '',
    thread_ids: { ...s.thread_ids },
    topic: s.topic ?? '',
  }
}

/** The request body that saves the form; the API reads the fields of the channel's kind. */
export function payload(f: ChannelForm) {
  return {
    event_kinds: f.events,
    settings: {
      host: f.host,
      port: Number(f.port) || 0,
      security: f.security,
      username: f.username,
      password: f.password,
      from: f.from,
      from_name: f.from_name,
      to: f.to.split(/[\s,;]+/).filter(Boolean),
      timeout: Number(f.timeout) || 0,
      ehlo_domain: f.ehlo_domain,
      url: f.url,
      ping: f.ping,
      bot_token: f.bot_token,
      chat_id: f.chat_id,
      thread_ids: f.thread_ids,
      user_key: f.user_key,
      api_token: f.api_token,
      topic: f.topic,
      token: f.token,
      secret: f.secret,
    },
  }
}

/**
 * Whether two forms save the same thing: a cleared topic id is no topic id,
 * and a number input's value (bound as a number) is its text.
 */
export function sameForm(a: ChannelForm, b: ChannelForm): boolean {
  const norm = (f: ChannelForm) =>
    JSON.stringify({
      ...f,
      port: String(f.port ?? ''),
      timeout: String(f.timeout ?? ''),
      thread_ids: Object.fromEntries(Object.entries(f.thread_ids).filter(([, v]) => v.trim() !== '').sort()),
      events: [...f.events].sort(),
    })
  return norm(a) === norm(b)
}

/** What the Notifications shell gives each kind's page. */
export type KindPageProps = {
  channels: NotificationChannel[]
  allNames: string[]
  eventKinds: EventKind[]
  onsaved: (c: NotificationChannel) => void
  ondeleted: (id: number) => void
}

/** A stored secret URL shows only its host, as the placeholder. */
export function storedHost(c: NotificationChannel | null): string {
  return c?.settings.url_host ? `Saved (${c.settings.url_host})` : ''
}
