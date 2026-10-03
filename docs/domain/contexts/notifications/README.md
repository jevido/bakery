# notifications

- Subdomain: generic
- Hosted in: `services/api` (module `contexts/notifications`)

## Purpose

Tell the people running The Bakery when something needs their attention: a
Deployment or a Backup execution failed, a Server became unreachable or came back, a
Server's disk usage is high. Each Notification goes to every Notification
channel subscribed to its Event kind: an email address, a Discord or Slack
channel, a Telegram chat, an ntfy topic or a webhook. It also emails an
Invitation to the person invited.

It does **not** decide what is worth telling: deployments, databases,
servers and identity announce what happened, and this context only turns
that into a message. It keeps no history beyond the recent Deliveries of
each channel; an audit log is something else.

## Language

| Term | Meaning |
| ---- | ------- |
| Notification channel | A named place Notifications go to: a Channel kind, its settings (secrets encrypted) and the Event kinds it is subscribed to. |
| Channel kind | `email` (SMTP server, from address, recipients), `discord` and `slack` (an incoming webhook URL, a secret), `telegram` (a bot token and a chat id), `ntfy` (a server URL, a topic and optionally a token) or `webhook` (any URL, kept secret, JSON signed with an optional secret). |
| Event kind | What a Notification is about: `deployment_failure`, `deployment_success`, `backup_failure`, `backup_success`, `server_unreachable`, `server_reachable`, `server_disk_usage`. A new channel is subscribed to all but the two `_success` ones. |
| Notification | What a publisher hands over: an Event kind, a title, a body, an optional link into the dashboard and when it happened. Not stored on its own. |
| Delivery | One Notification sent to one Notification channel: `pending`, `sent` or `failed`, the attempts made (at most 3) and the last error. The 50 newest per channel are kept. |
| Test notification | A Notification sent at once from a channel's Test button, recorded as a Delivery like any other. |

## Model

### Aggregates

| Aggregate | Invariants |
| --------- | ---------- |
| Notification channel | The name is unique (1–63 characters). The settings are valid for the Channel kind: URLs are `http` or `https`; an email channel has a host, a port, a security mode (`none`, `starttls`, `tls`), a from address and at least one recipient; Telegram has a bot token and a chat id; ntfy a server URL and a topic. It is subscribed to at least one Event kind. A secret setting left empty on a change keeps its stored value. |
| Delivery | Belongs to one Notification channel and goes when it goes. Attempts only grow, at most 3; after the third failure it is `failed` for good. Retries are 10 s after the first attempt and 60 s after the second. |

### Commands

- `AddChannel`, `ChangeChannel`, `DeleteChannel` [admin, owner].
- `TestChannel(channel)` [admin, owner]: sends a Test notification at once and
  answers with the outcome.
- `Notify(notification)`: a Delivery per Notification channel subscribed to
  its Event kind, sent in the background. Never fails the caller.
- `SendInvitation(invitation)`: the Invitation's link by email to the invited
  person, through the email channel with the lowest id; reports whether it
  was sent.

### Domain events

None published.

## Integration

- **Publishes:** the Notification channels API
  (`/api/notification-channels` and its Test and Deliveries) for the
  dashboard, admin-only; `notifications.Start(ctx)` (the dispatcher, and the
  subscriptions below). Nothing calls into it: it hears what the others
  announce.
- **Consumes**, each translated into a Notification in this context's words:
  - `deployments.OnDeploymentFinished` (succeeded or failed; not cancelled
    ones, nor those failed by a restart) → `deployment_failure` /
    `deployment_success`; a Preview Deployment's message names the
    Preview ("preview of pull request #n").
  - `databases.OnBackupExecutionFinished` (succeeded or failed; not those failed by a
    restart) → `backup_failure` / `backup_success`.
  - `servers.OnServerHealthChanged` (a Server probe changed something) →
    `server_unreachable`, `server_reachable` or `server_disk_usage`.
  - `identity.OnInvitationCreated` → an email to the invited person, whose
    outcome identity reports back to the inviting admin.
  - The auth middlewares from identity.
- **Talks to:** SMTP servers, Discord, Slack, Telegram's Bot API, ntfy
  servers and webhook receivers, each over its own small sender.

## Why it's shaped this way

- **A context of its own, generic.** What goes wrong lives in four contexts;
  none of them should know about Discord. Each announces what happened in its
  own terms and this context translates, so a new Channel kind touches only
  this context and a new Event kind touches only its publisher and the
  translation here.
- **Events are callbacks the downstream registers**, like every other event in
  The Bakery (`OnApplicationDeleted`, `OnCleanup`): the publishers export
  `On…(f)` and notifications registers in `Start`. There is one process, so a
  broker would add a moving part and no decoupling the callbacks lack. The
  publishers call them in their own goroutine with a recover, so a
  subscriber never slows or breaks a Deployment, a Backup execution or a probe.
- **SMTP settings live on the email Notification channel**, not in `.env`:
  an installation is configured from the dashboard, and more than one email
  channel (a team inbox, an on-call address) is possible. Goravel's mail
  facade reads one global configuration, so the email sender speaks SMTP
  itself (`net/smtp`).
- **Deliveries are stored and retried in-process** (now, +10 s, +60 s) by a
  dispatcher that claims due ones with `FOR UPDATE SKIP LOCKED`, so a
  restart resumes them instead of losing them. The volume one installation
  produces does not justify a queue worker to operate.
- **Only the newest 50 Deliveries per channel** are kept: they answer "did
  it arrive?", not "what happened last month?".
- **Invitation emails go through the first email channel**, whatever Event
  kinds it has, to the invited person only. An Invitation is addressed to a
  person, not to a channel's recipients, so it is not a Delivery; a failed
  send leaves the copy-the-link path as before and is logged.
- **The Telegram API base URL is configurable** (`BAKERY_TELEGRAM_API_URL`)
  only so tests can point it at a local stand-in; the other kinds take full
  URLs already.
- **Notification channels and Deliveries are The Bakery's own.** Coolify keeps
  one settings row per channel kind per team (one Discord, one Slack, ...)
  and no delivery log. The Bakery lets an admin add several named Notification
  channels of the same kind and shows each one's recent Deliveries, which
  is how a failing channel gets noticed. The Event kind values are
  Coolify's (`deployment_success`, `server_disk_usage`, ...), so the
  subscriptions map one to one. They were The Bakery's own before
  (`deployment_failed`, `deployment_succeeded`, `backup_failed`,
  `backup_succeeded`, `disk_almost_full`); stored subscriptions and
  Deliveries were migrated, but a webhook receiver that matched on the old
  names in the payload's `event` field must match the new ones. `ntfy` is a Channel kind Coolify does not
  have; it costs one small sender and is popular with self-hosters.
